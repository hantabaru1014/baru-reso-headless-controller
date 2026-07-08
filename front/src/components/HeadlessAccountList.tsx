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
import {
  callUnaryMethod,
  useMutation,
  useQuery,
  useTransport,
} from "@connectrpc/connect-query";
import {
  acceptFriendRequests,
  createHeadlessAccount,
  deleteHeadlessAccount,
  getFriendRequests,
  getHeadlessAccountStorageInfo,
  getResoniteUser,
  listContacts,
  listHeadlessAccounts,
  listHeadlessHost,
  refetchHeadlessAccountInfo,
  removeContact,
  searchResoniteUsers,
  sendFriendRequest,
  updateHeadlessAccountCredentials,
  updateHeadlessAccountIcon,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { RefetchButton } from "./base/RefetchButton";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { keepPreviousData, useInfiniteQuery } from "@tanstack/react-query";
import { usePaginationState } from "../hooks/usePaginationState";
import { toast } from "sonner";
import { DataTable, TextField } from "./base";
import { ColumnDef } from "@tanstack/react-table";
import {
  HeadlessAccount,
  HeadlessHostStatus,
  UserInfo,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import prettyBytes from "@/libs/prettyBytes";
import { resolveUrl } from "@/libs/skyfrostUtils";
import { MoreVertical, Search } from "lucide-react";
import { Input } from "./ui";
import { UserList } from "./base/UserList";
import { ScrollBase } from "./base/ScrollBase";
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
import { useTranslation } from "react-i18next";

function FriendRequestsDialog({
  onClose,
  accountId,
}: {
  onClose?: () => void;
  accountId: string;
}) {
  const { t } = useTranslation();
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
      header: t("headlessAccountList.icon"),
      cell: ({ row }) => (
        <ResoniteUserIcon
          iconUrl={row.original.iconUrl}
          alt={t("headlessAccountList.userIconAlt", {
            name: row.original.name,
          })}
        />
      ),
    },
    {
      accessorKey: "name",
      header: t("common.name"),
    },
    {
      id: "actions",
      header: t("headlessAccountList.action"),
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
                  toast.success(t("headlessAccountList.friendRequestRejected"));
                } catch (e) {
                  toast.error(
                    e instanceof Error
                      ? e.message
                      : t("headlessAccountList.friendRequestRejectFailed"),
                  );
                } finally {
                  setActionUserId(null);
                }
              }}
              disabled={isBusy}
            >
              {t("headlessAccountList.reject")}
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
                  toast.success(t("headlessAccountList.friendRequestAccepted"));
                } catch (e) {
                  toast.error(
                    e instanceof Error
                      ? e.message
                      : t("headlessAccountList.friendRequestAcceptFailed"),
                  );
                } finally {
                  setActionUserId(null);
                }
              }}
              disabled={isBusy}
            >
              {t("headlessAccountList.accept")}
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
          <Button
            variant="ghost"
            title={t("headlessAccountList.openFriendRequests")}
          >
            <Badge variant="default">
              {data?.requestedContacts.length ?? 0}
            </Badge>
          </Button>
        </DialogTrigger>
      )}
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader className="flex justify-between">
          <DialogTitle>{t("headlessAccountList.friendRequests")}</DialogTitle>
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
            <Button variant="outline">{t("common.close")}</Button>
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
  const { t } = useTranslation();
  const inputRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState("");
  const transport = useTransport();

  // フレンド申請 (SendFriendRequest) は container 経由で cloud を叩くため、
  // 対象アカウントで起動中のホストが必要. 検索側は Resonite の公開 API を叩く
  // controller RPC (SearchResoniteUsers / GetResoniteUser) を使うのでホスト不要.
  const { data: hostsData } = useQuery(
    listHeadlessHost,
    { page: { pageIndex: 0, pageSize: 100 } },
    { enabled: open },
  );
  const runningHost = useMemo(
    () =>
      hostsData?.hosts.find(
        (h) =>
          h.accountId === accountId && h.status === HeadlessHostStatus.RUNNING,
      ),
    [hostsData, accountId],
  );
  const hasRunningHost = !!runningHost;

  // 既存フレンド (Accepted) を除外するため全 contacts を取得.
  // ListContacts は container 経由なので、起動中ホストが必要. 無い時は空扱いで検索は動かす.
  const {
    data: contactsData,
    fetchNextPage: fetchNextContacts,
    hasNextPage: hasMoreContacts,
    isFetchingNextPage: isFetchingMoreContacts,
  } = useInfiniteQuery({
    queryKey: ["allContacts", accountId],
    queryFn: async ({ pageParam }) => {
      const res = await callUnaryMethod(transport, listContacts, {
        headlessAccountId: accountId,
        limit: 200,
        cursor: pageParam?.cursor,
      });
      return { contacts: res.contacts, nextCursor: res.nextCursor };
    },
    initialPageParam: undefined as { cursor?: string } | undefined,
    getNextPageParam: (last) =>
      last.nextCursor ? { cursor: last.nextCursor } : undefined,
    enabled: open && hasRunningHost,
  });

  // 未取得ページが残っていれば自動で次を fetch (dialog を開いた瞬間から順次全ページを取り切る)
  useEffect(() => {
    if (open && hasMoreContacts && !isFetchingMoreContacts) {
      fetchNextContacts();
    }
  }, [open, hasMoreContacts, isFetchingMoreContacts, fetchNextContacts]);

  const contactIdSet = useMemo(() => {
    const s = new Set<string>();
    contactsData?.pages.forEach((p) => p.contacts.forEach((c) => s.add(c.id)));
    return s;
  }, [contactsData]);

  // 名前検索: Resonite Cloud の GET /users?name=... を叩く controller RPC.
  const {
    data: searchResult,
    mutateAsync: mutateSearch,
    isPending: isPendingSearch,
    reset: resetSearch,
  } = useMutation(searchResoniteUsers);
  // ID 検索: 部分一致は非対応. 完全一致で GET /users/{id} を叩く RPC.
  const {
    data: idLookupResult,
    mutateAsync: mutateIdLookup,
    isPending: isPendingIdLookup,
    reset: resetIdLookup,
  } = useMutation(getResoniteUser);
  const { mutateAsync: mutateSendFriendRequest, isPending: isPendingSend } =
    useMutation(sendFriendRequest);
  const [sendingUserId, setSendingUserId] = useState<string | null>(null);

  const rawResults = useMemo(() => {
    if (idLookupResult) {
      return [
        {
          id: idLookupResult.id,
          name: idLookupResult.name,
          iconUrl: idLookupResult.iconUrl,
        },
      ];
    }
    return searchResult?.users ?? [];
  }, [idLookupResult, searchResult]);

  const visibleUsers = useMemo(
    () => rawResults.filter((u) => !contactIdSet.has(u.id)),
    [rawResults, contactIdSet],
  );

  const handleQueryChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setQuery(value);
    const trimmed = value.trim();
    if (!trimmed) {
      resetSearch();
      resetIdLookup();
      return;
    }
    // "U-" prefix なら ID 完全一致 lookup、それ以外は name の部分一致検索.
    // Cloud API は case-insensitive なので入力の大小を保つ.
    if (trimmed.toLowerCase().startsWith("u-")) {
      resetSearch();
      mutateIdLookup({ resoniteId: trimmed }).catch(() => {
        // 未存在などは無視 (毎打鍵で toast すると煩い)
      });
    } else {
      resetIdLookup();
      mutateSearch({ name: trimmed }).catch(() => {
        // 空結果や API エラーは無視
      });
    }
  };

  const handleSend = async (userId: string) => {
    setSendingUserId(userId);
    try {
      await mutateSendFriendRequest({
        headlessAccountId: accountId,
        user: { case: "userId", value: userId },
      });
      toast.success(t("headlessAccountList.friendRequestSent"));
      setQuery("");
      resetSearch();
      resetIdLookup();
      inputRef.current?.focus();
    } catch (e) {
      toast.error(
        e instanceof Error
          ? e.message
          : t("headlessAccountList.friendRequestSendFailed"),
      );
    } finally {
      setSendingUserId(null);
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) {
          setQuery("");
          resetSearch();
          resetIdLookup();
          onClose?.();
        }
      }}
    >
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("headlessAccountList.addFriend")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          {!hasRunningHost && (
            <p className="text-sm text-destructive">
              {t("headlessAccountList.runningHostRequired")}
            </p>
          )}
          <div className="relative">
            <Search className="absolute left-3 top-3 h-4 w-4 text-gray-400" />
            <Input
              ref={inputRef}
              placeholder={t("headlessAccountList.searchPlaceholder")}
              value={query}
              onChange={handleQueryChange}
              className="pl-10"
            />
          </div>
          <ScrollBase height="60vh">
            <UserList
              data={visibleUsers}
              isLoading={isPendingSearch || isPendingIdLookup}
              renderActions={(user) => {
                const isLoading = isPendingSend && sendingUserId === user.id;
                return (
                  <Button
                    onClick={() => handleSend(user.id)}
                    disabled={isLoading || !hasRunningHost}
                  >
                    {isLoading
                      ? t("common.sending")
                      : t("headlessAccountList.apply")}
                  </Button>
                );
              }}
            />
            {!isPendingSearch &&
              !isPendingIdLookup &&
              query.trim() !== "" &&
              visibleUsers.length === 0 && (
                <p className="text-center text-sm text-muted-foreground py-4">
                  {t("headlessAccountList.noMatchingUsers")}
                  {rawResults.length > 0 &&
                    t("headlessAccountList.alreadyFriendExcluded")}
                </p>
              )}
          </ScrollBase>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">{t("common.close")}</Button>
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
  const { t } = useTranslation();
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
          <DialogTitle>{t("headlessAccountList.addAccountTitle")}</DialogTitle>
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
            helperText={t("headlessAccountList.accountGroupHelper")}
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
                toast.success(t("headlessAccountList.accountAdded"));
              } catch (e) {
                toast.error(
                  e instanceof Error
                    ? e.message
                    : t("headlessAccountList.accountAddFailed"),
                );
                return;
              }
              onClose?.();
            }}
            disabled={isPending || !groupId}
          >
            {t("common.add")}
          </Button>
          <DialogClose asChild>
            <Button variant="outline">{t("common.cancel")}</Button>
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
  const { t } = useTranslation();
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
          <DialogTitle>
            {t("headlessAccountList.updateCredentialsTitle")}
          </DialogTitle>
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
                toast.success(t("headlessAccountList.credentialsUpdated"));
              } catch (e) {
                toast.error(
                  e instanceof Error
                    ? e.message
                    : t("headlessAccountList.credentialsUpdateFailed"),
                );
                return;
              }
              onClose?.();
            }}
            disabled={isPending}
          >
            {t("common.update")}
          </Button>
          <DialogClose asChild>
            <Button variant="outline">{t("common.cancel")}</Button>
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
  const { t } = useTranslation();
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
      toast.success(t("headlessAccountList.iconUpdated"));
      refetch();
    },
    [iconChangeAccount, mutateUpdateIcon, refetch, t],
  );

  const handleRefetchInfo = useCallback(
    async (accountId: string) => {
      setActionAccountId(accountId);
      try {
        await mutateRefetchAccountInfo({ accountId });
        toast.success(t("headlessAccountList.accountInfoRefetched"));
        refetch();
      } catch (e) {
        toast.error(
          e instanceof Error
            ? e.message
            : t("headlessAccountList.accountInfoRefetchFailed"),
        );
      } finally {
        setActionAccountId(null);
      }
    },
    [mutateRefetchAccountInfo, refetch, t],
  );

  const handleDeleteAccount = useCallback(
    async (accountId: string) => {
      setActionAccountId(accountId);
      try {
        await mutateDeleteAccount({ accountId });
        toast.success(t("headlessAccountList.accountDeleted"));
        refetch();
      } catch (e) {
        toast.error(
          e instanceof Error
            ? e.message
            : t("headlessAccountList.accountDeleteFailed"),
        );
      } finally {
        setActionAccountId(null);
      }
    },
    [mutateDeleteAccount, refetch, t],
  );

  const columns: ColumnDef<HeadlessAccount>[] = useMemo(
    () => [
      {
        accessorKey: "iconUrl",
        header: t("headlessAccountList.icon"),
        cell: ({ row }) => {
          return (
            <ResoniteUserIcon
              iconUrl={row.original.iconUrl}
              alt={t("headlessAccountList.userIconAlt", {
                name: row.original.userName,
              })}
            />
          );
        },
      },
      {
        accessorKey: "userName",
        header: t("headlessAccountList.userName"),
      },
      {
        header: t("headlessAccountList.storage"),
        cell: ({ row }) => <StorageInfoTip accountId={row.original.userId} />,
      },
      {
        id: "friendRequests",
        header: t("headlessAccountList.friendReq"),
        cell: ({ row }) => (
          <FriendRequestsDialog accountId={row.original.userId} />
        ),
      },
      {
        id: "actions",
        header: t("common.actions"),
        cell: ({ row }) => {
          const canWrite = hasPermission(
            row.original.groupId,
            PERMISSION_KEYS.ACCOUNT_WRITE,
          );
          const canUse = hasPermission(
            row.original.groupId,
            PERMISSION_KEYS.ACCOUNT_USE,
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
                  disabled={!canUse}
                  onClick={() =>
                    setChatAccount({
                      userId: row.original.userId,
                      userName: row.original.userName,
                    })
                  }
                >
                  {t("headlessAccountList.openChat")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={!canWrite}
                  onClick={() => setSendFriendReqAccountId(row.original.userId)}
                >
                  {t("headlessAccountList.addFriend")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={!canWrite}
                  onClick={() => setUpdateDialogAccountId(row.original.userId)}
                >
                  {t("headlessAccountList.updateCredentials")}
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
                  {t("headlessAccountList.changeIcon")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={
                    !canWrite ||
                    (isPendingRefetch &&
                      actionAccountId === row.original.userId)
                  }
                  onClick={() => handleRefetchInfo(row.original.userId)}
                >
                  {t("headlessAccountList.refetchNameIcon")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={
                    !canWrite ||
                    (isPendingDelete && actionAccountId === row.original.userId)
                  }
                  onClick={() => handleDeleteAccount(row.original.userId)}
                >
                  {t("common.delete")}
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
      t,
    ],
  );

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <PermissionGuardedButton
          allowed={canCreate}
          disabledReason={t("headlessAccountList.noCreatePermission")}
          onClick={() => setIsOpenNewAccountDialog(true)}
        >
          {t("common.add")}
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
