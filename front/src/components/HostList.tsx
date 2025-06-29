import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  listHeadlessAccounts,
  listHeadlessHost,
  startHeadlessHost,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { ColumnDef } from "@tanstack/react-table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogClose,
} from "./ui/dialog";
import { Button } from "./ui/button";
import { Avatar, AvatarImage, AvatarFallback } from "./ui/avatar";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "./ui/collapsible";
import { ChevronDown } from "lucide-react";
import { useNavigate } from "react-router";
import { hostStatusToLabel } from "../libs/hostUtils";
import { RefetchButton } from "./base/RefetchButton";
import { useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { SelectField } from "./base/SelectField";
import { HeadlessHost } from "front/pbgen/hdlctrl/v1/controller_pb";
import { DataTable } from "./base/DataTable";
import { toast } from "sonner";
import { TextField } from "./base";
import { resolveUrl } from "@/libs/skyfrostUtils";

const newHostFormSchema = z.object({
  name: z.string().min(1, "ホスト名は必須です"),
  universeId: z.string().optional(),
  usernameOverride: z.string().optional(),
  tag: z.string().min(1, "バージョンは必須です"),
  accountId: z.string().min(1, "ホストユーザは必須です"),
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

  const {
    control,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<NewHostFormData>({
    resolver: zodResolver(newHostFormSchema),
    defaultValues: {
      name: "",
      universeId: "",
      usernameOverride: "",
      tag: "latestRelease",
      accountId: "",
    },
  });

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
      });
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
            name="accountId"
            control={control}
            render={({ field }) => (
              <SelectField
                label="ホストユーザ"
                options={
                  accounts?.accounts.map((account) => ({
                    id: account.userId,
                    label: (
                      <span className="flex items-center gap-2">
                        <Avatar>
                          <AvatarImage
                            src={resolveUrl(account.iconUrl)}
                            alt={account.userName}
                          />
                          <AvatarFallback>
                            {account.userName.charAt(0)}
                          </AvatarFallback>
                        </Avatar>
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
  const { data, isPending, refetch } = useQuery(listHeadlessHost);
  const navigate = useNavigate();
  const [isNewHostDialogOpen, setIsNewHostDialogOpen] = useState(false);

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <Button onClick={() => setIsNewHostDialogOpen(true)}>
          ヘッドレスを開始
        </Button>
      </div>
      <DataTable
        columns={columns}
        data={data?.hosts ?? []}
        isLoading={isPending}
        onClickRow={(row) => navigate(`/hosts/${row.id}`)}
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
