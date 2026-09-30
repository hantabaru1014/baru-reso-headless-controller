import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { TriangleAlert } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  getHeadlessHost,
  getSessionDetails,
  listHeadlessAccounts,
  listHeadlessHost,
  searchSessions,
  transferResources,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { listGroups } from "../../pbgen/hdlctrl/v1/permission-GroupService_connectquery";
import { invalidate } from "../libs/notificationDispatch";
import { PERMISSION_KEYS, PermissionKey } from "../libs/permissionUtils";
import { GroupSelectField } from "./GroupSelectField";
import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui";

/** 移管の起点にするリソース. TransferResourcesRequest.resource と同じ形. */
export type TransferResource =
  | { case: "hostId"; value: string }
  | { case: "sessionId"; value: string }
  | { case: "account"; value: { groupId: string; accountId: string } };

// 起点リソース自身は必ず移管対象に含まれるので、移管先候補はその種別の write 権限で絞る.
// 一緒に移管されるリソース分の権限は dry_run でサーバーが検証する.
const START_RESOURCE_PERMISSION: Record<
  TransferResource["case"],
  PermissionKey
> = {
  hostId: PERMISSION_KEYS.HOST_WRITE,
  sessionId: PERMISSION_KEYS.SESSION_WRITE,
  account: PERMISSION_KEYS.ACCOUNT_WRITE,
};

function TransferPreviewList({
  label,
  items,
}: {
  label: string;
  items: { id: string; name: string }[];
}) {
  const { t } = useTranslation();
  return (
    <div className="space-y-1">
      <p className="font-medium">{label}</p>
      {items.length === 0 ? (
        <p className="text-muted-foreground">{t("common.none")}</p>
      ) : (
        <ul className="max-h-32 overflow-auto rounded border divide-y">
          {items.map((item) => (
            <li key={item.id} className="px-2 py-1 truncate">
              {item.name || item.id}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function ResourceTransferDialogBody({
  resource,
  sourceGroupId,
  onClose,
}: {
  resource: TransferResource;
  sourceGroupId: string;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data: groupsData } = useQuery(listGroups, {});
  const [destinationGroupId, setDestinationGroupId] = useState("");

  // 呼び出し側が閉じる際に起点リソースを空に戻すと、閉じるアニメーション中に空のリクエストが飛んでしまう.
  const hasResource =
    resource.case === "account"
      ? !!resource.value.groupId && !!resource.value.accountId
      : !!resource.value;

  // dry_run で権限・移管先の検証と移管対象の一覧取得だけを行う.
  // 毎回最新を見せたいのでキャッシュは残さない.
  const {
    data: preview,
    error: previewError,
    isFetching: isFetchingPreview,
  } = useQuery(
    transferResources,
    { resource, destinationGroupId, dryRun: true },
    {
      enabled: hasResource && !!destinationGroupId,
      retry: false,
      gcTime: 0,
    },
  );
  const { mutateAsync: mutateTransfer, isPending: isPendingTransfer } =
    useMutation(transferResources);

  const groupName = (groupId: string) =>
    groupsData?.groups.find((g) => g.id === groupId)?.name ?? groupId;

  const handleTransfer = async () => {
    try {
      const res = await mutateTransfer({
        resource,
        destinationGroupId,
        dryRun: false,
      });
      toast.success(
        t("resourceTransferDialog.transferred", {
          group: groupName(res.destinationGroupId),
        }),
      );
      invalidate(queryClient, listHeadlessAccounts);
      invalidate(queryClient, listHeadlessHost);
      invalidate(queryClient, getHeadlessHost);
      invalidate(queryClient, searchSessions);
      invalidate(queryClient, getSessionDetails);
      onClose();
    } catch (e) {
      toast.error(
        e instanceof Error
          ? e.message
          : t("resourceTransferDialog.transferFailed"),
      );
    }
  };

  return (
    <>
      <div className="space-y-4">
        <p className="text-sm text-muted-foreground">
          {t("resourceTransferDialog.description")}
        </p>
        <GroupSelectField
          label={t("resourceTransferDialog.destinationGroup")}
          helperText={t("resourceTransferDialog.destinationGroupHelper")}
          value={destinationGroupId}
          onChange={setDestinationGroupId}
          requiredPermission={START_RESOURCE_PERMISSION[resource.case]}
          excludeGroupId={sourceGroupId}
          readOnly={isPendingTransfer}
        />
        {previewError ? (
          <Alert variant="destructive">
            <AlertDescription>{previewError.message}</AlertDescription>
          </Alert>
        ) : preview ? (
          <div className="space-y-3 text-sm">
            <p>
              {t("resourceTransferDialog.previewSummary", {
                source: groupName(preview.sourceGroupId),
                destination: groupName(preview.destinationGroupId),
              })}
            </p>
            <div className="space-y-1">
              <p className="font-medium">
                {t("resourceTransferDialog.account")}
              </p>
              {preview.account ? (
                <p>
                  {preview.account.userName
                    ? `${preview.account.userName} (${preview.account.userId})`
                    : preview.account.userId}
                </p>
              ) : (
                <p className="text-muted-foreground">
                  {t("resourceTransferDialog.noAccount")}
                </p>
              )}
            </div>
            {preview.account?.merged && (
              <Alert>
                <TriangleAlert />
                <AlertTitle>
                  {t("resourceTransferDialog.mergedTitle")}
                </AlertTitle>
                <AlertDescription>
                  {t("resourceTransferDialog.mergedDescription", {
                    destination: groupName(preview.destinationGroupId),
                  })}
                </AlertDescription>
              </Alert>
            )}
            <TransferPreviewList
              label={t("resourceTransferDialog.hosts", {
                n: preview.hosts.length,
              })}
              items={preview.hosts}
            />
            <TransferPreviewList
              label={t("resourceTransferDialog.sessions", {
                n: preview.sessions.length,
              })}
              items={preview.sessions}
            />
          </div>
        ) : (
          isFetchingPreview && (
            <p className="text-center text-sm text-muted-foreground py-4">
              {t("common.loading")}
            </p>
          )
        )}
      </div>
      <DialogFooter>
        <Button
          onClick={handleTransfer}
          disabled={!preview || !!previewError || isPendingTransfer}
        >
          {isPendingTransfer
            ? t("resourceTransferDialog.transferring")
            : t("resourceTransferDialog.transfer")}
        </Button>
        <DialogClose asChild>
          <Button variant="outline">{t("common.cancel")}</Button>
        </DialogClose>
      </DialogFooter>
    </>
  );
}

/**
 * リソース (ホスト / セッション / ヘッドレスアカウント) を別グループへ移管するダイアログ.
 *
 * どのリソースを起点にしても、依存関係にある一式 (アカウント・そのアカウントを使う全ホスト・
 * それらのホスト上の全セッション) がまとめて移管されるため、移管先を選んだ時点で dry_run を投げて
 * 移管対象をプレビューし、確認の上で実行する.
 */
export function ResourceTransferDialog({
  open,
  onClose,
  resource,
  sourceGroupId,
}: {
  open: boolean;
  onClose: () => void;
  resource: TransferResource;
  /** 起点リソースが現在所属しているグループ. 移管先候補から除外する. */
  sourceGroupId: string;
}) {
  const { t } = useTranslation();

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{t("resourceTransferDialog.title")}</DialogTitle>
        </DialogHeader>
        {/* DialogContent の中身は open の間だけマウントされるので、閉じるたびに選択状態が破棄される */}
        <ResourceTransferDialogBody
          resource={resource}
          sourceGroupId={sourceGroupId}
          onClose={onClose}
        />
      </DialogContent>
    </Dialog>
  );
}
