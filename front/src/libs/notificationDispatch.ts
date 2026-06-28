import { createConnectQueryKey } from "@connectrpc/connect-query";
import type {
  DescMessage,
  DescMethodUnary,
  MessageInitShape,
} from "@bufbuild/protobuf";
import type { QueryClient } from "@tanstack/react-query";
import type { NotificationEvent } from "../../pbgen/hdlctrl/v1/notification_pb";
import { JobCompletedEvent_Level } from "../../pbgen/hdlctrl/v1/notification_pb";
import {
  getHeadlessHost,
  getSessionDetails,
  listHeadlessHost,
  listUsersInSession,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { toast } from "sonner";

// connect-query が生成する queryKey は
//   ["connect-query", { serviceName, methodName, cardinality?, input? }]
// 形式. TanStack Query は部分マッチで invalidate するので、input を省くと
// 同 method の全 input を一括無効化できる.

function invalidate<I extends DescMessage, O extends DescMessage>(
  queryClient: QueryClient,
  schema: DescMethodUnary<I, O>,
  input?: MessageInitShape<I>,
): void {
  const queryKey =
    input === undefined
      ? createConnectQueryKey({ schema, cardinality: undefined })
      : createConnectQueryKey({ schema, input, cardinality: "finite" });
  void queryClient.invalidateQueries({ queryKey });
}

/**
 * 受信した NotificationEvent から、影響範囲のクエリを invalidate する.
 * dispatcher (backend) は 1 つの事実につき 1 イベントしか送らないので、
 * 関連クエリの fan-out はここで完結させる.
 */
export function dispatchNotification(
  queryClient: QueryClient,
  ev: NotificationEvent,
): void {
  const payload = ev.payload;
  switch (payload.case) {
    case "sessionUpdated": {
      const { sessionId, hostId } = payload.value;
      invalidate(queryClient, getSessionDetails, { sessionId });
      if (hostId) invalidate(queryClient, getHeadlessHost, { hostId });
      invalidate(queryClient, listHeadlessHost);
      break;
    }

    case "sessionLifecycle": {
      const { sessionId, hostId } = payload.value;
      invalidate(queryClient, getSessionDetails, { sessionId });
      if (hostId) invalidate(queryClient, getHeadlessHost, { hostId });
      invalidate(queryClient, listHeadlessHost);
      break;
    }

    case "sessionUserChanged": {
      const { sessionId } = payload.value;
      invalidate(queryClient, listUsersInSession, { sessionId });
      invalidate(queryClient, getSessionDetails, { sessionId });
      break;
    }

    case "hostUpdated": {
      const { hostId } = payload.value;
      invalidate(queryClient, getHeadlessHost, { hostId });
      invalidate(queryClient, listHeadlessHost);
      break;
    }

    case "hostListChanged": {
      invalidate(queryClient, listHeadlessHost);
      break;
    }

    case "jobCompleted": {
      // クエリ invalidate は対応する hostListChanged / sessionLifecycle / hostUpdated
      // 経由で別途行われる (docker event watcher や container HostEvent stream 由来).
      // ここでは toast 表示のみ.
      const { message, level } = payload.value;

      if (level === JobCompletedEvent_Level.ERROR) {
        toast.error(message);
      } else if (level === JobCompletedEvent_Level.SUCCESS) {
        toast.success(message);
      } else {
        toast(message);
      }

      break;
    }

    case "keepAlive":
    case undefined:
    default:
      // keepAlive は no-op.
      break;
  }
}
