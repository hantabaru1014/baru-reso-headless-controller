import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { ColumnDef } from "@tanstack/react-table";
import {
  addGroupMember,
  listGroupMembers,
  removeGroupMember,
  updateGroupMemberRole,
} from "../../pbgen/hdlctrl/v1/permission-GroupService_connectquery";
import { listRoles } from "../../pbgen/hdlctrl/v1/permission-RoleService_connectquery";
import { listUsers } from "../../pbgen/hdlctrl/v1/user-UserService_connectquery";
import { GroupMember } from "../../pbgen/hdlctrl/v1/permission_pb";
import { User } from "../../pbgen/hdlctrl/v1/user_pb";
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
  RefetchButton,
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

function AddMemberDialog({
  groupId,
  excludeUserIds,
  open,
  onClose,
}: {
  groupId: string;
  /** 既存メンバー (検索結果から除外する) */
  excludeUserIds: Set<string>;
  open: boolean;
  onClose?: () => void;
}) {
  const [query, setQuery] = useState("");
  const [selectedUser, setSelectedUser] = useState<User | undefined>(undefined);
  const [roleId, setRoleId] = useState("");
  const { data: rolesData } = useQuery(listRoles, { groupId });
  const { data: usersData, isPending: isUsersPending } = useQuery(
    listUsers,
    {},
    { enabled: open },
  );
  const { mutateAsync, isPending } = useMutation(addGroupMember);

  const roleOptions = useMemo(
    () =>
      (rolesData?.roles ?? []).map((r) => ({
        id: r.id,
        label: `${r.name}${r.isBuiltin ? " (組込)" : ""}`,
      })),
    [rolesData?.roles],
  );

  const reset = () => {
    setQuery("");
    setSelectedUser(undefined);
    setRoleId("");
  };

  const filteredUsers = useMemo(() => {
    const q = query.trim().toLowerCase();
    const all = usersData?.users ?? [];
    const remaining = all.filter((u) => !excludeUserIds.has(u.id));
    if (!q) return remaining.slice(0, 50);
    return remaining
      .filter(
        (u) =>
          u.id.toLowerCase().includes(q) ||
          u.resoniteId.toLowerCase().includes(q),
      )
      .slice(0, 50);
  }, [usersData?.users, excludeUserIds, query]);

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
          <DialogTitle>メンバーを追加</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          {selectedUser ? (
            <div className="flex items-center justify-between gap-3 rounded-md border p-3">
              <div className="flex items-center gap-3 min-w-0">
                <ResoniteUserIcon
                  iconUrl={selectedUser.iconUrl}
                  alt={selectedUser.id}
                  className="size-10"
                />
                <div className="flex flex-col min-w-0">
                  <span className="font-mono text-sm truncate">
                    {selectedUser.id}
                  </span>
                  <span className="font-mono text-xs text-muted-foreground truncate">
                    {selectedUser.resoniteId}
                  </span>
                </div>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setSelectedUser(undefined)}
              >
                変更
              </Button>
            </div>
          ) : (
            <>
              <TextField
                label="ユーザー検索"
                placeholder="ユーザー ID または Resonite ID で検索"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
              <ScrollBase height="240px">
                <div className="space-y-1">
                  {isUsersPending && (
                    <p className="p-2 text-sm text-muted-foreground">
                      読み込み中...
                    </p>
                  )}
                  {!isUsersPending && filteredUsers.length === 0 && (
                    <p className="p-2 text-sm text-muted-foreground">
                      該当するユーザーが見つかりません
                    </p>
                  )}
                  {filteredUsers.map((u) => (
                    <button
                      key={u.id}
                      type="button"
                      onClick={() => setSelectedUser(u)}
                      className="flex w-full items-center gap-3 rounded-md p-2 hover:bg-accent text-left"
                    >
                      <ResoniteUserIcon
                        iconUrl={u.iconUrl}
                        alt={u.id}
                        className="size-8"
                      />
                      <div className="flex flex-col min-w-0">
                        <span className="font-mono text-sm truncate">
                          {u.id}
                        </span>
                        <span className="font-mono text-xs text-muted-foreground truncate">
                          {u.resoniteId}
                        </span>
                      </div>
                    </button>
                  ))}
                </div>
              </ScrollBase>
            </>
          )}
          <SelectField
            label="ロール"
            options={roleOptions}
            selectedId={roleId}
            onChange={(o) => setRoleId(o.id)}
          />
        </div>
        <DialogFooter>
          <Button
            disabled={!selectedUser || !roleId || isPending}
            onClick={async () => {
              if (!selectedUser) return;
              try {
                await mutateAsync({
                  groupId,
                  userId: selectedUser.id,
                  roleId,
                });
                toast.success("メンバーを追加しました");
                reset();
                onClose?.();
              } catch (e) {
                toast.error(
                  e instanceof Error ? e.message : "追加に失敗しました",
                );
              }
            }}
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

