import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  Button,
  DialogTrigger,
  Badge,
  DialogClose,
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  Skeleton,
} from "./ui";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  acceptFriendRequests,
  createHeadlessAccount,
  deleteHeadlessAccount,
  getFriendRequests,
  getHeadlessAccountStorageInfo,
  listHeadlessAccounts,
  refetchHeadlessAccountInfo,
  removeContact,
  sendFriendRequest,
  updateHeadlessAccountCredentials,
  updateHeadlessAccountIcon,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { RefetchButton } from "./base/RefetchButton";
import { useCallback, useEffect, useMemo, useState } from "react";
import { keepPreviousData } from "@tanstack/react-query";
import { usePaginationState } from "../hooks/usePaginationState";
import { toast } from "sonner";
import { DataTable, TextField } from "./base";
import { ColumnDef } from "@tanstack/react-table";
import {
  HeadlessAccount,
  UserInfo,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import prettyBytes from "@/libs/prettyBytes";
import { resolveUrl } from "@/libs/skyfrostUtils";
import { MoreVertical } from "lucide-react";
import { IconChangeDialog } from "./IconChangeDialog";
import { ResoniteUserIcon } from "./ResoniteUserIcon";
import { ChatDialog } from "./chat";
import { GroupSelectField } from "./GroupSelectField";
import { PermissionGuardedButton } from "./base/PermissionGuardedButton";
import { usePermissions } from "../hooks/usePermissions";
import { useDefaultGroupId } from "../hooks/useDefaultGroupId";
import { useAtomValue } from "jotai";
import { currentGroupIdAtom } from "../atoms/currentGroupAtom";
import { PERMISSION_KEYS } from "../libs/permissionUtils";

function FriendRequestsDialog({
  onClose,
  accountId,
}: {
  onClose?: () => void;
  accountId: string;
}) {
  const { data, isPending, refetch } = useQuery(getFriendRequests, {
    headlessAccountId: accountId,
  });
  const { mutateAsync: mutateAcceptFriendRequest, isPending: isPendingAccept } =
    useMutation(acceptFriendRequests);
  const { mutateAsync: mutateRemoveContact, isPending: isPendingRemove } =
    useMutation(removeContact);
  const [actionUserId, setActionUserId] = useState<string | null>(null);

  const columns: ColumnDef<UserInfo>[] = [
    {
      accessorKey: "iconUrl",
      header: "アイコン",
      cell: ({ row }) => (
        <ResoniteUserIcon
          iconUrl={row.original.iconUrl}
          alt={`${row.original.name}のアイコン`}
        />
      ),
    },
    {
      accessorKey: "name",
      header: "名前",
    },
    {
      id: "actions",
      header: "アクション",
      cell: ({ row }) => {
        const isBusy =
          (isPendingAccept || isPendingRemove) &&
          actionUserId === row.original.id;
        return (
          <div className="flex gap-2 justify-end">
            <Button
              variant="destructive"
              onClick={async () => {
                setActionUserId(row.original.id);
                try {
                  await mutateRemoveContact({
                    headlessAccountId: accountId,
                    targetUserId: row.original.id,
                  });
                  refetch();
                  toast.success("フレンドリクエストを拒否しました");
                } catch (e) {
                  toast.error(
                    e instanceof Error
                      ? e.message
                      : "フレンドリクエストの拒否に失敗しました",
                  );
                } finally {
                  setActionUserId(null);
                }
              }}
              disabled={isBusy}
            >
              拒否
            </Button>
            <Button
              onClick={async () => {
                setActionUserId(row.original.id);
                try {
                  await mutateAcceptFriendRequest({
                    headlessAccountId: accountId,
                    targetUserId: row.original.id,
                  });
                  refetch();
                  toast.success("フレンドリクエストを承認しました");
                } catch (e) {
                  toast.error(
                    e instanceof Error
                      ? e.message
                      : "フレンドリクエストの承認に失敗しました",
                  );
                } finally {
                  setActionUserId(null);
                }
              }}
              disabled={isBusy}
            >
              承認
            </Button>
          </div>
        );
      },
    },
  ];

  return (
    <Dialog onOpenChange={(open) => !open && onClose?.()}>
      {(data?.requestedContacts.length ?? 0) > 0 && (
        <DialogTrigger asChild>
          <Button variant="ghost" title="フレンドリクエスト一覧を開く">
            <Badge variant="default">
              {data?.requestedContacts.length ?? 0}
            </Badge>
          </Button>
        </DialogTrigger>
      )}
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader className="flex justify-between">
          <DialogTitle>フレンドリクエスト</DialogTitle>
        </DialogHeader>
        <div>
          <div className="flex justify-end mb-2">
            <RefetchButton refetch={refetch} />
          </div>
          <DataTable
            columns={columns}
            data={data?.requestedContacts ?? []}
            isLoading={isPending}
          />
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">閉じる</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function SendFriendRequestDialog({
  accountId,
  open,
  onClose,
}: {
  accountId: string;
  open: boolean;
  onClose?: () => void;
}) {
  const [query, setQuery] = useState("");
  const { mutateAsync: mutateSendFriendRequest, isPending } =
    useMutation(sendFriendRequest);

  const handleSend = async () => {
    const value = query.trim();
    if (!value) return;
    const isId = value.toLowerCase().startsWith("u-");
    try {
      await mutateSendFriendRequest({
        headlessAccountId: accountId,
        user: { case: isId ? "userId" : "userName", value },
      });
      toast.success("フレンドリクエストを送信しました");
      setQuery("");
      onClose?.();
    } catch (e) {
      toast.error(
        e instanceof Error
          ? e.message
          : "フレンドリクエストの送信に失敗しました",
      );
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) {
          setQuery("");
          onClose?.();
        }
      }}
    >
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>フレンド追加</DialogTitle>
        </DialogHeader>
        <div className="grid gap-3 py-2">
          <TextField
            label="ユーザーID または ユーザー名"
            placeholder="U-xxxx または username"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <p className="text-xs text-muted-foreground">
            &quot;U-&quot; で始まる場合は ID
            として、そうでなければユーザー名として検索します
          </p>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">キャンセル</Button>
          </DialogClose>
          <Button onClick={handleSend} disabled={isPending || !query.trim()}>
            {isPending ? "送信中..." : "送信"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function NewAccountDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose?: () => void;
}) {
  const { mutateAsync: mutateCreateAccount, isPending } = useMutation(
    createHeadlessAccount,
  );
  const defaultGroupId = useDefaultGroupId(PERMISSION_KEYS.ACCOUNT_WRITE);
  const [credential, setCredential] = useState("");
  const [password, setPassword] = useState("");
  const [groupId, setGroupId] = useState("");

  // ダイアログを開く際にコンテキストグループを初期値として埋める.
  useEffect(() => {
    if (open && defaultGroupId && !groupId) {
      setGroupId(defaultGroupId);
    }
  }, [open, defaultGroupId, groupId]);

  return (
    <Dialog
      open={open}
      onOpenChange={(open) => {
        if (open) {
          setCredential("");
          setPassword("");
          setGroupId(defaultGroupId);
        } else {
          onClose?.();
        }
      }}
    >
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>ヘッドレスアカウントを追加</DialogTitle>
        </DialogHeader>
        <div className="grid gap-4 py-4">
          <TextField
            label="Email or UserID"
            value={credential}
            onChange={(e) => setCredential(e.target.value)}
          />
          <TextField
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          <GroupSelectField
            value={groupId}
            onChange={setGroupId}
            requiredPermission={PERMISSION_KEYS.ACCOUNT_WRITE}
            helperText="アカウントを所属させるグループ"
          />
        </div>
        <DialogFooter>
          <Button
            onClick={async () => {
              try {
                await mutateCreateAccount({
                  credential,
                  password,
                  groupId: groupId || undefined,
                });
                toast.success("アカウントを追加しました");
              } catch (e) {
                toast.error(
                  e instanceof Error
                    ? e.message
                    : "アカウントの追加に失敗しました",
                );
                return;
              }
              onClose?.();
            }}
            disabled={isPending || !groupId}
          >
            追加
          </Button>
          <DialogClose asChild>
            <Button variant="outline">キャンセル</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function UpdateAccountCredentialsDialog({
  accountId,
  open,
  onClose,
}: {
  accountId: string;
  open: boolean;
  onClose?: () => void;
}) {
  const { mutateAsync: mutateUpdateAccount, isPending } = useMutation(
    updateHeadlessAccountCredentials,
  );
  const [credential, setCredential] = useState("");
  const [password, setPassword] = useState("");

  return (
    <Dialog
      open={open}
      onOpenChange={(open) => {
        if (open) {
          setCredential("");
          setPassword("");
        } else {
          onClose?.();
        }
      }}
    >
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>アカウントのログイン情報を更新</DialogTitle>
        </DialogHeader>
        <div className="grid gap-4 py-4">
          <TextField
            label="Email or UserID"
            value={credential}
            onChange={(e) => setCredential(e.target.value)}
          />
          <TextField
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        <DialogFooter>
          <Button
            onClick={async () => {
              try {
                await mutateUpdateAccount({
                  accountId,
                  credential,
                  password,
                });
                toast.success("ログイン情報を更新しました");
              } catch (e) {
                toast.error(
                  e instanceof Error
                    ? e.message
                    : "ログイン情報の更新に失敗しました",
                );
                return;
              }
              onClose?.();
            }}
            disabled={isPending}
          >
            更新
          </Button>
          <DialogClose asChild>
            <Button variant="outline">キャンセル</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function StorageInfoTip({ accountId }: { accountId: string }) {
  const { data, isPending } = useQuery(getHeadlessAccountStorageInfo, {
    accountId,
  });

  return isPending ? (
    <Skeleton className="h-4 w-8 rounded" />
  ) : (
    <span>
      {prettyBytes(Number(data?.storageUsedBytes))}/
      {prettyBytes(Number(data?.storageQuotaBytes))}
    </span>
  );
}

export default function HeadlessAccountList() {
  const { pageIndex, pageSize, setPageIndex, setPageSize } = usePaginationState(
    { defaultPageSize: 20 },
  );
  const currentGroupId = useAtomValue(currentGroupIdAtom);
  const { data, isPending, refetch } = useQuery(
    listHeadlessAccounts,
    {
      page: { pageIndex, pageSize },
      groupId: currentGroupId ?? undefined,
    },
    { placeholderData: keepPreviousData },
  );
  const { hasPermission, groupsWithPermission } = usePermissions();
  const canCreate =
    groupsWithPermission(PERMISSION_KEYS.ACCOUNT_WRITE).length > 0;

  const { mutateAsync: mutateDeleteAccount, isPending: isPendingDelete } =
    useMutation(deleteHeadlessAccount);
  const { mutateAsync: mutateRefetchAccountInfo, isPending: isPendingRefetch } =
    useMutation(refetchHeadlessAccountInfo);
  const { mutateAsync: mutateUpdateIcon, isPending: isUpdatingIcon } =
    useMutation(updateHeadlessAccountIcon);
  const [updateDialogAccountId, setUpdateDialogAccountId] = useState<string>();
  const [isOpenNewAccountDialog, setIsOpenNewAccountDialog] = useState(false);
  const [actionAccountId, setActionAccountId] = useState<string | null>(null);
  const [iconChangeAccount, setIconChangeAccount] = useState<{
    userId: string;
    iconUrl: string;
  }>();
  const [chatAccount, setChatAccount] = useState<{
    userId: string;
    userName: string;
  }>();
  const [sendFriendReqAccountId, setSendFriendReqAccountId] =
    useState<string>();

  const handleChangeIcon = useCallback((userId: string, iconUrl: string) => {
    setIconChangeAccount({ userId, iconUrl });
  }, []);

  const handleUploadIcon = useCallback(
    async (iconData: Uint8Array) => {
      if (!iconChangeAccount) return;
      await mutateUpdateIcon({
        accountId: iconChangeAccount.userId,
        iconData,
      });
      toast.success("アイコンを更新しました");
      refetch();
    },
    [iconChangeAccount, mutateUpdateIcon, refetch],
  );

  const handleRefetchInfo = useCallback(
    async (accountId: string) => {
      setActionAccountId(accountId);
      try {
        await mutateRefetchAccountInfo({ accountId });
        toast.success("アカウント情報を再取得しました");
        refetch();
      } catch (e) {
        toast.error(
          e instanceof Error
            ? e.message
            : "アカウント情報の再取得に失敗しました",
        );
      } finally {
        setActionAccountId(null);
      }
    },
    [mutateRefetchAccountInfo, refetch],
  );

  const handleDeleteAccount = useCallback(
    async (accountId: string) => {
      setActionAccountId(accountId);
      try {
        await mutateDeleteAccount({ accountId });
        toast.success("アカウントを削除しました");
        refetch();
      } catch (e) {
        toast.error(
          e instanceof Error ? e.message : "アカウントの削除に失敗しました",
        );
      } finally {
        setActionAccountId(null);
      }
    },
    [mutateDeleteAccount, refetch],
  );

  const columns: ColumnDef<HeadlessAccount>[] = useMemo(
    () => [
      {
        accessorKey: "iconUrl",
        header: "アイコン",
        cell: ({ row }) => {
          return (
            <ResoniteUserIcon
              iconUrl={row.original.iconUrl}
              alt={`${row.original.userName}のアイコン`}
            />
          );
        },
      },
      {
        accessorKey: "userName",
        header: "ユーザ名",
      },
      {
        header: "ストレージ",
        cell: ({ row }) => <StorageInfoTip accountId={row.original.userId} />,
      },
      {
        id: "friendRequests",
        header: "フレリク",
        cell: ({ row }) => (
          <FriendRequestsDialog accountId={row.original.userId} />
        ),
      },
      {
        id: "actions",
        header: "操作",
        cell: ({ row }) => {
          const canWrite = hasPermission(
            row.original.groupId,
            PERMISSION_KEYS.ACCOUNT_WRITE,
          );
          return (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost">
                  <MoreVertical />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                <DropdownMenuItem
                  onClick={() =>
                    setChatAccount({
                      userId: row.original.userId,
                      userName: row.original.userName,
                    })
                  }
                >
                  チャットを開く
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={!canWrite}
                  onClick={() => setSendFriendReqAccountId(row.original.userId)}
                >
                  フレンド追加
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={!canWrite}
                  onClick={() => setUpdateDialogAccountId(row.original.userId)}
                >
                  ログイン情報の更新
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={!canWrite}
                  onClick={() =>
                    handleChangeIcon(
                      row.original.userId,
                      resolveUrl(row.original.iconUrl) ?? "",
                    )
                  }
                >
                  アイコンを変更
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={
                    !canWrite ||
                    (isPendingRefetch &&
                      actionAccountId === row.original.userId)
                  }
                  onClick={() => handleRefetchInfo(row.original.userId)}
                >
                  名前とアイコンの再取得
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={
                    !canWrite ||
                    (isPendingDelete && actionAccountId === row.original.userId)
                  }
                  onClick={() => handleDeleteAccount(row.original.userId)}
                >
                  削除
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          );
        },
      },
    ],
    [
      setUpdateDialogAccountId,
      handleRefetchInfo,
      handleDeleteAccount,
      handleChangeIcon,
      isPendingRefetch,
      isPendingDelete,
      actionAccountId,
      hasPermission,
    ],
  );

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <PermissionGuardedButton
          allowed={canCreate}
          disabledReason="アカウントを作成できる権限を持つグループがありません"
          onClick={() => setIsOpenNewAccountDialog(true)}
        >
          追加
        </PermissionGuardedButton>
      </div>
      <DataTable
        columns={columns}
        data={data?.accounts || []}
        isLoading={isPending}
        pagination={{
          pageIndex,
          pageSize,
          totalCount: data?.page?.totalCount ?? 0,
          onPageIndexChange: setPageIndex,
          onPageSizeChange: setPageSize,
        }}
      />
      <NewAccountDialog
        open={isOpenNewAccountDialog}
        onClose={() => {
          setIsOpenNewAccountDialog(false);
          refetch();
        }}
      />
      <UpdateAccountCredentialsDialog
        accountId={updateDialogAccountId ?? ""}
        open={!!updateDialogAccountId}
        onClose={() => {
          setUpdateDialogAccountId(undefined);
          refetch();
        }}
      />
      <IconChangeDialog
        open={!!iconChangeAccount}
        onClose={() => setIconChangeAccount(undefined)}
        currentIconUrl={iconChangeAccount?.iconUrl}
        onUpload={handleUploadIcon}
        isUploading={isUpdatingIcon}
      />
      <ChatDialog
        open={!!chatAccount}
        onClose={() => setChatAccount(undefined)}
        accountId={chatAccount?.userId ?? ""}
        accountName={chatAccount?.userName ?? ""}
      />
      <SendFriendRequestDialog
        accountId={sendFriendReqAccountId ?? ""}
        open={!!sendFriendReqAccountId}
        onClose={() => setSendFriendReqAccountId(undefined)}
      />
    </div>
  );
}
