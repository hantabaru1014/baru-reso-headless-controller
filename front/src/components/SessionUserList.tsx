import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "./ui/dialog";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Search, Check } from "lucide-react";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  banUser,
  getHeadlessHost,
  getSessionDetails,
  inviteUser,
  kickUser,
  listUsersInSession,
  respawnUser,
  searchUserInfo,
  updateUserRole,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { UserInfoSchema } from "../../pbgen/hdlctrl/v1/controller_pb";
import { DirectChatDialog } from "./chat";
import { create } from "@bufbuild/protobuf";
import { UserRoles } from "../constants";
import { EditableSelectField } from "./base/EditableSelectField";
import { useRef, useState } from "react";
import { UserList } from "./base/UserList";
import { RefetchButton } from "./base/RefetchButton";
import { ScrollBase } from "./base/ScrollBase";
import { ColumnDef } from "@tanstack/react-table";
import { UserInSession as UserInSessionProto } from "../../pbgen/headless/v1/headless_pb";
import { DataTable } from "./base";
import { toast } from "sonner";
import { useMemo } from "react";
import { usePermissions } from "../hooks/usePermissions";
import { PERMISSION_KEYS } from "../libs/permissionUtils";
import { useTranslation } from "react-i18next";

const PAGE_SIZE_OPTIONS = [10, 20, 30];

