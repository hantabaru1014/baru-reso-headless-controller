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
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import {
  createRegistrationToken,
  deleteUser,
  listInvitations,
  listUsers,
  reissueInvitation,
  revokeInvitation,
} from "../../../pbgen/hdlctrl/v1/user-UserService_connectquery";
import {
  CreateRegistrationTokenResponse,
  Invitation,
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
  InvitedBadge,
  RefetchButton,
  ResoniteUserCell,
  ResoniteUserPicker,
  SelectField,
} from "../../components/base";
import { PermissionGuardedButton } from "../../components/base/PermissionGuardedButton";
import { ResoniteUserIcon } from "../../components/ResoniteUserIcon";
import { usePermissions } from "../../hooks/usePermissions";
import { PERMISSION_KEYS } from "../../libs/permissionUtils";
import { formatTimestamp } from "../../libs/datetimeUtils";
import { sessionAtom } from "../../atoms/sessionAtom";

const makeInviteFormSchema = (t: TFunction) =>
  z.object({
    resoniteUser: z
      .object({ id: z.string(), name: z.string(), iconUrl: z.string() })
      .nullable()
      .refine((u) => u !== null, t("adminUsersPage.resoniteUserRequired")),
    personalRoleId: z.string().min(1, t("adminUsersPage.roleRequired")),
  });
type InviteFormSchema = ReturnType<typeof makeInviteFormSchema>;
// resoniteUser は未選択 (null) を入力として許し、バリデーション後は non-null になる.
type InviteFormInput = z.input<InviteFormSchema>;
type InviteFormData = z.output<InviteFormSchema>;

function InviteLinkView({
  token,
  expiresAt,
}: {
  token: string;
  expiresAt?: Timestamp;
}) {
  const { t } = useTranslation();
  // 招待 URL: personal_role_id は token と紐付けて永続化済なので URL には載せない.
  const inviteUrl = `${window.location.origin}/register/${token}`;

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(inviteUrl);
      toast.success(t("adminUsersPage.urlCopied"));
    } catch {
      toast.error(t("adminUsersPage.copyError"));
    }
  };

  return (
    <>
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
        {t("adminUsersPage.expiresAt")} {formatTimestamp(expiresAt)}
      </div>
    </>
  );
}

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
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<InviteFormInput, unknown, InviteFormData>({
    resolver: zodResolver(inviteFormSchema),
    defaultValues: { resoniteUser: null, personalRoleId: "seed-admin" },
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
        resoniteId: data.resoniteUser.id,
        personalRoleId: data.personalRoleId || undefined,
      });
      setIssued({ ...res, personalRoleId: data.personalRoleId });
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t("adminUsersPage.tokenIssueError"),
      );
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
            <InviteLinkView token={issued.token} expiresAt={issued.expiresAt} />
            <p className="text-muted-foreground text-xs">
              {t("adminUsersPage.invitedGroupHint")}
            </p>
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
            <Controller
              name="resoniteUser"
              control={control}
              render={({ field }) => (
                <ResoniteUserPicker
                  label={t("adminUsersPage.resoniteUserLabel")}
                  value={field.value ?? undefined}
                  onChange={(u) => field.onChange(u ?? null)}
                  error={errors.resoniteUser?.message}
                  disabled={isPending}
                />
              )}
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

