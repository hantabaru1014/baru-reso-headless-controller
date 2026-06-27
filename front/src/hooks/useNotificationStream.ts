import { createClient } from "@connectrpc/connect";
import { useTransport } from "@connectrpc/connect-query";
import { ConnectError, Code } from "@connectrpc/connect";
import { useQueryClient } from "@tanstack/react-query";
import { useAtomValue, useSetAtom } from "jotai";
import { useEffect } from "react";
import { sessionAtom, sessionRefreshTokenAtom } from "../atoms/sessionAtom";
import { NotificationService } from "../../pbgen/hdlctrl/v1/notification_pb";
import { dispatchNotification } from "../libs/notificationDispatch";

const initialBackoffMs = 1_000;
const maxBackoffMs = 30_000;

/**
 * NotificationService.SubscribeNotifications を 1 本だけ張り、受信した
 * NotificationEvent を queryClient.invalidateQueries にマッピングする.
 *
 * ログイン中のみ動作. 切断時 exponential backoff (1s → 30s cap) で再接続.
 * 認証エラーが返ったら session をクリアしてサインインに誘導する.
 */
export function useNotificationStream(): void {
  const queryClient = useQueryClient();
  const transport = useTransport();
  const session = useAtomValue(sessionAtom);
  const setSession = useSetAtom(sessionAtom);
  const setRefreshToken = useSetAtom(sessionRefreshTokenAtom);

  useEffect(() => {
    if (!session) return;

    const controller = new AbortController();
    let backoffMs = initialBackoffMs;

    void (async () => {
      const client = createClient(NotificationService, transport);
      while (!controller.signal.aborted) {
        try {
          const stream = client.subscribeNotifications(
            {},
            { signal: controller.signal },
          );
          for await (const ev of stream) {
            backoffMs = initialBackoffMs;
            dispatchNotification(queryClient, ev);
          }
        } catch (e) {
          if (controller.signal.aborted) break;

          if (e instanceof ConnectError && e.code === Code.Unauthenticated) {
            setSession(null);
            setRefreshToken(null);
            break;
          }

          console.warn("notification stream error; reconnecting", e);
        }

        if (controller.signal.aborted) break;

        await waitOrAbort(backoffMs, controller.signal);
        backoffMs = Math.min(backoffMs * 2, maxBackoffMs);
      }
    })();

    return () => controller.abort();
  }, [queryClient, transport, session, setSession, setRefreshToken]);
}

function waitOrAbort(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    if (signal.aborted) {
      resolve();
      return;
    }

    const timer = setTimeout(() => {
      signal.removeEventListener("abort", onAbort);
      resolve();
    }, ms);

    const onAbort = () => {
      clearTimeout(timer);
      resolve();
    };
    signal.addEventListener("abort", onAbort, { once: true });
  });
}