function UserInviteDialog({
  isOpen: open,
  onClose,
  hostId,
  sessionId,
}: {
  isOpen: boolean;
  onClose: () => void;
  hostId?: string;
  sessionId?: string;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  const {
    data: searchResult,
    mutateAsync: mutateSearch,
    isPending: isPendingSearch,
  } = useMutation(searchUserInfo);
  const { mutateAsync: mutateInviteUser, isPending: isPendingInvite } =
    useMutation(inviteUser);
  const [invitingUserId, setInvitingUserId] = useState<string | null>(null);

  const handleQueryChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value.toLowerCase();
    setQuery(value);
    const isId = value.startsWith("u-");
    mutateSearch({
      hostId,
      parameters: {
        user: {
          case: isId ? "userId" : "userName",
          value,
        },
        onlyInContacts: true,
        partialMatch: true,
      },
    });
  };

  const handleInviteUser = async (userId: string) => {
    setInvitingUserId(userId);
    try {
      await mutateInviteUser({
        hostId,
        sessionId: sessionId,
        user: {
          case: "userId",
          value: userId,
        },
      });
      toast.success(t("sessionUserList.inviteSuccess"));
      setQuery("");
      inputRef.current?.focus();
    } catch (e) {
      toast.error(t("sessionUserList.inviteFailed", { error: e }));
    } finally {
      setInvitingUserId(null);
    }
  };

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("sessionUserList.inviteTitle")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="relative">
            <Search className="absolute left-3 top-3 h-4 w-4 text-gray-400" />
            <Input
              ref={inputRef}
              placeholder={t("sessionUserList.userIdOrNamePlaceholder")}
              value={query}
              onChange={handleQueryChange}
              className="pl-10"
            />
          </div>
          <ScrollBase height="60vh">
            <UserList
              data={searchResult?.users || []}
              isLoading={isPendingSearch}
              renderActions={(user) => {
                const isLoading = isPendingInvite && invitingUserId === user.id;
                return (
                  <Button
                    onClick={() => handleInviteUser(user.id)}
                    disabled={isLoading}
                  >
                    {isLoading
                      ? t("sessionUserList.inviting")
                      : t("sessionUserList.invite")}
                  </Button>
                );
              }}
            />
          </ScrollBase>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onClose()}>
            {t("sessionUserList.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

type UserInSession = Pick<
  UserInSessionProto,
  "id" | "name" | "role" | "isPresent"
> & {
  isHost: boolean;
};

export default function SessionUserList({ sessionId }: { sessionId: string }) {
  const { t } = useTranslation();
  const { data: sessionDetail } = useQuery(getSessionDetails, {
    sessionId,
  });
  const hostId = sessionDetail?.session?.hostId;
  const { data, isPending, refetch } = useQuery(listUsersInSession, {
    hostId,
    sessionId,
  });
  const { mutateAsync: mutateUpdateRole } = useMutation(updateUserRole);
  const { mutateAsync: mutateKickUser, isPending: isPendingKick } =
    useMutation(kickUser);
  const { mutateAsync: mutateBanUser, isPending: isPendingBan } =
    useMutation(banUser);
  const { mutateAsync: mutateRespawnUser, isPending: isPendingRespawn } =
    useMutation(respawnUser);
  const [isOpenInviteDialog, setIsOpenInviteDialog] = useState(false);
  const [chatUserId, setChatUserId] = useState<string | null>(null);
  const [actionUserId, setActionUserId] = useState<string | null>(null);

  // DirectChatDialog に渡す contact は id / name が同じ間は identity を保つ必要がある
  // (ChatMessagesPanel が contact を useEffect の dep に置いており、毎render新オブジェクトだと
  // ポーリング useEffect が毎回張り直され、非常に高頻度に再レンダーが発生する).
  const chatContactName = useMemo(
    () =>
      data?.users?.find((u) => u.id === chatUserId)?.name ?? chatUserId ?? "",
    [data?.users, chatUserId],
  );
  const chatContact = useMemo(
    () =>
      chatUserId
        ? create(UserInfoSchema, {
            id: chatUserId,
            name: chatContactName,
            iconUrl: "",
          })
        : null,
    [chatUserId, chatContactName],
  );

  const { data: hostData } = useQuery(getHeadlessHost, {
    hostId: hostId ?? "",
  });
  const { hasPermission } = usePermissions();
  // チャットはホストアカウントの account:use が必要 (backend の権限要件に合わせる).
  const canUseAccount = hasPermission(
    hostData?.host?.groupId,
    PERMISSION_KEYS.ACCOUNT_USE,
  );

  const [pageIndex, setPageIndex] = useState(0);
  const [pageSize, setPageSize] = useState(PAGE_SIZE_OPTIONS[0]);

  const users = useMemo<UserInSession[]>(
    () =>
      data?.users?.map((user, i) => ({
        ...user,
        isHost: i === 0, // TODO: ちゃんとホストユーザーのIDを返すようにする
      })) ?? [],
    [data?.users],
  );

  const totalPages = Math.max(1, Math.ceil(users.length / pageSize));
  const effectivePageIndex = Math.min(pageIndex, totalPages - 1);

  const pagedUsers = useMemo(
    () =>
      users.slice(
        effectivePageIndex * pageSize,
        (effectivePageIndex + 1) * pageSize,
      ),
    [users, effectivePageIndex, pageSize],
  );

  const handleUpdateRole = async (userId: string, role: string) => {
    try {
      await mutateUpdateRole({
        hostId,
        parameters: {
          sessionId,
          user: {
            case: "userId",
            value: userId,
          },
          role,
        },
      });
      setTimeout(() => {
        refetch();
      }, 500);
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  const handleKickUser = async (userId: string) => {
    setActionUserId(userId);
    try {
      await mutateKickUser({
        hostId,
        parameters: {
          sessionId,
          user: {
            case: "userId",
            value: userId,
          },
        },
      });
      setTimeout(() => {
        refetch();
      }, 500);
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    } finally {
      setActionUserId(null);
    }
  };

  const handleBanUser = async (userId: string) => {
    setActionUserId(userId);
    try {
      await mutateBanUser({
        hostId,
        parameters: {
          sessionId,
          user: {
            case: "userId",
            value: userId,
          },
        },
      });
      setTimeout(() => {
        refetch();
      }, 500);
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    } finally {
      setActionUserId(null);
    }
  };

  const handleRespawnUser = async (userId: string) => {
    setActionUserId(userId);
    try {
      await mutateRespawnUser({
        hostId,
        parameters: {
          sessionId,
          user: {
            case: "userId",
            value: userId,
          },
        },
      });
      toast.success(t("sessionUserList.respawnSuccess"));
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t("sessionUserList.respawnFailed"),
      );
    } finally {
      setActionUserId(null);
    }
  };

  const columns: ColumnDef<UserInSession>[] = [
    {
      accessorKey: "name",
      header: t("sessionUserList.columnName"),
    },
    {
      accessorKey: "role",
      header: t("sessionUserList.columnRole"),
      cell: ({ row }) => (
        <EditableSelectField<string>
          selectedId={row.original.role}
          options={UserRoles.map((r) => r)}
          onSave={(v) => handleUpdateRole(row.original.id, v)}
          readonly={row.original.isHost}
        />
      ),
    },
    {
      accessorKey: "isPresent",
      header: t("sessionUserList.columnAway"),
      cell: ({ cell }) =>
        !cell.getValue<boolean>() ? <Check className="h-4 w-4" /> : null,
    },
    {
      id: "actions",
      header: t("sessionUserList.columnActions"),
      cell: ({ row }) => (
        <div className="space-x-2">
          <Button
            variant="outline"
            size="sm"
            disabled={!canUseAccount}
            onClick={() => setChatUserId(row.original.id)}
          >
            {t("sessionUserList.chat")}
          </Button>
          {!row.original.isHost && (
            <>
              <Button
                variant="outline"
                size="sm"
                onClick={() => handleRespawnUser(row.original.id)}
                disabled={
                  (isPendingKick || isPendingBan || isPendingRespawn) &&
                  actionUserId === row.original.id
                }
              >
                Respawn
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => handleKickUser(row.original.id)}
                disabled={
                  (isPendingKick || isPendingBan || isPendingRespawn) &&
                  actionUserId === row.original.id
                }
              >
                Kick
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={() => handleBanUser(row.original.id)}
                disabled={
                  (isPendingKick || isPendingBan || isPendingRespawn) &&
                  actionUserId === row.original.id
                }
              >
                Ban
              </Button>
            </>
          )}
        </div>
      ),
    },
  ];

  return (
    <>
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">
            {t("sessionUserList.userListTitle")}
          </h2>
          <div className="flex items-center space-x-2">
            <RefetchButton refetch={refetch} />
            <Button onClick={() => setIsOpenInviteDialog(true)}>
              {t("sessionUserList.inviteUser")}
            </Button>
          </div>
        </div>
        <DataTable
          columns={columns}
          data={pagedUsers}
          isLoading={isPending}
          pagination={{
            pageIndex: effectivePageIndex,
            pageSize,
            totalCount: users.length,
            pageSizeOptions: PAGE_SIZE_OPTIONS,
            onPageIndexChange: setPageIndex,
            onPageSizeChange: setPageSize,
          }}
        />
      </div>
      <UserInviteDialog
        isOpen={isOpenInviteDialog}
        onClose={() => {
          setIsOpenInviteDialog(false);
          refetch();
        }}
        hostId={hostId}
        sessionId={sessionId}
      />
      {chatUserId && chatContact && hostData?.host && (
        <DirectChatDialog
          open={!!chatUserId}
          onClose={() => setChatUserId(null)}
          accountId={hostData.host.accountId}
          accountName={hostData.host.accountName}
          contact={chatContact}
        />
      )}
    </>
  );
}