function ReissueInvitationDialog({
  invitation,
  onClose,
  onReissued,
}: {
  invitation: Invitation | undefined;
  onClose: () => void;
  onReissued: () => void;
}) {
  const { t } = useTranslation();
  const { mutateAsync, isPending } = useMutation(reissueInvitation);
  const [issued, setIssued] = useState<
    { token: string; expiresAt?: Timestamp } | undefined
  >(undefined);

  const handleClose = () => {
    setIssued(undefined);
    onClose();
  };

  const handleReissue = async () => {
    if (!invitation) return;
    try {
      const res = await mutateAsync({ invitationId: invitation.id });
      setIssued({ token: res.token, expiresAt: res.expiresAt });
      onReissued();
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t("adminUsersPage.tokenIssueError"),
      );
    }
  };

  return (
    <Dialog
      open={!!invitation}
      onOpenChange={(o) => {
        if (!o) handleClose();
      }}
    >
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{t("adminUsersPage.reissueTitle")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          {invitation && (
            <ResoniteUserCell
              resoniteId={invitation.resoniteId}
              iconClassName="size-10"
            />
          )}
          {issued ? (
            <InviteLinkView token={issued.token} expiresAt={issued.expiresAt} />
          ) : (
            <p className="text-muted-foreground text-sm">
              {t("adminUsersPage.reissueDescription")}
            </p>
          )}
        </div>
        <DialogFooter>
          {!issued && (
            <Button onClick={handleReissue} disabled={isPending}>
              {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("adminUsersPage.reissue")}
            </Button>
          )}
          <DialogClose asChild>
            <Button variant="outline">{t("common.close")}</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function RevokeInvitationDialog({
  invitation,
  onClose,
  onRevoked,
}: {
  invitation: Invitation | undefined;
  onClose: () => void;
  onRevoked: () => void;
}) {
  const { t } = useTranslation();
  const { mutateAsync, isPending } = useMutation(revokeInvitation);

  const handleConfirm = async () => {
    if (!invitation) return;
    try {
      await mutateAsync({ invitationId: invitation.id });
      toast.success(t("adminUsersPage.revokeSuccess"));
      onRevoked();
      onClose();
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t("adminUsersPage.revokeError"),
      );
    }
  };

  return (
    <AlertDialog
      open={!!invitation}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("adminUsersPage.revokeTitle")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("adminUsersPage.revokeDescription", {
              resoniteId: invitation?.resoniteId,
            })}
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
            {t("adminUsersPage.revoke")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function InvitationList({
  invitations,
  canManage,
  onChanged,
}: {
  invitations: Invitation[];
  canManage: boolean;
  onChanged: () => void;
}) {
  const { t } = useTranslation();
  const [reissueTarget, setReissueTarget] = useState<Invitation | undefined>(
    undefined,
  );
  const [revokeTarget, setRevokeTarget] = useState<Invitation | undefined>(
    undefined,
  );
  const { data: rolesData } = useQuery(listRoles, {});
  const roleNameById = useMemo(
    () => new Map((rolesData?.roles ?? []).map((r) => [r.id, r.name])),
    [rolesData?.roles],
  );

  const columns: ColumnDef<Invitation>[] = [
    {
      id: "resoniteUser",
      header: t("adminUsersPage.resoniteUserLabel"),
      cell: ({ row }) => (
        <ResoniteUserCell
          resoniteId={row.original.resoniteId}
          iconClassName="size-8"
          badge={<InvitedBadge expiresAt={row.original.expiresAt} />}
        />
      ),
    },
    {
      id: "personalRole",
      header: t("adminUsersPage.personalRoleLabel"),
      cell: ({ row }) => {
        const roleId = row.original.personalRoleId ?? "seed-admin";
        return (
          <span className="text-xs">{roleNameById.get(roleId) ?? roleId}</span>
        );
      },
    },
    {
      id: "expiresAt",
      header: t("adminUsersPage.expiresAtColumn"),
      cell: ({ row }) => (
        <span className="text-xs">
          {formatTimestamp(row.original.expiresAt)}
        </span>
      ),
    },
    {
      id: "actions",
      header: t("common.actions"),
      cell: ({ row }) => (
        <div className="flex gap-1">
          <PermissionGuardedButton
            allowed={canManage}
            disabledReason={t("adminUsersPage.noCreatePermission")}
            variant="ghost"
            size="sm"
            onClick={() => setReissueTarget(row.original)}
          >
            {t("adminUsersPage.reissue")}
          </PermissionGuardedButton>
          <PermissionGuardedButton
            allowed={canManage}
            disabledReason={t("adminUsersPage.noCreatePermission")}
            variant="ghost"
            size="sm"
            onClick={() => setRevokeTarget(row.original)}
          >
            {t("adminUsersPage.revoke")}
          </PermissionGuardedButton>
        </div>
      ),
    },
  ];

  return (
    <div className="space-y-2">
      <h2 className="text-lg font-semibold">
        {t("adminUsersPage.invitationsTitle")}
      </h2>
      <p className="text-muted-foreground text-sm">
        {t("adminUsersPage.invitationsDescription")}
      </p>
      <DataTable columns={columns} data={invitations} />
      <ReissueInvitationDialog
        invitation={reissueTarget}
        onClose={() => setReissueTarget(undefined)}
        onReissued={onChanged}
      />
      <RevokeInvitationDialog
        invitation={revokeTarget}
        onClose={() => setRevokeTarget(undefined)}
        onRevoked={onChanged}
      />
    </div>
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
  const { data: invitationsData, refetch: refetchInvitations } = useQuery(
    listInvitations,
    {},
    { enabled: canAccess },
  );
  const invitations = invitationsData?.invitations ?? [];
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
        <RefetchButton
          refetch={() => Promise.all([refetch(), refetchInvitations()])}
        />
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
      {invitations.length > 0 && (
        <InvitationList
          invitations={invitations}
          canManage={canCreate}
          onChanged={() => refetchInvitations()}
        />
      )}
      <InviteUserDialog
        open={inviteOpen}
        onClose={() => {
          setInviteOpen(false);
          refetch();
          refetchInvitations();
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
