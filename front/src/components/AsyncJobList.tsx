import { useQuery } from "@connectrpc/connect-query";
import { keepPreviousData } from "@tanstack/react-query";
import { ColumnDef } from "@tanstack/react-table";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  getAsyncJob,
  listAsyncJobs,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  AsyncJob,
  AsyncJobStatus,
  AsyncJobType,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import { usePaginationState } from "../hooks/usePaginationState";
import { usePermissions } from "../hooks/usePermissions";
import {
  asyncJobStatusToLabel,
  asyncJobTypeToLabel,
  isAsyncJobActive,
  resolveAsyncJobTarget,
} from "../libs/asyncJobUtils";
import { formatTimestamp } from "../libs/datetimeUtils";
import { PERMISSION_KEYS } from "../libs/permissionUtils";
import { DataTable } from "./base/DataTable";
import { RefetchButton } from "./base/RefetchButton";
import { SelectField } from "./base/SelectField";
import { UserCell } from "./base/UserCell";
import HostTip from "./HostTip";
import SessionTip from "./SessionTip";
import {
  Badge,
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Label,
} from "./ui";

/** 未完了の job が残っている間だけポーリングする間隔 (ms). */
const ACTIVE_POLL_INTERVAL_MS = 5000;

const STATUS_VALUES = [
  AsyncJobStatus.PENDING,
  AsyncJobStatus.RUNNING,
  AsyncJobStatus.SUCCEEDED,
  AsyncJobStatus.FAILED,
];

const TYPE_VALUES = [
  AsyncJobType.START_HOST,
  AsyncJobType.SHUTDOWN_HOST,
  AsyncJobType.RESTART_HOST,
  AsyncJobType.START_SESSION,
  AsyncJobType.STOP_SESSION,
  AsyncJobType.BUILD_IMAGE,
];

/** SelectField は id (文字列) で選択状態を持つので、enum 値から導出する. */
const filterId = (value?: number) =>
  value === undefined ? "ALL" : String(value);

/**
 * enum 値を選ぶフィルタ. 「全て」を undefined として扱う点だけが共通の関心事で、
 * status 用と種別用で中身は同じなのでここに畳んでいる.
 */
function EnumFilter<V extends number>({
  label,
  values,
  selected,
  toLabel,
  onChange,
}: {
  label: string;
  values: V[];
  selected?: V;
  toLabel: (v: V) => string;
  onChange: (v?: V) => void;
}) {
  const { t } = useTranslation();
  const options = [
    { id: "ALL", value: undefined, label: t("asyncJobList.filterAll") },
    ...values.map((v) => ({ id: filterId(v), value: v, label: toLabel(v) })),
  ];

  return (
    <SelectField
      label={label}
      options={options}
      selectedId={filterId(selected)}
      onChange={(o) => onChange(o.value)}
    />
  );
}

const statusBadgeVariant = (
  s: AsyncJobStatus,
): "default" | "secondary" | "destructive" | "outline" => {
  switch (s) {
    case AsyncJobStatus.SUCCEEDED:
      return "default";
    case AsyncJobStatus.FAILED:
      return "destructive";
    case AsyncJobStatus.RUNNING:
    case AsyncJobStatus.PENDING:
      return "secondary";
    default:
      return "outline";
  }
};

