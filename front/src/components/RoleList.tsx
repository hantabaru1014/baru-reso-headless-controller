import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { ColumnDef } from "@tanstack/react-table";
import { create } from "@bufbuild/protobuf";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  createRole,
  deleteRole,
  listPermissions,
  listRoles,
  updateRole,
} from "../../pbgen/hdlctrl/v1/permission-RoleService_connectquery";
import {
  PermissionKeyListSchema,
  Role,
  RoleScope,
} from "../../pbgen/hdlctrl/v1/permission_pb";
import {
  Button,
  Checkbox,
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui";
import { DataTable, RefetchButton, TextField } from "./base";
import { PermissionGuardedButton } from "./base/PermissionGuardedButton";
import {
  permissionKeyToLabel,
  roleScopeToLabel,
} from "../libs/permissionUtils";

const roleFormSchema = z.object({
  name: z.string().min(1, "ロール名は必須です"),
  permissionKeys: z.array(z.string()),
});
type RoleFormData = z.infer<typeof roleFormSchema>;

function RoleEditorDialog({
  groupId,
  scope,
  initial,
  open,
  onClose,
}: {
  groupId?: string;
  scope: RoleScope;
  /** undefined のとき新規作成. 渡した場合はそのロールを編集する. */
  initial?: Role;
  open: boolean;
  onClose?: () => void;
}) {
  const isEdit = !!initial;
  const { data: permsData } = useQuery(listPermissions, { scope });
  const { mutateAsync: mutateCreate, isPending: isCreating } =
    useMutation(createRole);
  const { mutateAsync: mutateUpdate, isPending: isUpdating } =
    useMutation(updateRole);

  const {
    control,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<RoleFormData>({
    resolver: zodResolver(roleFormSchema),
    defaultValues: {
      name: initial?.name ?? "",
      permissionKeys: initial?.permissionKeys ?? [],
    },
  });

  const onSubmit = async (data: RoleFormData) => {
    try {
      if (isEdit && initial) {
        await mutateUpdate({
          roleId: initial.id,
          name: data.name,
          permissionKeys: create(PermissionKeyListSchema, {
            keys: data.permissionKeys,
          }),
        });
        toast.success("ロールを更新しました");
      } else {
        await mutateCreate({
          groupId,
          name: data.name,
          scope,
          permissionKeys: data.permissionKeys,
        });
        toast.success("ロールを作成しました");
      }
      reset();
      onClose?.();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "保存に失敗しました");
    }
  };

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
      <DialogContent className="sm:max-w-[500px] max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {isEdit ? "ロールを編集" : "ロールを作成"} (
            {roleScopeToLabel(scope)})
          </DialogTitle>
        </DialogHeader>
        <form
          id="role-form"
          onSubmit={handleSubmit(onSubmit)}
          className="space-y-4"
        >
          <Controller
            name="name"
            control={control}
            render={({ field }) => (
              <TextField
                label="ロール名"
                {...field}
                error={errors.name?.message}
              />
            )}
          />
          <Controller
            name="permissionKeys"
            control={control}
            render={({ field }) => (
              <div className="space-y-2">
                <p className="text-sm font-medium">付与するパーミッション</p>
                {(permsData?.permissions ?? []).map((p) => {
                  const checked = field.value.includes(p.key);
                  return (
                    <label
                      key={p.key}
                      className="flex items-center gap-2 cursor-pointer"
                    >
                      <Checkbox
                        checked={checked}
                        onCheckedChange={(c) => {
                          const next = c
                            ? [...field.value, p.key]
                            : field.value.filter((k) => k !== p.key);
                          field.onChange(next);
                        }}
                      />
                      <span className="text-sm">
                        {permissionKeyToLabel(p.key)}{" "}
                        <span className="text-muted-foreground font-mono text-xs">
                          ({p.key})
                        </span>
                      </span>
                    </label>
                  );
                })}
              </div>
            )}
          />
        </form>
        <DialogFooter>
          <Button
            type="submit"
            form="role-form"
            disabled={isCreating || isUpdating}
          >
            {isEdit ? "保存" : "作成"}
          </Button>
          <DialogClose asChild>
            <Button variant="outline">キャンセル</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * ロール一覧 + 編集/削除/作成 UI.
 *
 * groupId 指定時: そのグループ内カスタムロール + 割り当て可能なグローバルロールを表示.
 * groupId 未指定時: グローバルロール (seed + グローバルカスタム) を表示.
 */
export default function RoleList({
  groupId,
  canManage,
  scope = RoleScope.NORMAL,
}: {
  /** 未指定の場合はグローバル */
  groupId?: string;
  /** 作成/編集/削除を許可するか */
  canManage: boolean;
  scope?: RoleScope;
}) {
  const { data, isPending, refetch } = useQuery(
    listRoles,
    groupId ? { groupId } : {},
  );
  const { mutateAsync: mutateDelete } = useMutation(deleteRole);
  const [editingRole, setEditingRole] = useState<Role | undefined>(undefined);
  const [isCreating, setIsCreating] = useState(false);

  const columns: ColumnDef<Role>[] = useMemo(
    () => [
      {
        accessorKey: "name",
        header: "名前",
      },
      {
        accessorKey: "scope",
        header: "スコープ",
        cell: ({ cell }) => roleScopeToLabel(cell.getValue<RoleScope>()),
      },
      {
        accessorKey: "isBuiltin",
        header: "種別",
        cell: ({ cell }) =>
          cell.getValue<boolean>() ? "組込 (seed)" : "カスタム",
      },
      {
        accessorKey: "permissionKeys",
        header: "パーミッション",
        cell: ({ cell }) => (
          <span className="text-xs text-muted-foreground">
            {cell.getValue<string[]>().length} 件
          </span>
        ),
      },
      {
        id: "actions",
        header: "操作",
        cell: ({ row }) => {
          const role = row.original;
          const editable = canManage && !role.isBuiltin;
          return (
            <div className="flex gap-1">
              <PermissionGuardedButton
                allowed={editable}
                disabledReason={
                  role.isBuiltin
                    ? "組込ロールは編集できません"
                    : "編集権限がありません"
                }
                size="sm"
                variant="ghost"
                onClick={() => setEditingRole(role)}
              >
                編集
              </PermissionGuardedButton>
              <PermissionGuardedButton
                allowed={editable}
                disabledReason={
                  role.isBuiltin
                    ? "組込ロールは削除できません"
                    : "削除権限がありません"
                }
                size="sm"
                variant="ghost"
                onClick={async () => {
                  if (!confirm(`ロール "${role.name}" を削除しますか?`)) return;
                  try {
                    await mutateDelete({ roleId: role.id });
                    toast.success("ロールを削除しました");
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
            </div>
          );
        },
      },
    ],
    [canManage, mutateDelete, refetch],
  );

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <PermissionGuardedButton
          allowed={canManage}
          onClick={() => setIsCreating(true)}
        >
          ロール作成
        </PermissionGuardedButton>
      </div>
      <DataTable
        columns={columns}
        data={data?.roles ?? []}
        isLoading={isPending}
      />
      {isCreating && (
        <RoleEditorDialog
          groupId={groupId}
          scope={scope}
          open
          onClose={() => {
            setIsCreating(false);
            refetch();
          }}
        />
      )}
      {editingRole && (
        <RoleEditorDialog
          groupId={groupId}
          scope={editingRole.scope}
          initial={editingRole}
          open
          onClose={() => {
            setEditingRole(undefined);
            refetch();
          }}
        />
      )}
    </div>
  );
}
