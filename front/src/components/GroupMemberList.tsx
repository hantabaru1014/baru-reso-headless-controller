import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { ColumnDef } from "@tanstack/react-table";
import {
  addGroupMember,
  addInvitedGroupMember,
  listGroupMembers,
  removeGroupMember,
  removeInvitedGroupMember,
  updateGroupMemberRole,
  updateInvitedGroupMemberRole,
} from "../../pbgen/hdlctrl/v1/permission-GroupService_connectquery";
import { listRoles } from "../../pbgen/hdlctrl/v1/permission-RoleService_connectquery";
import {
  listInvitations,
  listUsers,
} from "../../pbgen/hdlctrl/v1/user-UserService_connectquery";
import {
  GroupMember,
  InvitedGroupMember,
} from "../../pbgen/hdlctrl/v1/permission_pb";
import { Invitation, User } from "../../pbgen/hdlctrl/v1/user_pb";
import {
  Button,
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui";
import {
  DataTable,
  InvitedBadge,
  RefetchButton,
  ResoniteUserCell,
  ScrollBase,
  SelectField,
  TextField,
  UserCell,
} from "./base";
import { ResoniteUserIcon } from "./ResoniteUserIcon";
import { PermissionGuardedButton } from "./base/PermissionGuardedButton";
import { usePermissions } from "../hooks/usePermissions";
import { PERMISSION_KEYS } from "../libs/permissionUtils";
import { useInvalidateMyPermissions } from "../hooks/useInvalidateMyPermissions";
import { useTranslation } from "react-i18next";

/** 登録済みユーザー or 招待中 (未登録) ユーザー */
type Candidate =
  { kind: "user"; user: User } | { kind: "invitation"; invitation: Invitation };

type MemberRow =
  | { kind: "member"; member: GroupMember }
  | { kind: "invited"; member: InvitedGroupMember };

function CandidateCell({ candidate }: { candidate: Candidate }) {
  if (candidate.kind === "invitation") {
    return (
      <ResoniteUserCell
        resoniteId={candidate.invitation.resoniteId}
        size="md"
        badge={<InvitedBadge expiresAt={candidate.invitation.expiresAt} />}
      />
    );
  }
  const u = candidate.user;
  return (
    <div className="flex items-center gap-3 min-w-0">
      <ResoniteUserIcon iconUrl={u.iconUrl} alt={u.id} className="size-8" />
      <div className="flex flex-col min-w-0">
        <span className="font-mono text-sm truncate">{u.id}</span>
        <span className="font-mono text-xs text-muted-foreground truncate">
          {u.resoniteId}
        </span>
      </div>
    </div>
  );
}

function AddMemberDialog({
  groupId,
  excludeUserIds,
  excludeInvitationIds,
  open,
  onClose,
}: {
  groupId: string;
  /** 既存メンバー (検索結果から除外する) */
  excludeUserIds: Set<string>;
  /** 既に参加予定の招待 (検索結果から除外する) */
  excludeInvitationIds: Set<string>;
  open: boolean;
  onClose?: () => void;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<Candidate | undefined>(undefined);
  const [roleId, setRoleId] = useState("");
  const { data: rolesData } = useQuery(listRoles, { groupId });
  const { data: usersData, isPending: isUsersPending } = useQuery(
    listUsers,
    {},
    { enabled: open },
  );
  const { data: invitationsData, isPending: isInvitationsPending } = useQuery(
    listInvitations,
    {},
    { enabled: open },
  );
  const { mutateAsync: mutateAddUser, isPending: isAddingUser } =
    useMutation(addGroupMember);
  const { mutateAsync: mutateAddInvited, isPending: isAddingInvited } =
    useMutation(addInvitedGroupMember);
  const isPending = isAddingUser || isAddingInvited;
  const isListPending = isUsersPending || isInvitationsPending;

  const roleOptions = useMemo(
    () =>
      (rolesData?.roles ?? []).map((r) => ({
        id: r.id,
        label: `${r.name}${r.isBuiltin ? ` ${t("groupMemberList.builtinSuffix")}` : ""}`,
      })),
    [rolesData?.roles, t],
  );

  const reset = () => {
    setQuery("");
    setSelected(undefined);
    setRoleId("");
  };

  const filteredCandidates = useMemo(() => {
    const q = query.trim().toLowerCase();
    const candidates: Candidate[] = [
      ...(usersData?.users ?? [])
        .filter((u) => !excludeUserIds.has(u.id))
        .map((user) => ({ kind: "user" as const, user })),
      ...(invitationsData?.invitations ?? [])
        .filter((i) => !excludeInvitationIds.has(i.id))
        .map((invitation) => ({ kind: "invitation" as const, invitation })),
    ];
    const matches = (c: Candidate) =>
      c.kind === "user"
        ? c.user.id.toLowerCase().includes(q) ||
          c.user.resoniteId.toLowerCase().includes(q)
        : c.invitation.resoniteId.toLowerCase().includes(q);
    return (q ? candidates.filter(matches) : candidates).slice(0, 50);
  }, [
    usersData?.users,
    invitationsData?.invitations,
    excludeUserIds,
    excludeInvitationIds,
    query,
  ]);

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) {
          reset();
          onClose?.();
        }
      }}
    >
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{t("groupMemberList.addMemberTitle")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          {selected ? (
            <div className="flex items-center justify-between gap-3 rounded-md border p-3">
              <CandidateCell candidate={selected} />
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setSelected(undefined)}
              >
                {t("groupMemberList.change")}
              </Button>
            </div>
          ) : (
            <>
              <TextField
                label={t("groupMemberList.userSearch")}
                placeholder={t("groupMemberList.userSearchPlaceholder")}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
              <ScrollBase height="240px">
                <div className="space-y-1">
                  {isListPending && (
                    <p className="p-2 text-sm text-muted-foreground">
                      {t("common.loading")}
                    </p>
                  )}
                  {!isListPending && filteredCandidates.length === 0 && (
                    <p className="p-2 text-sm text-muted-foreground">
                      {t("groupMemberList.noUsersFound")}
                    </p>
                  )}
                  {filteredCandidates.map((c) => (
                    <button
                      key={
                        c.kind === "user"
                          ? `user:${c.user.id}`
                          : `invitation:${c.invitation.id}`
                      }
                      type="button"
                      onClick={() => setSelected(c)}
                      className="flex w-full items-center rounded-md p-2 hover:bg-accent text-left"
                    >
                      <CandidateCell candidate={c} />
                    </button>
                  ))}
                </div>
              </ScrollBase>
            </>
          )}
          {selected?.kind === "invitation" && (
            <p className="text-muted-foreground text-xs">
              {t("groupMemberList.invitedMemberHint")}
            </p>
          )}
          <SelectField
            label={t("groupMemberList.role")}
            options={roleOptions}
            selectedId={roleId}
            onChange={(o) => setRoleId(o.id)}
          />
        </div>
        <DialogFooter>
          <Button
            disabled={!selected || !roleId || isPending}
            onClick={async () => {
              if (!selected) return;
              try {
                if (selected.kind === "user") {
                  await mutateAddUser({
                    groupId,
                    userId: selected.user.id,
                    roleId,
                  });
                } else {
                  await mutateAddInvited({
                    groupId,
                    invitationId: selected.invitation.id,
                    roleId,
                  });
                }
                toast.success(t("groupMemberList.memberAdded"));
                reset();
                onClose?.();
              } catch (e) {
                toast.error(
                  e instanceof Error
                    ? e.message
                    : t("groupMemberList.addFailed"),
                );
              }
            }}
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

