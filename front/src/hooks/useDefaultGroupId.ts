import { useQuery } from "@connectrpc/connect-query";
import { useAtomValue } from "jotai";
import { useMemo } from "react";
import { listGroups } from "../../pbgen/hdlctrl/v1/permission-GroupService_connectquery";
import { GroupType } from "../../pbgen/hdlctrl/v1/permission_pb";
import { currentGroupIdAtom } from "../atoms/currentGroupAtom";
import { usePermissions } from "./usePermissions";
import { PermissionKey } from "../libs/permissionUtils";

/**
 * リソース作成フォーム用に「デフォルト所属グループ ID」を返す.
 *
 * 優先順:
 *  1. `currentGroupIdAtom` が non-null かつ、当該グループに対し `requiredPermission` を保持していればそれ
 *  2. 自分の personal グループ (持っていれば)
 *  3. 空文字 (まだ判定できない / 該当無し)
 *
 * 戻り値が変わったタイミングでフォームの初期値として `setValue` する想定.
 */
export function useDefaultGroupId(requiredPermission: PermissionKey): string {
  const currentGroupId = useAtomValue(currentGroupIdAtom);
  const { hasPermission, permissions } = usePermissions();
  const { data } = useQuery(listGroups, {});

  return useMemo(() => {
    if (currentGroupId && hasPermission(currentGroupId, requiredPermission)) {
      return currentGroupId;
    }
    const groups = data?.groups ?? [];
    const myGroupIds = new Set(permissions?.groups.map((g) => g.groupId) ?? []);
    const personal = groups.find(
      (g) => g.type === GroupType.PERSONAL && myGroupIds.has(g.id),
    );
    return personal?.id ?? "";
  }, [
    currentGroupId,
    hasPermission,
    requiredPermission,
    data?.groups,
    permissions?.groups,
  ]);
}
