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
import { useInvalidateMyPermissions } from "../hooks/useInvalidateMyPermissions";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";

const makeRoleFormSchema = (t: TFunction) =>
  z.object({
    name: z.string().min(1, t("roleList.nameRequired")),
    permissionKeys: z.array(z.string()),
  });
type RoleFormData = z.infer<ReturnType<typeof makeRoleFormSchema>>;

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
  const { t } = useTranslation();
  const isEdit = !!initial;
  const invalidateMyPermissions = useInvalidateMyPermissions();
  const { data: permsData } = useQuery(listPermissions, { scope });
  const { mutateAsync: mutateCreate, isPending: isCreating } =
    useMutation(createRole);
  const { mutateAsync: mutateUpdate, isPending: isUpdating } =
    useMutation(updateRole);
  const schema = useMemo(() => makeRoleFormSchema(t), [t]);

  const {
    control,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<RoleFormData>({
    resolver: zodResolver(schema),
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
        toast.success(t("roleList.roleUpdated"));
      } else {
        await mutateCreate({
          groupId,
          name: data.name,
          scope,
          permissionKeys: data.permissionKeys,
        });
        toast.success(t("roleList.roleCreated"));
      }
      // ロールの権限変更は自分自身の実効権限に影響しうるため getMyPermissions を invalidate.
      // ダイアログのクローズを再取得の完了で待たせないため await しない.
      invalidateMyPermissions();
      reset();
      onClose?.();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("roleList.saveFailed"));
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
            {isEdit ? t("roleList.editRole") : t("roleList.createRole")} (
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
                label={t("roleList.roleName")}
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
                <p className="text-sm font-medium">
                  {t("roleList.grantPermissions")}
                </p>
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
            {isEdit ? t("common.save") : t("common.create")}
          </Button>
          <DialogClose asChild>
            <Button variant="outline">{t("common.cancel")}</Button>
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
  const { t } = useTranslation();
  const { data, isPending, refetch } = useQuery(
    listRoles,
    groupId ? { groupId } : {},
  );
  // グループ詳細では「そのグループ専用カスタムロール」だけを表示する.
  // ListRoles(group_id) は割り当て可能な scope-match なグローバルロールも返すので、
  // group_id 指定時は groupId 一致のものに絞る.
  const filteredRoles = useMemo(() => {
    const all = data?.roles ?? [];
    if (!groupId) return all;
    return all.filter((r) => r.groupId === groupId);
  }, [data?.roles, groupId]);
  const { mutateAsync: mutateDelete } = useMutation(deleteRole);
  const [editingRole, setEditingRole] = useState<Role | undefined>(undefined);
  const [isCreating, setIsCreating] = useState(false);

  // ロール削除は自分自身の実効権限に影響しうるため getMyPermissions を invalidate.
  const invalidateMyPermissions = useInvalidateMyPermissions();

  const columns: ColumnDef<Role>[] = useMemo(
    () => [
      {
        accessorKey: "name",
        header: t("common.name"),
      },
      {
        accessorKey: "scope",
        header: t("roleList.scope"),
        cell: ({ cell }) => roleScopeToLabel(cell.getValue<RoleScope>()),
      },
      {
        accessorKey: "isBuiltin",
        header: t("roleList.type"),
        cell: ({ cell }) =>
          cell.getValue<boolean>()
            ? t("roleList.builtin")
            : t("roleList.custom"),
      },
      {
        accessorKey: "permissionKeys",
        header: t("roleList.permission"),
        cell: ({ cell }) => (
          <span className="text-xs text-muted-foreground">
            {t("roleList.countSuffix", {
              count: cell.getValue<string[]>().length,
            })}
          </span>
        ),
      },
      {
        id: "actions",
        header: t("common.actions"),
        cell: ({ row }) => {
          const role = row.original;
          const editable = canManage && !role.isBuiltin;
          return (
            <div className="flex gap-1">
              <PermissionGuardedButton
                allowed={editable}
                disabledReason={
                  role.isBuiltin
                    ? t("roleList.builtinNotEditable")
                    : t("roleList.noEditPermission")
                }
                size="sm"
                variant="ghost"
                onClick={() => setEditingRole(role)}
              >
                {t("common.edit")}
              </PermissionGuardedButton>
              <PermissionGuardedButton
                allowed={editable}
                disabledReason={
                  role.isBuiltin
                    ? t("roleList.builtinNotDeletable")
                    : t("roleList.noDeletePermission")
                }
                size="sm"
                variant="ghost"
                onClick={async () => {
                  if (
                    !confirm(t("roleList.confirmDelete", { name: role.name }))
                  )
                    return;
                  try {
                    await mutateDelete({ roleId: role.id });
                    toast.success(t("roleList.roleDeleted"));
                    refetch();
                    invalidateMyPermissions();
                  } catch (e) {
                    toast.error(
                      e instanceof Error
                        ? e.message
                        : t("roleList.deleteFailed"),
                    );
                  }
                }}
              >
                {t("common.delete")}
              </PermissionGuardedButton>
            </div>
          );
        },
      },
    ],
    [canManage, mutateDelete, refetch, invalidateMyPermissions, t],
  );

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <PermissionGuardedButton
          allowed={canManage}
          onClick={() => setIsCreating(true)}
        >
          {t("roleList.createRoleButton")}
        </PermissionGuardedButton>
      </div>
      <DataTable columns={columns} data={filteredRoles} isLoading={isPending} />
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
