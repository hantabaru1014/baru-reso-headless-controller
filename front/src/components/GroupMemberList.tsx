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
import { GroupMember } from "../../pbgen/hdlctrl/v1/permission_pb";
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
  SelectField,
  TextField,
  UserCell,
} from "./base";
import { PermissionGuardedButton } from "./base/PermissionGuardedButton";
import { usePermissions } from "../hooks/usePermissions";
import { PERMISSION_KEYS } from "../libs/permissionUtils";

function AddMemberDialog({
  groupId,
  open,
  onClose,
}: {
  groupId: string;
  open: boolean;
  onClose?: () => void;
}) {
  const [userId, setUserId] = useState("");
  const [roleId, setRoleId] = useState("");
  const { data: rolesData } = useQuery(listRoles, { groupId });
  const { mutateAsync, isPending } = useMutation(addGroupMember);

  const roleOptions = useMemo(
    () =>
      (rolesData?.roles ?? []).map((r) => ({
        id: r.id,
        label: `${r.name}${r.isBuiltin ? " (組込)" : ""}`,
      })),
    [rolesData?.roles],
  );

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) {
          setUserId("");
          setRoleId("");
          onClose?.();
        }
      }}
    >
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>メンバーを追加</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <TextField
            label="ユーザー ID"
            value={userId}
            onChange={(e) => setUserId(e.target.value)}
          />
          <SelectField
            label="ロール"
            options={roleOptions}
            selectedId={roleId}
            onChange={(o) => setRoleId(o.id)}
          />
        </div>
        <DialogFooter>
          <Button
            disabled={!userId || !roleId || isPending}
            onClick={async () => {
              try {
                await mutateAsync({ groupId, userId, roleId });
                toast.success("メンバーを追加しました");
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
        open={isAddOpen}
        onClose={() => {
          setIsAddOpen(false);
          refetch();
        }}
      />
    </div>
  );
}
