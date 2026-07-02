import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useState } from "react";
import { useNavigate } from "react-router";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { toast } from "sonner";
import { ColumnDef } from "@tanstack/react-table";
import {
  createGroup,
  listGroups,
} from "../../pbgen/hdlctrl/v1/permission-GroupService_connectquery";
import { Group, GroupType } from "../../pbgen/hdlctrl/v1/permission_pb";
import {
  Button,
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui";
import { DataTable, RefetchButton, TextField } from "./base";
import { PermissionGuardedButton } from "./base/PermissionGuardedButton";
import { usePermissions } from "../hooks/usePermissions";
import { useInvalidateMyPermissions } from "../hooks/useInvalidateMyPermissions";
import { PERMISSION_KEYS, groupTypeToLabel } from "../libs/permissionUtils";

const newGroupFormSchema = z.object({
  name: z.string().min(1, "グループ名は必須です"),
});
type NewGroupFormData = z.infer<typeof newGroupFormSchema>;

function NewGroupDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose?: () => void;
  /** 作成成功時. 引数の groupId は新規グループ ID. */
  onCreated?: (groupId: string) => void;
}) {
  const { mutateAsync, isPending } = useMutation(createGroup);

  const {
    control,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<NewGroupFormData>({
    resolver: zodResolver(newGroupFormSchema),
    defaultValues: { name: "" },
  });

  const onSubmit = async (data: NewGroupFormData) => {
    try {
      const res = await mutateAsync({ name: data.name });
      toast.success("グループを作成しました");
      reset();
      if (res.group?.id) onCreated?.(res.group.id);
      onClose?.();
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : "グループの作成に失敗しました",
      );
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
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>新規グループを作成</DialogTitle>
        </DialogHeader>
        <form
          id="new-group-form"
          onSubmit={handleSubmit(onSubmit)}
          className="space-y-4"
        >
          <Controller
            name="name"
            control={control}
            render={({ field }) => (
              <TextField
                label="グループ名"
                {...field}
                error={errors.name?.message}
              />
            )}
          />
        </form>
        <DialogFooter>
          <Button type="submit" form="new-group-form" disabled={isPending}>
            作成
          </Button>
          <DialogClose asChild>
            <Button variant="outline">キャンセル</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const columns: ColumnDef<Group>[] = [
  {
    accessorKey: "name",
    header: "名前",
  },
  {
    accessorKey: "type",
    header: "種別",
    cell: ({ cell }) => groupTypeToLabel(cell.getValue<GroupType>()),
  },
  {
    accessorKey: "id",
    header: "ID",
    cell: ({ cell }) => (
      <span className="font-mono text-xs">{cell.getValue<string>()}</span>
    ),
  },
];

export default function GroupList() {
  const navigate = useNavigate();
  const { data, isPending, refetch } = useQuery(listGroups, {});
  const [isNewDialogOpen, setIsNewDialogOpen] = useState(false);
  const { hasSystemPermission } = usePermissions();
  const invalidateMyPermissions = useInvalidateMyPermissions();
  const canCreate = hasSystemPermission(PERMISSION_KEYS.SYSTEM_GROUP_MANAGE);

  const handleCreated = async (newGroupId: string) => {
    // 新規グループへの権限を即時反映するため getMyPermissions を invalidate.
    await invalidateMyPermissions();
    navigate(`/groups/${newGroupId}`);
  };

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <PermissionGuardedButton
          allowed={canCreate}
          disabledReason="グループ作成権限がありません"
          onClick={() => setIsNewDialogOpen(true)}
        >
          新規グループ
        </PermissionGuardedButton>
      </div>
      <DataTable
        columns={columns}
        data={data?.groups ?? []}
        isLoading={isPending}
        onClickRow={(row) => navigate(`/groups/${row.id}`)}
      />
      <NewGroupDialog
        open={isNewDialogOpen}
        onClose={() => {
          setIsNewDialogOpen(false);
          refetch();
        }}
        onCreated={handleCreated}
      />
    </div>
  );
}
