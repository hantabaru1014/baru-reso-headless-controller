import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
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
  registerHeadlessAccount,
  removeContact,
  searchResoniteUsers,
  sendFriendRequest,
  updateHeadlessAccountCredentials,
  updateHeadlessAccountIcon,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { listGroups } from "../../pbgen/hdlctrl/v1/permission-GroupService_connectquery";
import { RefetchButton } from "./base/RefetchButton";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { keepPreviousData, useInfiniteQuery } from "@tanstack/react-query";
import { usePaginationState } from "../hooks/usePaginationState";
import { toast } from "sonner";
import { DataTable, FormSection, TextField } from "./base";
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
import { ResourceTransferDialog } from "./ResourceTransferDialog";
import { GroupSelectField } from "./GroupSelectField";
import { PermissionGuardedButton } from "./base/PermissionGuardedButton";
import { usePermissions } from "../hooks/usePermissions";
import { useDefaultGroupId } from "../hooks/useDefaultGroupId";
import { useAtomValue } from "jotai";
import { currentGroupIdAtom } from "../atoms/currentGroupAtom";
import { PERMISSION_KEYS } from "../libs/permissionUtils";
import { useTranslation } from "react-i18next";

// 同一の Resonite アカウントを複数グループに登録できるため、
// アカウントは groupId + userId の組で特定する.
type AccountRef = { groupId: string; userId: string };

const isSameAccount = (a: AccountRef | null | undefined, b: AccountRef) =>
  a?.groupId === b.groupId && a.userId === b.userId;

