import { useAtom } from "jotai";
import { useEffect, useRef } from "react";
import { sessionAtom } from "../atoms/sessionAtom";
import { useMutation } from "@connectrpc/connect-query";
import { getResoniteUser } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";

/**
 * sessionAtom の resoniteId が変わったときに自動的に GetResoniteUser を呼び出し、
 * resoniteName と iconUrl を更新するフック。
 * App.tsx などのルートコンポーネントで使用する。
 */
export const useResoniteUserSync = () => {
  const [session, setSession] = useAtom(sessionAtom);
  const { mutateAsync: fetchResoniteUser } = useMutation(getResoniteUser);
  const lastFetchedResoniteId = useRef<string | null>(null);

  useEffect(() => {
    const resoniteId = session?.user?.resoniteId;

    // resoniteId がない、または既にフェッチ済みの場合はスキップ
    if (!resoniteId || resoniteId === lastFetchedResoniteId.current) {
      return;
    }

    // resoniteName が既にある場合もスキップ
    if (session?.user?.resoniteName) {
      lastFetchedResoniteId.current = resoniteId;
      return;
    }

    // 非同期でユーザー情報を取得
    (async () => {
      try {
        const userInfo = await fetchResoniteUser({ resoniteId });
        lastFetchedResoniteId.current = resoniteId;

        setSession((prev) => {
          if (!prev) return prev;
          return {
            ...prev,
            user: {
              ...prev.user,
              resoniteName: userInfo.name,
              iconUrl: userInfo.iconUrl || prev.user.iconUrl,
            },
          };
        });
      } catch {
        // エラー時はスキップ（後でリトライされる可能性あり）
        lastFetchedResoniteId.current = resoniteId;
      }
    })();
  }, [
    session?.user?.resoniteId,
    session?.user?.resoniteName,
    fetchResoniteUser,
    setSession,
  ]);
};