export default function GroupMemberList({ groupId }: { groupId: string }) {
  const { data, isPending, refetch } = useQuery(listGroupMembers, { groupId });
  const { data: rolesData } = useQuery(listRoles, { groupId });
  const { hasPermission } = usePermissions();
  const { mutateAsync: mutateUpdateRole } = useMutation(updateGroupMemberRole);
  const { mutateAsync: mutateRemove, isPending: isRemoving } =
    useMutation(removeGroupMember);
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

  const columns: ColumnDef<GroupMember>[] = useMemo(
    () => [
      {
        accessorKey: "userId",
        header: "ユーザー",
        cell: ({ cell }) => <UserCell userId={cell.getValue<string>()} />,
      },
      {
        accessorKey: "roleId",
        header: "ロール",
        cell: ({ row }) => {
          const member = row.original;
          const currentRoleName = rolesById.get(member.roleId) ?? member.roleId;
          if (!canManage) return <span>{currentRoleName}</span>;
          return (
            <SelectField
              options={(rolesData?.roles ?? []).map((r) => ({
                id: r.id,
                label: `${r.name}${r.isBuiltin ? " (組込)" : ""}`,
              }))}
              selectedId={member.roleId}
              onChange={async (o) => {
                try {
                  await mutateUpdateRole({
                    groupId,
                    userId: member.userId,
                    roleId: o.id,
                  });
                  toast.success("ロールを更新しました");
                  refetch();
                  invalidateMyPermissions();
                } catch (e) {
                  toast.error(
                    e instanceof Error ? e.message : "ロール変更に失敗しました",
                  );
                }
              }}
            />
          );
        },
      },
      {
        accessorKey: "addedBy",
        header: "招待者",
        cell: ({ cell }) => {
          const v = cell.getValue<string | undefined>();
          return v ? (
            <UserCell userId={v} />
          ) : (
            <span className="text-muted-foreground text-xs">システム</span>
          );
        },
      },
      {
        id: "actions",
        header: "操作",
        cell: ({ row }) => (
          <PermissionGuardedButton
            allowed={canManage}
            variant="ghost"
            size="sm"
            disabled={isRemoving}
            onClick={async () => {
              if (
                !confirm(`${row.original.userId} をグループから削除しますか?`)
              )
                return;
              try {
                await mutateRemove({ groupId, userId: row.original.userId });
                toast.success("メンバーを削除しました");
                refetch();
                invalidateMyPermissions();
              } catch (e) {
                toast.error(
                  e instanceof Error ? e.message : "削除に失敗しました",
                );
              }
            }}
          >
            削除
          </PermissionGuardedButton>
        ),
      },
    ],
    [
      canManage,
      rolesById,
      rolesData?.roles,
      mutateUpdateRole,
      mutateRemove,
      isRemoving,
      groupId,
      refetch,
      invalidateMyPermissions,
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
          メンバー追加
        </PermissionGuardedButton>
      </div>
      <DataTable
        columns={columns}
        data={data?.members ?? []}
        isLoading={isPending}
      />
      <AddMemberDialog
        groupId={groupId}
        excludeUserIds={new Set((data?.members ?? []).map((m) => m.userId))}
        open={isAddOpen}
        onClose={() => {
          setIsAddOpen(false);
          refetch();
        }}
      />
    </div>
  );
}