function FriendRequestsDialog({
  onClose,
  groupId,
  accountId,
}: {
  onClose?: () => void;
  groupId: string;
  accountId: string;
}) {
  const { t } = useTranslation();
  const { data, isPending, refetch } = useQuery(getFriendRequests, {
    headlessAccountId: accountId,
    groupId,
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
                    groupId,
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
                    groupId,
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
  groupId,
  accountId,
  open,
  onClose,
}: {
  groupId: string;
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
    { page: { pageIndex: 0, pageSize: 100 }, groupId },
    { enabled: open },
  );
  const runningHost = useMemo(
    () =>
      hostsData?.hosts.find(
        (h) =>
          h.groupId === groupId &&
          h.accountId === accountId &&
          h.status === HeadlessHostStatus.RUNNING,
      ),
    [hostsData, groupId, accountId],
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
    queryKey: ["allContacts", groupId, accountId],
    queryFn: async ({ pageParam }) => {
      const res = await callUnaryMethod(transport, listContacts, {
        headlessAccountId: accountId,
        groupId,
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
        groupId,
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

// Resonite の登録条件 (SkyFrost の RegistrationRequest) と同じ.
const isValidResonitePassword = (password: string) =>
  password.length >= 8 &&
  /\d/.test(password) &&
  /\p{Ll}/u.test(password) &&
  /\p{Lu}/u.test(password);

const isOldEnoughForResonite = (dateOfBirth: string) => {
  const limit = new Date();
  limit.setFullYear(limit.getFullYear() - 16);
  return new Date(dateOfBirth) <= limit;
};

// 開くたびに入力をリセットするため、呼び出し側は開いている間だけマウントする.
function RegisterAccountDialog({
  onClose,
  onRegistered,
}: {
  onClose?: () => void;
  onRegistered?: (account: AccountRef) => void;
}) {
  const { t } = useTranslation();
  const { mutateAsync: mutateRegisterAccount, isPending } = useMutation(
    registerHeadlessAccount,
  );
  const defaultGroupId = useDefaultGroupId(PERMISSION_KEYS.ACCOUNT_WRITE);
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [passwordConfirm, setPasswordConfirm] = useState("");
  const [dateOfBirth, setDateOfBirth] = useState("");
  const [groupId, setGroupId] = useState(defaultGroupId);

  // グループ一覧の取得完了後にコンテキストグループを初期値として埋める.
  useEffect(() => {
    if (defaultGroupId && !groupId) {
      setGroupId(defaultGroupId);
    }
  }, [defaultGroupId, groupId]);

  const passwordError =
    password && !isValidResonitePassword(password)
      ? t("headlessAccountList.register.passwordRule")
      : undefined;
  const passwordConfirmError =
    passwordConfirm && password !== passwordConfirm
      ? t("headlessAccountList.register.passwordMismatch")
      : undefined;
  const dateOfBirthError =
    dateOfBirth && !isOldEnoughForResonite(dateOfBirth)
      ? t("headlessAccountList.register.ageRule")
      : undefined;
  const canSubmit =
    !!username.trim() &&
    !!email.trim() &&
    isValidResonitePassword(password) &&
    password === passwordConfirm &&
    !!dateOfBirth &&
    !dateOfBirthError &&
    !!groupId;

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !isPending) onClose?.();
      }}
    >
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>{t("headlessAccountList.register.title")}</DialogTitle>
          <DialogDescription>
            {t("headlessAccountList.register.description")}
          </DialogDescription>
        </DialogHeader>
        <fieldset disabled={isPending} className="grid gap-4 py-2">
          <FormSection
            title={t("headlessAccountList.register.resoniteSection")}
            caption={t("headlessAccountList.register.resoniteSectionCaption")}
          >
            <TextField
              label={t("headlessAccountList.register.username")}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
            <TextField
              label={t("headlessAccountList.register.email")}
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
            <TextField
              label={t("headlessAccountList.register.password")}
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              error={passwordError}
            />
            <TextField
              label={t("headlessAccountList.register.passwordConfirm")}
              type="password"
              autoComplete="new-password"
              value={passwordConfirm}
              onChange={(e) => setPasswordConfirm(e.target.value)}
              error={passwordConfirmError}
            />
            <TextField
              label={t("headlessAccountList.register.dateOfBirth")}
              type="date"
              value={dateOfBirth}
              onChange={(e) => setDateOfBirth(e.target.value)}
              error={dateOfBirthError}
            />
          </FormSection>
          <FormSection
            title={t("headlessAccountList.register.appSection")}
            caption={t("headlessAccountList.register.appSectionCaption")}
          >
            <GroupSelectField
              value={groupId}
              onChange={setGroupId}
              requiredPermission={PERMISSION_KEYS.ACCOUNT_WRITE}
              helperText={t("headlessAccountList.accountGroupHelper")}
            />
          </FormSection>
        </fieldset>
        <DialogFooter>
          <Button
            onClick={async () => {
              try {
                const res = await mutateRegisterAccount({
                  username,
                  email,
                  password,
                  dateOfBirth,
                  groupId: groupId || undefined,
                });
                toast.success(t("headlessAccountList.register.registered"));
                onRegistered?.({ groupId: res.groupId, userId: res.accountId });
              } catch (e) {
                toast.error(
                  e instanceof Error
                    ? e.message
                    : t("headlessAccountList.register.failed"),
                );
              }
            }}
            disabled={isPending || !canSubmit}
          >
            {isPending
              ? t("headlessAccountList.register.registering")
              : t("headlessAccountList.register.submit")}
          </Button>
          <DialogClose asChild>
            <Button variant="outline" disabled={isPending}>
              {t("common.cancel")}
            </Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function UpdateAccountCredentialsDialog({
  groupId,
  accountId,
  open,
  onClose,
}: {
  groupId: string;
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
                  groupId,
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

function StorageInfoTip({
  groupId,
  accountId,
}: {
  groupId: string;
  accountId: string;
}) {
  const { data, isPending } = useQuery(getHeadlessAccountStorageInfo, {
    accountId,
    groupId,
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
  const [updateDialogAccount, setUpdateDialogAccount] = useState<AccountRef>();
  const [isOpenNewAccountDialog, setIsOpenNewAccountDialog] = useState(false);
  const [isOpenRegisterDialog, setIsOpenRegisterDialog] = useState(false);
  const [actionAccount, setActionAccount] = useState<AccountRef | null>(null);
  // isNewAccount: 新規登録直後のアイコン設定ステップとして開いている.
  const [iconChangeAccount, setIconChangeAccount] = useState<
    AccountRef & { iconUrl: string; isNewAccount?: boolean; open: boolean }
  >();
  const [chatAccount, setChatAccount] = useState<
    AccountRef & { userName: string }
  >();
  const [sendFriendReqAccount, setSendFriendReqAccount] =
    useState<AccountRef>();
  const [transferAccount, setTransferAccount] = useState<AccountRef>();

  // 全グループ表示では同じアカウントが複数行に並びうるので、所属グループ名で区別できるようにする.
  const { data: groupsData } = useQuery(listGroups, {});
  const groupNameById = useMemo(() => {
    const m = new Map<string, string>();
    for (const g of groupsData?.groups ?? []) m.set(g.id, g.name);
    return m;
  }, [groupsData?.groups]);

  const handleUploadIcon = useCallback(
    async (iconData: Uint8Array) => {
      if (!iconChangeAccount) return;
      await mutateUpdateIcon({
        accountId: iconChangeAccount.userId,
        groupId: iconChangeAccount.groupId,
        iconData,
      });
      toast.success(t("headlessAccountList.iconUpdated"));
      refetch();
    },
    [iconChangeAccount, mutateUpdateIcon, refetch, t],
  );

  const handleRefetchInfo = useCallback(
    async ({ groupId, userId }: AccountRef) => {
      setActionAccount({ groupId, userId });
      try {
        await mutateRefetchAccountInfo({ accountId: userId, groupId });
        toast.success(t("headlessAccountList.accountInfoRefetched"));
        refetch();
      } catch (e) {
        toast.error(
          e instanceof Error
            ? e.message
            : t("headlessAccountList.accountInfoRefetchFailed"),
        );
      } finally {
        setActionAccount(null);
      }
    },
    [mutateRefetchAccountInfo, refetch, t],
  );

  const handleDeleteAccount = useCallback(
    async ({ groupId, userId }: AccountRef) => {
      setActionAccount({ groupId, userId });
      try {
        await mutateDeleteAccount({ accountId: userId, groupId });
        toast.success(t("headlessAccountList.accountDeleted"));
        refetch();
      } catch (e) {
        toast.error(
          e instanceof Error
            ? e.message
            : t("headlessAccountList.accountDeleteFailed"),
        );
      } finally {
        setActionAccount(null);
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
      ...(currentGroupId
        ? []
        : [
            {
              accessorKey: "groupId",
              header: t("headlessAccountList.group"),
              cell: ({ row }) =>
                groupNameById.get(row.original.groupId) ?? row.original.groupId,
            } satisfies ColumnDef<HeadlessAccount>,
          ]),
      {
        header: t("headlessAccountList.storage"),
        cell: ({ row }) => (
          <StorageInfoTip
            groupId={row.original.groupId}
            accountId={row.original.userId}
          />
        ),
      },
      {
        id: "friendRequests",
        header: t("headlessAccountList.friendReq"),
        cell: ({ row }) => (
          <FriendRequestsDialog
            groupId={row.original.groupId}
            accountId={row.original.userId}
          />
        ),
      },
      {
        id: "actions",
        header: t("common.actions"),
        cell: ({ row }) => {
          const account: AccountRef = {
            groupId: row.original.groupId,
            userId: row.original.userId,
          };
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
                      ...account,
                      userName: row.original.userName,
                    })
                  }
                >
                  {t("headlessAccountList.openChat")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={!canWrite}
                  onClick={() => setSendFriendReqAccount(account)}
                >
                  {t("headlessAccountList.addFriend")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={!canWrite}
                  onClick={() => setUpdateDialogAccount(account)}
                >
                  {t("headlessAccountList.updateCredentials")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={!canWrite}
                  onClick={() =>
                    setIconChangeAccount({
                      ...account,
                      iconUrl: resolveUrl(row.original.iconUrl) ?? "",
                      open: true,
                    })
                  }
                >
                  {t("headlessAccountList.changeIcon")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={
                    !canWrite ||
                    (isPendingRefetch && isSameAccount(actionAccount, account))
                  }
                  onClick={() => handleRefetchInfo(account)}
                >
                  {t("headlessAccountList.refetchNameIcon")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={!canWrite}
                  onClick={() => setTransferAccount(account)}
                >
                  {t("resourceTransferDialog.title")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={
                    !canWrite ||
                    (isPendingDelete && isSameAccount(actionAccount, account))
                  }
                  onClick={() => handleDeleteAccount(account)}
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
      handleRefetchInfo,
      handleDeleteAccount,
      isPendingRefetch,
      isPendingDelete,
      actionAccount,
      hasPermission,
      currentGroupId,
      groupNameById,
      t,
    ],
  );

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <PermissionGuardedButton
          variant="outline"
          allowed={canCreate}
          disabledReason={t("headlessAccountList.noCreatePermission")}
          onClick={() => setIsOpenRegisterDialog(true)}
        >
          {t("headlessAccountList.register.open")}
        </PermissionGuardedButton>
        <PermissionGuardedButton
          allowed={canCreate}
          disabledReason={t("headlessAccountList.noCreatePermission")}
          onClick={() => setIsOpenNewAccountDialog(true)}
        >
          {t("headlessAccountList.addAccountTitle")}
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
      {isOpenRegisterDialog && (
        <RegisterAccountDialog
          onClose={() => setIsOpenRegisterDialog(false)}
          onRegistered={(account) => {
            setIsOpenRegisterDialog(false);
            refetch();
            setIconChangeAccount({
              ...account,
              iconUrl: "",
              isNewAccount: true,
              open: true,
            });
          }}
        />
      )}
      <UpdateAccountCredentialsDialog
        groupId={updateDialogAccount?.groupId ?? ""}
        accountId={updateDialogAccount?.userId ?? ""}
        open={!!updateDialogAccount}
        onClose={() => {
          setUpdateDialogAccount(undefined);
          refetch();
        }}
      />
      <IconChangeDialog
        open={!!iconChangeAccount?.open}
        onClose={() =>
          // 閉じるアニメーション中に文言が既定に戻らないよう、内容は残して閉じる.
          setIconChangeAccount((a) => a && { ...a, open: false })
        }
        currentIconUrl={iconChangeAccount?.iconUrl}
        onUpload={handleUploadIcon}
        isUploading={isUpdatingIcon}
        {...(iconChangeAccount?.isNewAccount && {
          title: t("headlessAccountList.register.iconTitle"),
          description: t("headlessAccountList.register.iconDescription"),
          cancelLabel: t("headlessAccountList.register.skipIcon"),
        })}
      />
      <ChatDialog
        open={!!chatAccount}
        onClose={() => setChatAccount(undefined)}
        groupId={chatAccount?.groupId ?? ""}
        accountId={chatAccount?.userId ?? ""}
        accountName={chatAccount?.userName ?? ""}
      />
      <SendFriendRequestDialog
        groupId={sendFriendReqAccount?.groupId ?? ""}
        accountId={sendFriendReqAccount?.userId ?? ""}
        open={!!sendFriendReqAccount}
        onClose={() => setSendFriendReqAccount(undefined)}
      />
      <ResourceTransferDialog
        open={!!transferAccount}
        onClose={() => setTransferAccount(undefined)}
        resource={{
          case: "account",
          value: {
            groupId: transferAccount?.groupId ?? "",
            accountId: transferAccount?.userId ?? "",
          },
        }}
        sourceGroupId={transferAccount?.groupId ?? ""}
      />
    </div>
  );
}