export default function GroupMemberList({ groupId }: { groupId: string }) {
  const { t } = useTranslation();
  const { data, isPending, refetch } = useQuery(listGroupMembers, { groupId });
  const { data: rolesData } = useQuery(listRoles, { groupId });
  const { hasPermission } = usePermissions();
  const { mutateAsync: mutateUpdateRole } = useMutation(updateGroupMemberRole);
  const { mutateAsync: mutateUpdateInvitedRole } = useMutation(
    updateInvitedGroupMemberRole,
  );
  const { mutateAsync: mutateRemove, isPending: isRemoving } =
    useMutation(removeGroupMember);
  const { mutateAsync: mutateRemoveInvited, isPending: isRemovingInvited } =
    useMutation(removeInvitedGroupMember);
  const [isAddOpen, setIsAddOpen] = useState(false);

  // メンバーのロール変更/削除は自分自身の権限に影響しうるため getMyPermissions を invalidate.
  const invalidateMyPermissions = useInvalidateMyPermissions();

  const canManage = hasPermission(
    groupId,
    PERMISSION_KEYS.GROUP_MEMBERS_MANAGE,
  );

  const rolesById = useMemo(() => {
    const m = new Map<string, string>();
    for (const r of rolesData?.roles ?? []) m.set(r.id, r.name);
    return m;
  }, [rolesData?.roles]);

  const excludeUserIds = useMemo(
    () => new Set((data?.members ?? []).map((m) => m.userId)),
    [data?.members],
  );
  const excludeInvitationIds = useMemo(
    () => new Set((data?.invitedMembers ?? []).map((m) => m.invitationId)),
    [data?.invitedMembers],
  );

  const rows: MemberRow[] = useMemo(
    () => [
      ...(data?.members ?? []).map((member) => ({
        kind: "member" as const,
        member,
      })),
      ...(data?.invitedMembers ?? []).map((member) => ({
        kind: "invited" as const,
        member,
      })),
    ],
    [data?.members, data?.invitedMembers],
  );

  const columns: ColumnDef<MemberRow>[] = useMemo(
    () => [
      {
        id: "user",
        header: t("groupMemberList.user"),
        cell: ({ row }) =>
          row.original.kind === "member" ? (
            <UserCell userId={row.original.member.userId} />
          ) : (
            <ResoniteUserCell
              resoniteId={row.original.member.resoniteId}
              badge={<InvitedBadge expiresAt={row.original.member.expiresAt} />}
            />
          ),
      },
      {
        id: "role",
        header: t("groupMemberList.role"),
        cell: ({ row }) => {
          const r = row.original;
          const currentRoleName =
            rolesById.get(r.member.roleId) ?? r.member.roleId;
          if (!canManage) return <span>{currentRoleName}</span>;
          return (
            <SelectField
              options={(rolesData?.roles ?? []).map((role) => ({
                id: role.id,
                label: `${role.name}${role.isBuiltin ? ` ${t("groupMemberList.builtinSuffix")}` : ""}`,
              }))}
              selectedId={r.member.roleId}
              onChange={async (o) => {
                try {
                  if (r.kind === "member") {
                    await mutateUpdateRole({
                      groupId,
                      userId: r.member.userId,
                      roleId: o.id,
                    });
                  } else {
                    await mutateUpdateInvitedRole({
                      groupId,
                      invitationId: r.member.invitationId,
                      roleId: o.id,
                    });
                  }
                  toast.success(t("groupMemberList.roleUpdated"));
                  refetch();
                  if (r.kind === "member") invalidateMyPermissions();
                } catch (e) {
                  toast.error(
                    e instanceof Error
                      ? e.message
                      : t("groupMemberList.roleUpdateFailed"),
                  );
                }
              }}
            />
          );
        },
      },
      {
        id: "addedBy",
        header: t("groupMemberList.invitedBy"),
        cell: ({ row }) => {
          const v = row.original.member.addedBy;
          return v ? (
            <UserCell userId={v} />
          ) : (
            <span className="text-muted-foreground text-xs">
              {t("groupMemberList.system")}
            </span>
          );
        },
      },
      {
        id: "actions",
        header: t("common.actions"),
        cell: ({ row }) => {
          const r = row.original;
          return (
            <PermissionGuardedButton
              allowed={canManage}
              variant="ghost"
              size="sm"
              disabled={isRemoving || isRemovingInvited}
              onClick={async () => {
                const label =
                  r.kind === "member" ? r.member.userId : r.member.resoniteId;
                if (
                  !confirm(
                    t("groupMemberList.confirmRemove", { userId: label }),
                  )
                )
                  return;
                try {
                  if (r.kind === "member") {
                    await mutateRemove({ groupId, userId: r.member.userId });
                  } else {
                    await mutateRemoveInvited({
                      groupId,
                      invitationId: r.member.invitationId,
                    });
                  }
                  toast.success(t("groupMemberList.memberRemoved"));
                  refetch();
                  if (r.kind === "member") invalidateMyPermissions();
                } catch (e) {
                  toast.error(
                    e instanceof Error
                      ? e.message
                      : t("groupMemberList.removeFailed"),
                  );
                }
              }}
            >
              {t("common.delete")}
            </PermissionGuardedButton>
          );
        },
      },
    ],
    [
      canManage,
      rolesById,
      rolesData?.roles,
      mutateUpdateRole,
      mutateUpdateInvitedRole,
      mutateRemove,
      mutateRemoveInvited,
      isRemoving,
      isRemovingInvited,
      groupId,
      refetch,
      invalidateMyPermissions,
      t,
    ],
  );

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <PermissionGuardedButton
          allowed={canManage}
          onClick={() => setIsAddOpen(true)}
        >
          {t("groupMemberList.addMember")}
        </PermissionGuardedButton>
      </div>
      <DataTable columns={columns} data={rows} isLoading={isPending} />
      <AddMemberDialog
        groupId={groupId}
        excludeUserIds={excludeUserIds}
        excludeInvitationIds={excludeInvitationIds}
        open={isAddOpen}
        onClose={() => {
          setIsAddOpen(false);
          refetch();
        }}
      />
    </div>
  );
}
