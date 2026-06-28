import { useState } from "react";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useAtomValue } from "jotai";
import { toast } from "sonner";
import { ColumnDef } from "@tanstack/react-table";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
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
import { DataTable, RefetchButton, TextField } from "../../components/base";
import { PermissionGuardedButton } from "../../components/base/PermissionGuardedButton";
import { ResoniteUserIcon } from "../../components/ResoniteUserIcon";
import { usePermissions } from "../../hooks/usePermissions";
import { PERMISSION_KEYS } from "../../libs/permissionUtils";
import { formatTimestamp } from "../../libs/datetimeUtils";
import { sessionAtom } from "../../atoms/sessionAtom";

const inviteFormSchema = z.object({
  resoniteId: z
    .string()
    .min(1, "Resonite ID は必須です")
    .regex(/^U-/, "Resonite ID は U- から始まる必要があります"),
});
type InviteFormData = z.infer<typeof inviteFormSchema>;

function InviteUserDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { mutateAsync, isPending } = useMutation(createRegistrationToken);
  const [issued, setIssued] = useState<
    CreateRegistrationTokenResponse | undefined
  >(undefined);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<InviteFormData>({
    resolver: zodResolver(inviteFormSchema),
    defaultValues: { resoniteId: "" },
  });

  const handleClose = () => {
    reset();
    setIssued(undefined);
    onClose();
  };

  const onSubmit = async (data: InviteFormData) => {
    try {
      const res = await mutateAsync({ resoniteId: data.resoniteId });
      setIssued(res);
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : "招待トークンの発行に失敗しました",
      );
    }
  };

  const inviteUrl = issued
    ? `${window.location.origin}/register/${issued.token}`
    : "";

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(inviteUrl);
      toast.success("招待 URL をコピーしました");
    } catch {
      toast.error("クリップボードへのコピーに失敗しました");
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
          <DialogTitle>ユーザーを招待</DialogTitle>
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
                  以下のリンクを本人に共有してください
                </div>
              </div>
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium">招待 URL</label>
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
                  title="URL をコピー"
                >
                  <Copy />
                </Button>
              </div>
            </div>
            <div className="text-muted-foreground text-xs">
              有効期限: {formatTimestamp(issued.expiresAt)}
            </div>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="outline">閉じる</Button>
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
            <p className="text-muted-foreground text-xs">
              指定された Resonite ユーザー宛の登録リンクを発行します。
              リンクを開いたユーザーが ID とパスワードを設定して登録します。
            </p>
            <DialogFooter>
              <Button type="submit" form="invite-form" disabled={isPending}>
                {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                招待リンク発行
              </Button>
              <DialogClose asChild>
                <Button variant="outline" type="button">
                  キャンセル
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
  const { mutateAsync, isPending } = useMutation(deleteUser);

  const handleConfirm = async () => {
    if (!user) return;
    try {
      await mutateAsync({ userId: user.id });
      toast.success("ユーザーを削除しました");
      onDeleted();
      onClose();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "削除に失敗しました");
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
          <AlertDialogTitle>ユーザーを削除しますか?</AlertDialogTitle>
          <AlertDialogDescription>
            ユーザー <span className="font-mono">{user?.id}</span> (Resonite ID:{" "}
            <span className="font-mono">{user?.resoniteId}</span>)
            を削除します。この操作は取り消せません。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={isPending}>キャンセル</AlertDialogCancel>
          <AlertDialogAction
            disabled={isPending}
            onClick={(e) => {
              e.preventDefault();
              handleConfirm();
            }}
            className="bg-destructive text-white hover:bg-destructive/90"
          >
            {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            削除
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export default function AdminUsersPage() {
  const { hasSystemPermission, isPending: isPermPending } = usePermissions();
  const session = useAtomValue(sessionAtom);
  const canList = hasSystemPermission(PERMISSION_KEYS.SYSTEM_USER_LIST);
  const canCreate = hasSystemPermission(PERMISSION_KEYS.SYSTEM_USER_CREATE);
  const canDelete = hasSystemPermission(PERMISSION_KEYS.SYSTEM_USER_DELETE);

  const { data, isPending, refetch } = useQuery(
    listUsers,
    {},
    { enabled: canList },
  );
  const [inviteOpen, setInviteOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<User | undefined>(undefined);

  const currentUserId = session?.user.id;

  const columns: ColumnDef<User>[] = [
    {
      id: "icon",
      header: "アイコン",
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
      header: "ユーザーID",
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
      header: "作成日時",
      cell: ({ row }) => (
        <span className="text-xs">
          {formatTimestamp(row.original.createdAt)}
        </span>
      ),
    },
    {
      id: "actions",
      header: "操作",
      cell: ({ row }) => {
        const isSelf = row.original.id === currentUserId;
        const disabledReason = isSelf
          ? "自分自身は削除できません"
          : !canDelete
            ? "削除権限がありません"
            : undefined;
        return (
          <PermissionGuardedButton
            allowed={canDelete && !isSelf}
            disabledReason={disabledReason}
            variant="ghost"
            size="sm"
            onClick={() => setDeleteTarget(row.original)}
          >
            削除
          </PermissionGuardedButton>
        );
      },
    },
  ];

  if (isPermPending) return null;

  if (!canList) {
    return (
      <div className="container mx-auto p-4">
        <p className="text-destructive text-sm">権限がありません</p>
      </div>
    );
  }

  return (
    <div className="container mx-auto p-4 space-y-4">
      <p className="text-muted-foreground text-sm">
        システム上の全ユーザーを表示しています。
      </p>
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <PermissionGuardedButton
          allowed={canCreate}
          disabledReason="ユーザー作成権限がありません"
          onClick={() => setInviteOpen(true)}
        >
          ユーザーを招待
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
