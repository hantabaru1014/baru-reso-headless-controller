import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  listHeadlessAccounts,
  listHeadlessHost,
  startHeadlessHost,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { ColumnDef } from "@tanstack/react-table";
import { keepPreviousData } from "@tanstack/react-query";
import { usePaginationState } from "../hooks/usePaginationState";
import { useDefaultGroupId } from "../hooks/useDefaultGroupId";
import { useAtomValue } from "jotai";
import { currentGroupIdAtom } from "../atoms/currentGroupAtom";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogClose,
} from "./ui/dialog";
import { Button } from "./ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "./ui/collapsible";
import { ChevronDown } from "lucide-react";
import { useNavigate } from "react-router";
import { hostStatusToLabel } from "../libs/hostUtils";
import { RefetchButton } from "./base/RefetchButton";
import { useEffect, useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { SelectField } from "./base/SelectField";
import {
  HeadlessHost,
  HeadlessHostAutoUpdatePolicy,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import { DataTable } from "./base/DataTable";
import { toast } from "sonner";
import { CheckboxField, TextField } from "./base";
import { ResoniteUserIcon } from "./ResoniteUserIcon";
import { GroupSelectField } from "./GroupSelectField";
import { PermissionGuardedButton } from "./base/PermissionGuardedButton";
import { usePermissions } from "../hooks/usePermissions";
import { PERMISSION_KEYS } from "../libs/permissionUtils";

const newHostFormSchema = z.object({
  name: z.string().min(1, "ホスト名は必須です"),
  universeId: z.string().optional(),
  usernameOverride: z.string().optional(),
  tag: z.string().min(1, "バージョンは必須です"),
  accountId: z.string().min(1, "ホストユーザは必須です"),
  groupId: z.string().min(1, "所属グループは必須です"),
  autoUpdate: z.boolean(),
});
type NewHostFormData = z.infer<typeof newHostFormSchema>;

function NewHostDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose?: () => void;
}) {
  const { data: accounts } = useQuery(listHeadlessAccounts);
  const { mutateAsync: mutateStartHost, isPending } =
    useMutation(startHeadlessHost);
  const [isAdvancedOpen, setIsAdvancedOpen] = useState(false);
  const defaultGroupId = useDefaultGroupId(PERMISSION_KEYS.HOST_WRITE);

  const {
    control,
    handleSubmit,
    reset,
    watch,
    setValue,
    getValues,
    formState: { errors },
  } = useForm<NewHostFormData>({
    resolver: zodResolver(newHostFormSchema),
    defaultValues: {
      name: "",
      universeId: "",
      usernameOverride: "",
      tag: "latestRelease",
      accountId: "",
      groupId: "",
      autoUpdate: false,
    },
  });

  const selectedGroupId = watch("groupId");

  // 非同期に解決される default group をユーザーが触っていなければ反映する.
  useEffect(() => {
    if (defaultGroupId && !getValues("groupId")) {
      setValue("groupId", defaultGroupId);
    }
  }, [defaultGroupId, getValues, setValue]);

  const onSubmit = async (data: NewHostFormData) => {
    try {
      await mutateStartHost({
        name: data.name,
        headlessAccountId: data.accountId,
        imageTag: data.tag,
        startupConfig: {
          universeId: data.universeId || undefined,
          usernameOverride: data.usernameOverride || undefined,
        },
        autoUpdatePolicy: data.autoUpdate
          ? HeadlessHostAutoUpdatePolicy.USERS_EMPTY
          : HeadlessHostAutoUpdatePolicy.NEVER,
        groupId: data.groupId || undefined,
      });
      // 非同期 job として実行されるので「受け付けた」だけ通知し、
      // 完了は notificationDispatch 経由の JobCompletedEvent toast で出す.
      toast.success("ホストの起動を受け付けました");
      onClose?.();
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : "ホストの開始に失敗しました",
      );
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(open) => {
        if (!open) {
          onClose?.();
          reset();
        }
      }}
    >
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>ヘッドレスを開始</DialogTitle>
        </DialogHeader>
        <form
          id="new-host-form"
          onSubmit={handleSubmit(onSubmit)}
          className="space-y-4"
        >
          <Controller
            name="name"
            control={control}
            render={({ field }) => (
              <TextField
                label="ホスト名"
                {...field}
                error={errors.name?.message}
              />
            )}
          />
          <Controller
            name="tag"
            control={control}
            render={({ field }) => (
              <SelectField
                label="バージョン"
                options={[
                  { id: "latestRelease", label: "最新リリース" },
                  { id: "latestPreRelease", label: "最新プレリリース" },
                ]}
                selectedId={field.value}
                onChange={(option) => field.onChange(option.id)}
                error={errors.tag?.message}
              />
            )}
          />
          <Controller
            name="groupId"
            control={control}
            render={({ field }) => (
              <GroupSelectField
                value={field.value}
                onChange={field.onChange}
                requiredPermission={PERMISSION_KEYS.HOST_WRITE}
                helperText="ホストを所属させるグループ。アカウントは同じグループのものから選びます"
                error={errors.groupId?.message}
              />
            )}
          />
          <Controller
            name="accountId"
            control={control}
            render={({ field }) => (
              <SelectField
                label="ホストユーザ"
                helperText="選択中のグループに所属するアカウントのみ表示されます"
                options={
                  accounts?.accounts
                    .filter(
                      (account) =>
                        !selectedGroupId || account.groupId === selectedGroupId,
                    )
                    .map((account) => ({
                      id: account.userId,
                      label: (
                        <span className="flex items-center gap-2">
                          <ResoniteUserIcon
                            iconUrl={account.iconUrl}
                            alt={`${account.userName}のアイコン`}
                          />
                          <span className="text-sm font-medium">
                            {account.userName}
                          </span>
                        </span>
                      ),
                    })) ?? []
                }
                selectedId={field.value}
                onChange={(option) => field.onChange(option.id)}
                error={errors.accountId?.message}
              />
            )}
          />
          <Controller
            name="autoUpdate"
            control={control}
            render={({ field }) => (
              <CheckboxField
                label="自動アップグレード"
                helperText="新しいバージョンがリリースされたら、セッション参加者が 0 人になった瞬間に自動で最新バージョンへ再起動します"
                checked={field.value}
                onCheckedChange={(checked) => field.onChange(checked === true)}
              />
            )}
          />
          <Collapsible open={isAdvancedOpen} onOpenChange={setIsAdvancedOpen}>
            <CollapsibleTrigger className="flex w-full items-center justify-between py-2">
              <span className="text-sm font-medium">詳細設定(任意)</span>
              <ChevronDown className="h-4 w-4" />
            </CollapsibleTrigger>
            <CollapsibleContent className="space-y-2">
              <Controller
                name="universeId"
                control={control}
                render={({ field }) => (
                  <TextField
                    label="Universe ID"
                    {...field}
                    error={errors.universeId?.message}
                  />
                )}
              />
              <Controller
                name="usernameOverride"
                control={control}
                render={({ field }) => (
                  <TextField
                    label="Username Override"
                    {...field}
                    error={errors.usernameOverride?.message}
                  />
                )}
              />
            </CollapsibleContent>
          </Collapsible>
        </form>
        <DialogFooter>
          <Button type="submit" form="new-host-form" disabled={isPending}>
            開始
          </Button>
          <DialogClose asChild>
            <Button variant="outline">キャンセル</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const columns: ColumnDef<HeadlessHost>[] = [
  {
    accessorKey: "id",
    header: "ID",
    cell: ({ cell }) => cell.getValue<string>().slice(0, 16),
  },
  {
    accessorKey: "name",
    header: "名前",
  },
  {
    accessorKey: "accountName",
    header: "アカウント名",
  },
  {
    accessorKey: "status",
    header: "ステータス",
    cell: ({ row }) => hostStatusToLabel(row.original.status),
  },
  {
    accessorKey: "resoniteVersion",
    header: "バージョン",
    cell: ({ row }) =>
      row.original.resoniteVersion
        ? `${row.original.resoniteVersion} (v${row.original.appVersion})`
        : "不明",
  },
  {
    accessorKey: "fps",
    header: "fps",
    cell: ({ row }) =>
      row.original.fps ? Math.floor(row.original.fps * 10) / 10 : "N/A",
  },
];

export default function HostList() {
  const { pageIndex, pageSize, setPageIndex, setPageSize } = usePaginationState(
    { defaultPageSize: 20 },
  );
  const currentGroupId = useAtomValue(currentGroupIdAtom);
  const { data, isPending, refetch } = useQuery(
    listHeadlessHost,
    {
      page: { pageIndex, pageSize },
      groupId: currentGroupId ?? undefined,
    },
    { placeholderData: keepPreviousData },
  );
  const navigate = useNavigate();
  const [isNewHostDialogOpen, setIsNewHostDialogOpen] = useState(false);
  const { groupsWithPermission } = usePermissions();
  const canStartHost =
    groupsWithPermission(PERMISSION_KEYS.HOST_WRITE).length > 0;

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <PermissionGuardedButton
          allowed={canStartHost}
          disabledReason="ホストを起動できる権限を持つグループがありません"
          onClick={() => setIsNewHostDialogOpen(true)}
        >
          ヘッドレスを開始
        </PermissionGuardedButton>
      </div>
      <DataTable
        columns={columns}
        data={data?.hosts ?? []}
        isLoading={isPending}
        onClickRow={(row) => navigate(`/hosts/${row.id}`)}
        pagination={{
          pageIndex,
          pageSize,
          totalCount: data?.page?.totalCount ?? 0,
          onPageIndexChange: setPageIndex,
          onPageSizeChange: setPageSize,
        }}
      />
      <NewHostDialog
        open={isNewHostDialogOpen}
        onClose={() => {
          setIsNewHostDialogOpen(false);
          refetch();
        }}
      />
    </div>
  );
}
