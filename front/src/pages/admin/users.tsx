import { useMemo, useState } from "react";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useAtomValue } from "jotai";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ColumnDef } from "@tanstack/react-table";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { TFunction } from "i18next";
import { Copy, Loader2 } from "lucide-react";
import {
  createRegistrationToken,
  deleteUser,
  listUsers,
} from "../../../pbgen/hdlctrl/v1/user-UserService_connectquery";
import {
  CreateRegistrationTokenResponse,
  User,
} from "../../../pbgen/hdlctrl/v1/user_pb";
import { listRoles } from "../../../pbgen/hdlctrl/v1/permission-RoleService_connectquery";
import { RoleScope } from "../../../pbgen/hdlctrl/v1/permission_pb";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  Button,
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../../components/ui";
import {
  DataTable,
  RefetchButton,
  SelectField,
  TextField,
} from "../../components/base";
import { PermissionGuardedButton } from "../../components/base/PermissionGuardedButton";
import { ResoniteUserIcon } from "../../components/ResoniteUserIcon";
import { usePermissions } from "../../hooks/usePermissions";
import { PERMISSION_KEYS } from "../../libs/permissionUtils";
import { formatTimestamp } from "../../libs/datetimeUtils";
import { sessionAtom } from "../../atoms/sessionAtom";

const makeInviteFormSchema = (t: TFunction) =>
  z.object({
    resoniteId: z
      .string()
      .min(1, t("adminUsersPage.resoniteIdRequired"))
      .regex(/^U-/, t("adminUsersPage.resoniteIdFormat")),
    personalRoleId: z.string().min(1, t("adminUsersPage.roleRequired")),
  });
type InviteFormData = z.infer<ReturnType<typeof makeInviteFormSchema>>;

function InviteUserDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const { mutateAsync, isPending } = useMutation(createRegistrationToken);
  const [issued, setIssued] = useState<
    (CreateRegistrationTokenResponse & { personalRoleId: string }) | undefined
  >(undefined);

  const inviteFormSchema = useMemo(() => makeInviteFormSchema(t), [t]);

  // 個人グループに付与するロール候補. group_id 未指定で ListRoles するとグローバル
  // (seed + グローバルカスタム) が返るので、NORMAL scope のものだけ採用する.
  const { data: rolesData } = useQuery(listRoles, {}, { enabled: open });
  const personalRoleOptions = useMemo(
    () =>
      (rolesData?.roles ?? [])
        .filter((r) => r.scope === RoleScope.NORMAL)
        .map((r) => ({
          id: r.id,
          label: `${r.name}${r.isBuiltin ? t("adminUsersPage.builtinSuffix") : ""}`,
        })),
    [rolesData?.roles, t],
  );

  const {
    control,
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<InviteFormData>({
    resolver: zodResolver(inviteFormSchema),
    defaultValues: { resoniteId: "", personalRoleId: "seed-admin" },
  });

  const handleClose = () => {
    reset();
    setIssued(undefined);
    onClose();
  };

  const onSubmit = async (data: InviteFormData) => {
    try {
      // personal_role_id はトークンと一緒に DB に保存される (改竄不能).
      const res = await mutateAsync({
        resoniteId: data.resoniteId,
        personalRoleId: data.personalRoleId || undefined,
      });
      setIssued({ ...res, personalRoleId: data.personalRoleId });
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t("adminUsersPage.tokenIssueError"),
      );
    }
  };

  // 招待 URL: personal_role_id は token と紐付けて永続化済なので URL には載せない.
  const inviteUrl = issued
    ? `${window.location.origin}/register/${issued.token}`
    : "";

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(inviteUrl);
      toast.success(t("adminUsersPage.urlCopied"));
    } catch {
      toast.error(t("adminUsersPage.copyError"));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) handleClose();
      }}
    >
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{t("adminUsersPage.inviteUser")}</DialogTitle>
        </DialogHeader>
        {issued ? (
          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <ResoniteUserIcon
                iconUrl={issued.iconUrl}
                alt={issued.resoniteUserName}
                className="size-12"
              />
              <div>
                <div className="font-medium">{issued.resoniteUserName}</div>
                <div className="text-muted-foreground text-xs">
                  {t("adminUsersPage.shareLinkHint")}
                </div>
              </div>
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium">
                {t("adminUsersPage.inviteUrl")}
              </label>
              <div className="flex gap-2">
                <input
                  readOnly
                  value={inviteUrl}
                  className="flex-1 rounded-md border bg-muted px-3 py-2 text-xs font-mono"
                  onFocus={(e) => e.target.select()}
                />
                <Button
                  variant="outline"
                  size="icon"
                  onClick={handleCopy}
                  title={t("adminUsersPage.copyUrl")}
                >
                  <Copy />
                </Button>
              </div>
            </div>
            <div className="text-muted-foreground text-xs">
              {t("adminUsersPage.expiresAt")}{" "}
              {formatTimestamp(issued.expiresAt)}
            </div>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="outline">{t("common.close")}</Button>
              </DialogClose>
            </DialogFooter>
          </div>
        ) : (
          <form
            id="invite-form"
            onSubmit={handleSubmit(onSubmit)}
            className="space-y-4"
          >
            <TextField
              label="Resonite ID"
              placeholder="U-username"
              {...register("resoniteId")}
              error={errors.resoniteId?.message}
            />
            <Controller
              name="personalRoleId"
              control={control}
              render={({ field }) => (
                <SelectField
                  label={t("adminUsersPage.personalRoleLabel")}
                  helperText={t("adminUsersPage.personalRoleHelperText")}
                  options={personalRoleOptions}
                  selectedId={field.value}
                  onChange={(o) => field.onChange(o.id)}
                  error={errors.personalRoleId?.message}
                />
              )}
            />
            <p className="text-muted-foreground text-xs">
              {t("adminUsersPage.inviteHint")}
            </p>
            <DialogFooter>
              <Button type="submit" form="invite-form" disabled={isPending}>
                {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {t("adminUsersPage.issueInviteLink")}
              </Button>
              <DialogClose asChild>
                <Button variant="outline" type="button">
                  {t("common.cancel")}
                </Button>
              </DialogClose>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}

function DeleteUserDialog({
  user,
  open,
  onClose,
  onDeleted,
}: {
  user: User | undefined;
  open: boolean;
  onClose: () => void;
  onDeleted: () => void;
}) {
  const { t } = useTranslation();
  const { mutateAsync, isPending } = useMutation(deleteUser);

  const handleConfirm = async () => {
    if (!user) return;
    try {
      await mutateAsync({ userId: user.id });
      toast.success(t("adminUsersPage.deleteUserSuccess"));
      onDeleted();
      onClose();
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t("adminUsersPage.deleteError"),
      );
    }
  };

  return (
    <AlertDialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {t("adminUsersPage.deleteUserTitle")}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {t("adminUsersPage.deleteUserPrefix")}{" "}
            <span className="font-mono">{user?.id}</span> (Resonite ID:{" "}
            <span className="font-mono">{user?.resoniteId}</span>)
            {t("adminUsersPage.deleteUserSuffix")}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={isPending}>
            {t("common.cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            disabled={isPending}
            onClick={(e) => {
              e.preventDefault();
              handleConfirm();
            }}
            className="bg-destructive text-white hover:bg-destructive/90"
          >
            {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {t("common.delete")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export default function AdminUsersPage() {
  const { t } = useTranslation();
  const { hasSystemPermission, isPending: isPermPending } = usePermissions();
  const session = useAtomValue(sessionAtom);
  const canCreate = hasSystemPermission(PERMISSION_KEYS.SYSTEM_USER_CREATE);
  const canDelete = hasSystemPermission(PERMISSION_KEYS.SYSTEM_USER_DELETE);
  // ListUsers は認証のみで許可される. list/create/delete のいずれかを持てばページを開ける
  // (user.create のみのカスタムロールでも招待できるように).
  const canAccess =
    hasSystemPermission(PERMISSION_KEYS.SYSTEM_USER_LIST) ||
    canCreate ||
    canDelete;

  const { data, isPending, refetch } = useQuery(
    listUsers,
    {},
    { enabled: canAccess },
  );
  const [inviteOpen, setInviteOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<User | undefined>(undefined);

  const currentUserId = session?.user.id;

  const columns: ColumnDef<User>[] = [
    {
      id: "icon",
      header: t("adminUsersPage.iconColumn"),
      cell: ({ row }) => (
        <ResoniteUserIcon
          iconUrl={row.original.iconUrl}
          alt={row.original.id}
          className="size-8"
        />
      ),
      size: 60,
    },
    {
      accessorKey: "id",
      header: t("adminUsersPage.userIdColumn"),
      cell: ({ cell }) => (
        <span className="font-mono text-xs">{cell.getValue<string>()}</span>
      ),
    },
    {
      accessorKey: "resoniteId",
      header: "Resonite ID",
      cell: ({ cell }) => (
        <span className="font-mono text-xs">{cell.getValue<string>()}</span>
      ),
    },
    {
      accessorKey: "createdAt",
      header: t("adminUsersPage.createdAtColumn"),
      cell: ({ row }) => (
        <span className="text-xs">
          {formatTimestamp(row.original.createdAt)}
        </span>
      ),
    },
    {
      id: "actions",
      header: t("common.actions"),
      cell: ({ row }) => {
        const isSelf = row.original.id === currentUserId;
        const isSystem = row.original.id === "system";
        const disabledReason = isSelf
          ? t("adminUsersPage.cannotDeleteSelf")
          : isSystem
            ? t("adminUsersPage.cannotDeleteSystem")
            : !canDelete
              ? t("adminUsersPage.noDeletePermission")
              : undefined;
        return (
          <PermissionGuardedButton
            allowed={canDelete && !isSelf && !isSystem}
            disabledReason={disabledReason}
            variant="ghost"
            size="sm"
            onClick={() => setDeleteTarget(row.original)}
          >
            {t("common.delete")}
          </PermissionGuardedButton>
        );
      },
    },
  ];

  if (isPermPending) return null;

  if (!canAccess) {
    return (
      <div className="container mx-auto p-4">
        <p className="text-destructive text-sm">
          {t("adminUsersPage.noPermission")}
        </p>
      </div>
    );
  }

  return (
    <div className="container mx-auto p-4 space-y-4">
      <p className="text-muted-foreground text-sm">
        {t("adminUsersPage.description")}
      </p>
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <PermissionGuardedButton
          allowed={canCreate}
          disabledReason={t("adminUsersPage.noCreatePermission")}
          onClick={() => setInviteOpen(true)}
        >
          {t("adminUsersPage.inviteUser")}
        </PermissionGuardedButton>
      </div>
      <DataTable
        columns={columns}
        data={data?.users ?? []}
        isLoading={isPending}
      />
      <InviteUserDialog
        open={inviteOpen}
        onClose={() => {
          setInviteOpen(false);
          refetch();
        }}
      />
      <DeleteUserDialog
        user={deleteTarget}
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(undefined)}
        onDeleted={() => refetch()}
      />
    </div>
  );
}
