import { useEffect, useRef } from "react";
import { useAtomValue } from "jotai";
import { UseFormGetValues, UseFormSetValue } from "react-hook-form";
import { Transport } from "@connectrpc/connect";
import { callUnaryMethod, useTransport } from "@connectrpc/connect-query";
import {
  getResoniteUser,
  searchResoniteUsers,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { sessionAtom } from "../atoms/sessionAtom";
import {
  fillAutoInviteUsers,
  fillDefaultUserRoles,
  isSameUserName,
  KnownResoniteUser,
  SessionFormValues,
} from "../libs/sessionFormUtils";

/** ID と名前から Resonite ユーザーを引く。見つからない・失敗した分は結果に含めない。 */
async function fetchUsers(
  transport: Transport,
  ids: Iterable<string>,
  names: Iterable<string>,
): Promise<KnownResoniteUser[]> {
  const results = await Promise.allSettled([
    ...Array.from(ids, (resoniteId) =>
      callUnaryMethod(transport, getResoniteUser, { resoniteId }),
    ),
    ...Array.from(names, async (name) => {
      // 部分一致検索なので、名前が完全一致するユーザーだけ採用する。
      const { users } = await callUnaryMethod(transport, searchResoniteUsers, {
        name,
      });
      return users.find((u) => isSameUserName(u.name, name));
    }),
  ]);
  return results.flatMap((r) =>
    r.status === "fulfilled" && r.value ? [r.value] : [],
  );
}

/**
 * 履歴から復元したセッションフォームの自動招待・ロール行を補完する。
 * 履歴には参加許可が ID のみ、招待とロールが名前のみで残るので、
 * 不足分を Resonite Cloud から引き (ホスト不要)、同じユーザーの行を 1 行にまとめる。
 */
export const useFillRestoredSessionUsers = (
  getValues: UseFormGetValues<SessionFormValues>,
  setValue: UseFormSetValue<SessionFormValues>,
) => {
  const transport = useTransport();
  const session = useAtomValue(sessionAtom);
  const hasStarted = useRef(false);

  useEffect(() => {
    if (hasStarted.current) return;
    hasStarted.current = true;

    const invites = getValues("autoInviteUsernames") ?? [];
    const roles = getValues("defaultUserRoles") ?? [];
    if (
      invites.every((u) => u.userId && u.userName) &&
      roles.every((r) => r.iconUrl)
    ) {
      return;
    }

    // 取得中の編集を消さないよう、呼ばれた時点の値に対して補完する。
    const fill = (users: KnownResoniteUser[]) => {
      setValue(
        "autoInviteUsernames",
        fillAutoInviteUsers(getValues("autoInviteUsernames"), users),
      );
      setValue(
        "defaultUserRoles",
        fillDefaultUserRoles(getValues("defaultUserRoles"), users),
      );
    };

    // ログインユーザーの分は手元の情報ですぐ補完し、問い合わせない。
    const me = session?.user;
    const known: KnownResoniteUser[] =
      me?.resoniteId && me.resoniteName
        ? [{ id: me.resoniteId, name: me.resoniteName, iconUrl: me.iconUrl }]
        : [];
    if (known.length > 0) fill(known);
    const isKnown = (name: string) =>
      known.some((u) => isSameUserName(u.name, name));

    const knownInvites = fillAutoInviteUsers(invites, known);
    const ids = knownInvites.filter((u) => !u.userName).map((u) => u.userId);
    const names = [
      ...knownInvites.filter((u) => !u.userId).map((u) => u.userName),
      ...roles
        .filter((r) => !r.iconUrl && !isKnown(r.userName))
        .map((r) => r.userName),
    ];
    if (ids.length === 0 && names.length === 0) return;

    void fetchUsers(transport, new Set(ids), new Set(names)).then((fetched) => {
      if (fetched.length > 0) fill(fetched);
    });
  }, [transport, session, getValues, setValue]);
};
