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
  updateHeadlessAccountCredentials,
  updateHeadlessAccountIcon,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { RefetchButton } from "./base/RefetchButton";
import { useCallback, useMemo, useState } from "react";
import { toast } from "sonner";
import { DataTable, TextField } from "./base";
import { ColumnDef } from "@tanstack/react-table";
import {
  HeadlessAccount,
  UserInfo,
} from "front/pbgen/hdlctrl/v1/controller_pb";
import prettyBytes from "@/libs/prettyBytes";
import { resolveUrl } from "@/libs/skyfrostUtils";
import { MoreVertical } from "lucide-react";
import { IconChangeDialog } from "./IconChangeDialog";
import { ResoniteUserIcon } from "./ResoniteUserIcon";
import { ChatDialog } from "./chat";

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
  const { mutateAsync: mutateAcceptFriendRequest } =
    useMutation(acceptFriendRequests);

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
      cell: ({ row }) => (
        <Button
          onClick={async () => {
            try {
              await mutateAcceptFriendRequest({
                headlessAccountId: accountId,
                targetUserId: row.original.id,
              });
              refetch();
            } catch (e) {
              toast.error(
                e instanceof Error
                  ? e.message
                  : "フレンドリクエストの承認に失敗しました",
              );
              return;
            }
            toast.success("フレンドリクエストを承認しました");
          }}
          className="w-full"
        >
          承認
        </Button>
      ),
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
        </div>
        <DialogFooter>
          <Button
            onClick={async () => {
              try {
                await mutateCreateAccount({
                  credential,
                  password,
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
            disabled={isPending}
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
  const { data, isPending, refetch } = useQuery(listHeadlessAccounts);
  const { mutateAsync: mutateDeleteAccount } = useMutation(
    deleteHeadlessAccount,
  );
  const { mutateAsync: mutateRefetchAccountInfo } = useMutation(
    refetchHeadlessAccountInfo,
  );
  const { mutateAsync: mutateUpdateIcon, isPending: isUpdatingIcon } =
    useMutation(updateHeadlessAccountIcon);
  const [updateDialogAccountId, setUpdateDialogAccountId] = useState<string>();
  const [isOpenNewAccountDialog, setIsOpenNewAccountDialog] = useState(false);
  const [iconChangeAccount, setIconChangeAccount] = useState<{
    userId: string;
    iconUrl: string;
  }>();
  const [chatAccount, setChatAccount] = useState<{
    userId: string;
    userName: string;
  }>();

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
      }
    },
    [mutateRefetchAccountInfo, refetch],
  );

  const handleDeleteAccount = useCallback(
    async (accountId: string) => {
      try {
        await mutateDeleteAccount({ accountId });
        toast.success("アカウントを削除しました");
        refetch();
      } catch (e) {
        toast.error(
          e instanceof Error ? e.message : "アカウントの削除に失敗しました",
        );
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
        cell: ({ row }) => (
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
                onClick={() => setUpdateDialogAccountId(row.original.userId)}
              >
                ログイン情報の更新
              </DropdownMenuItem>
              <DropdownMenuItem
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
                onClick={() => handleRefetchInfo(row.original.userId)}
              >
                名前とアイコンの再取得
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => handleDeleteAccount(row.original.userId)}
              >
                削除
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        ),
      },
    ],
    [
      setUpdateDialogAccountId,
      handleRefetchInfo,
      handleDeleteAccount,
      handleChangeIcon,
    ],
  );

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <Button onClick={() => setIsOpenNewAccountDialog(true)}>追加</Button>
      </div>
      <DataTable
        columns={columns}
        data={data?.accounts || []}
        isLoading={isPending}
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
    </div>
  );
}
