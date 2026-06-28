import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  cancelScheduledSessionOperation,
  listScheduledSessionOperations,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  ScheduledOperation,
  ScheduledSessionOperation,
  ScheduledOperationStatus,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import { Button } from "./ui/button";
import { useEffect, useMemo } from "react";
import { Link } from "react-router";
import { ColumnDef } from "@tanstack/react-table";
import { DataTable } from "./base";
import { RefetchButton } from "./base/RefetchButton";
import { toast } from "sonner";
import { keepPreviousData } from "@tanstack/react-query";
import { useAtomValue } from "jotai";
import { currentGroupIdAtom } from "../atoms/currentGroupAtom";
import {
  formatScheduled,
  operationKindLabel,
  scheduledOperationStatusToLabel,
} from "../libs/scheduledOperationUtils";
import { SessionUserCountTrigger_Comparator } from "../../pbgen/hdlctrl/v1/controller_pb";
import { usePaginationState } from "../hooks/usePaginationState";
import SessionTip from "./SessionTip";
import HostTip from "./HostTip";

type Props = {
  /** session 詳細から開く場合は session_id でフィルタする */
  sessionId?: string;
};

const operationCaseToKind = (
  op?: ScheduledOperation,
):
  | "START_SESSION"
  | "STOP_SESSION"
  | "UPDATE_PARAMETERS"
  | "UPDATE_EXTRA_SETTINGS"
  | "UNKNOWN" => {
  switch (op?.operation.case) {
    case "startSession":
      return "START_SESSION";
    case "stopSession":
      return "STOP_SESSION";
    case "updateParameters":
      return "UPDATE_PARAMETERS";
    case "updateExtraSettings":
      return "UPDATE_EXTRA_SETTINGS";
    default:
      return "UNKNOWN";
  }
};

export default function ScheduledOperationList({ sessionId }: Props) {
  const { pageIndex, pageSize, setPageIndex, setPageSize, syncFromServer } =
    usePaginationState({
      defaultPageSize: 20,
      paramPrefix: sessionId ? "scheduledOps" : "",
    });

  const currentGroupId = useAtomValue(currentGroupIdAtom);

  const { data, isPending, refetch } = useQuery(
    listScheduledSessionOperations,
    {
      sessionId,
      // ヘッダーで全グループ (null) のときは group_id 未指定で送り、user の
      // アクセス可能な全グループの op を backend から取得する.
      groupId: currentGroupId ?? undefined,
      page: { pageIndex, pageSize },
    },
    { placeholderData: keepPreviousData },
  );

  useEffect(() => {
    if (data?.page) {
      syncFromServer(data.page.pageIndex, data.page.pageSize);
    }
  }, [data?.page, syncFromServer]);

  const { mutateAsync: cancelMutate, isPending: isCancelPending } = useMutation(
    cancelScheduledSessionOperation,
  );

  const handleCancel = async (id: string) => {
    try {
      await cancelMutate({ id });
      toast.success("予約をキャンセルしました");
      refetch();
    } catch (e) {
      toast.error(`キャンセル失敗: ${(e as Error).message}`);
    }
  };

  const columns: ColumnDef<ScheduledSessionOperation>[] = useMemo(
    () => [
      {
        id: "kind",
        header: "種別",
        cell: ({ row }) => {
          const kind = operationCaseToKind(row.original.operation);
          return kind === "UNKNOWN" ? "(unknown)" : operationKindLabel(kind);
        },
      },
      {
        id: "target",
        header: "対象",
        cell: ({ row }) => {
          if (row.original.sessionId) {
            return <SessionTip sessionId={row.original.sessionId} />;
          }
          if (row.original.hostId) {
            return <HostTip hostId={row.original.hostId} />;
          }
          return "-";
        },
      },
      {
        id: "trigger",
        header: "実行条件",
        cell: ({ row }) => {
          const trig = row.original.trigger?.trigger;
          if (trig?.case === "time") {
            return formatScheduled(trig.value.scheduledAt);
          }
          if (trig?.case === "sessionUserCount") {
            const v = trig.value;
            const op =
              v.comparator ===
              SessionUserCountTrigger_Comparator.GREATER_OR_EQUAL
                ? "≥"
                : "≤";
            return (
              <span className="inline-flex items-center gap-1">
                <SessionTip sessionId={v.sessionId} />
                <span className="text-muted-foreground">のユーザー数</span>
                <span>
                  {op} {v.threshold}
                </span>
              </span>
            );
          }
          return "-";
        },
      },
      {
        id: "status",
        header: "状態",
        cell: ({ row }) => scheduledOperationStatusToLabel(row.original.status),
      },
      {
        id: "lastError",
        header: "エラー",
        cell: ({ row }) => row.original.lastError ?? "",
      },
      {
        id: "actions",
        header: "",
        cell: ({ row }) =>
          row.original.status === ScheduledOperationStatus.PENDING ? (
            <Button
              variant="outline"
              size="sm"
              disabled={isCancelPending}
              onClick={() => handleCancel(row.original.id)}
            >
              キャンセル
            </Button>
          ) : null,
      },
    ],
    [isCancelPending],
  );

  const newHref = sessionId
    ? `/sessions/scheduled/new?sessionId=${sessionId}`
    : "/sessions/scheduled/new";

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <Button asChild>
          <Link to={newHref}>新規予約</Link>
        </Button>
      </div>
      <DataTable
        columns={columns}
        data={data?.scheduledOperations ?? []}
        isLoading={isPending}
        pagination={{
          pageIndex,
          pageSize,
          totalCount: data?.page?.totalCount ?? 0,
          onPageIndexChange: setPageIndex,
          onPageSizeChange: setPageSize,
        }}
      />
    </div>
  );
}