export default function AsyncJobList() {
  const { t } = useTranslation();
  const { pageIndex, pageSize, setPageIndex, setPageSize } =
    usePaginationState();
  const { hasSystemPermission } = usePermissions();
  const canSeeAllUsers = hasSystemPermission(
    PERMISSION_KEYS.SYSTEM_GROUP_MANAGE,
  );

  const [statusFilter, setStatusFilter] = useState<AsyncJobStatus>();
  const [typeFilter, setTypeFilter] = useState<AsyncJobType>();
  const [includeAllUsers, setIncludeAllUsers] = useState(false);
  // エラー詳細ダイアログの対象. 一覧に載る last_error を summary として先に見せ、
  // ビルドログ等のフルテキストは開いてから GetAsyncJob で取りに行く
  // (一覧レスポンスに数百KB のログを載せないため).
  const [errorTarget, setErrorTarget] = useState<
    { id: string; summary: string } | undefined
  >();

  const { data, isPending, refetch } = useQuery(
    listAsyncJobs,
    {
      status: statusFilter,
      jobType: typeFilter,
      // include_all_users が false のときは backend が caller 自身の job のみに
      // 絞るので created_by は送らない.
      includeAllUsers: canSeeAllUsers && includeAllUsers ? true : undefined,
      page: { pageIndex, pageSize },
    },
    {
      placeholderData: keepPreviousData,
      // PENDING / RUNNING が残っている間だけ自動更新し、全部完了したら止める.
      refetchInterval: (query) =>
        query.state.data?.jobs.some(isAsyncJobActive)
          ? ACTIVE_POLL_INTERVAL_MS
          : false,
    },
  );

  const { data: errorDetailData, isPending: isErrorDetailPending } = useQuery(
    getAsyncJob,
    { id: errorTarget?.id ?? "" },
    {
      enabled: errorTarget !== undefined,
      // FAILED は終端状態でログも不変なので、開き直しやフォーカス復帰で
      // 数MB のログを取り直さない.
      staleTime: Infinity,
    },
  );

  // 詳細ログを持たない job (ホスト起動失敗など) は summary をそのまま全文として見せる.
  const errorDetailText = errorDetailData?.errorDetail ?? errorTarget?.summary;

  const handleCopyError = async (message: string) => {
    await navigator.clipboard.writeText(message);
    toast.success(t("asyncJobList.copied"));
  };

  const columns: ColumnDef<AsyncJob>[] = useMemo(
    () => [
      {
        id: "jobType",
        header: t("asyncJobList.columnJob"),
        cell: ({ row }) => asyncJobTypeToLabel(row.original.jobType),
      },
      {
        id: "target",
        header: t("asyncJobList.columnTarget"),
        cell: ({ row }) => {
          const { hostId, sessionId } = resolveAsyncJobTarget(row.original);
          if (hostId) {
            return <HostTip hostId={hostId} />;
          }
          if (sessionId) {
            return <SessionTip sessionId={sessionId} />;
          }
          return "-";
        },
      },
      {
        id: "createdBy",
        header: t("asyncJobList.columnCreatedBy"),
        cell: ({ row }) =>
          row.original.createdBy ? (
            <UserCell userId={row.original.createdBy} />
          ) : (
            "-"
          ),
      },
      {
        id: "status",
        header: t("asyncJobList.columnStatus"),
        cell: ({ row }) => (
          <Badge variant={statusBadgeVariant(row.original.status)}>
            {asyncJobStatusToLabel(row.original.status)}
          </Badge>
        ),
      },
      {
        id: "createdAt",
        header: t("asyncJobList.columnCreatedAt"),
        cell: ({ row }) => formatTimestamp(row.original.createdAt) || "-",
      },
      {
        id: "executedAt",
        header: t("asyncJobList.columnExecutedAt"),
        cell: ({ row }) => formatTimestamp(row.original.executedAt) || "-",
      },
      {
        id: "lastError",
        header: t("asyncJobList.columnError"),
        cell: ({ row }) => {
          const { id, status, lastError } = row.original;
          if (status !== AsyncJobStatus.FAILED || !lastError) {
            return "-";
          }
          return (
            <button
              type="button"
              className="block w-full truncate text-left text-destructive hover:underline"
              title={t("asyncJobList.showErrorDetail")}
              onClick={() => setErrorTarget({ id, summary: lastError })}
            >
              {lastError}
            </button>
          );
        },
      },
    ],
    [t],
  );

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <div className="flex gap-2 items-end">
          <EnumFilter
            label={t("asyncJobList.filterStatus")}
            values={STATUS_VALUES}
            selected={statusFilter}
            toLabel={asyncJobStatusToLabel}
            onChange={(v) => {
              setStatusFilter(v);
              setPageIndex(0);
            }}
          />
          <EnumFilter
            label={t("asyncJobList.filterType")}
            values={TYPE_VALUES}
            selected={typeFilter}
            toLabel={asyncJobTypeToLabel}
            onChange={(v) => {
              setTypeFilter(v);
              setPageIndex(0);
            }}
          />
          {canSeeAllUsers && (
            <div className="flex items-center gap-2 h-9">
              <Checkbox
                id="asyncJobIncludeAllUsers"
                checked={includeAllUsers}
                onCheckedChange={(checked) => {
                  setIncludeAllUsers(checked === true);
                  setPageIndex(0);
                }}
              />
              <Label htmlFor="asyncJobIncludeAllUsers">
                {t("asyncJobList.includeAllUsers")}
              </Label>
            </div>
          )}
        </div>
        <RefetchButton refetch={refetch} />
      </div>
      <DataTable
        columns={columns}
        data={data?.jobs ?? []}
        isLoading={isPending}
        pagination={{
          pageIndex,
          pageSize,
          totalCount: data?.page?.totalCount ?? 0,
          onPageIndexChange: setPageIndex,
          onPageSizeChange: setPageSize,
        }}
      />
      <Dialog
        open={errorTarget !== undefined}
        onOpenChange={(open) => !open && setErrorTarget(undefined)}
      >
        <DialogContent className="sm:max-w-[720px]">
          <DialogHeader>
            <DialogTitle>{t("asyncJobList.errorDialogTitle")}</DialogTitle>
          </DialogHeader>
          <pre className="max-h-[50vh] overflow-auto rounded-md border bg-muted p-3 text-xs whitespace-pre-wrap break-all">
            {errorDetailText}
          </pre>
          {isErrorDetailPending && (
            <p className="text-xs text-muted-foreground">
              {t("asyncJobList.loadingErrorDetail")}
            </p>
          )}
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => handleCopyError(errorDetailText ?? "")}
            >
              {t("asyncJobList.copy")}
            </Button>
            <Button
              variant="secondary"
              onClick={() => setErrorTarget(undefined)}
            >
              {t("asyncJobList.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
