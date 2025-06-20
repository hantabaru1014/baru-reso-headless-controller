import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  listHeadlessAccounts,
  listHeadlessHost,
  listHeadlessHostImageTags,
  startHeadlessHost,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { ColumnDef } from "@tanstack/react-table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "./ui/dialog";
import { Button } from "./ui/button";
import { Avatar, AvatarImage, AvatarFallback } from "./ui/avatar";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "./ui/collapsible";
import { ChevronDown } from "lucide-react";
import prettyBytes from "../libs/prettyBytes";
import { useNavigate } from "react-router";
import { hostStatusToLabel } from "../libs/hostUtils";
import { RefetchButton } from "./base/RefetchButton";
import { UserList, type UserInfo } from "./base/UserList";
import { useEffect, useMemo, useState } from "react";
import { SelectField } from "./base/SelectField";
import { HeadlessHost } from "front/pbgen/hdlctrl/v1/controller_pb";
import { DataTable } from "./base/DataTable";
import { toast } from "sonner";
import { TextField } from "./base";

function SelectHeadlessAccountDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: (result: UserInfo | null) => void;
}) {
  const { data, isPending } = useQuery(listHeadlessAccounts);

  return (
    <Dialog open={open} onOpenChange={(open) => !open && onClose(null)}>
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>ヘッドレスアカウントを選択</DialogTitle>
        </DialogHeader>
        <UserList
          data={
            data?.accounts.map((account) => ({
              id: account.userId,
              name: account.userName,
              iconUrl: account.iconUrl,
            })) ?? []
          }
          isLoading={isPending}
          renderActions={(account) => (
            <Button onClick={() => onClose(account)}>選択</Button>
          )}
        />
        <DialogFooter>
          <Button variant="outline" onClick={() => onClose(null)}>
            キャンセル
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function NewHostDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { data: tags } = useQuery(listHeadlessHostImageTags);
  const { mutateAsync: mutateStartHost, isPending } =
    useMutation(startHeadlessHost);
  const [name, setName] = useState("");
  const [universeId, setUniverseId] = useState("");
  const [usernameOverride, setUsernameOverride] = useState("");
  const [tag, setTag] = useState("");
  const [account, setAccount] = useState<UserInfo | null>(null);
  const [isAccountDialogOpen, setIsAccountDialogOpen] = useState(false);
  const [isAdvancedOpen, setIsAdvancedOpen] = useState(false);

  useEffect(() => {
    if (open) {
      setName("");
      setAccount(null);
      setTag("");
    }
  }, [open]);

  const tagOptions = useMemo(() => {
    const list = tags?.tags ?? [];
    list.reverse();

    return list.slice(0, 5).map((tag) => ({
      id: tag.tag,
      label: tag.tag,
    }));
  }, [tags]);

  useEffect(() => {
    const releaseTags =
      tags?.tags.filter((t) => !t.isPrerelease).map((t) => t.tag) ?? [];
    if (!tag && releaseTags.length > 0) {
      setTag(releaseTags[releaseTags.length - 1] ?? "");
    }
  }, [tags]);

  return (
    <>
      <Dialog open={open} onOpenChange={(open) => !open && onClose()}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>ヘッドレスを開始</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <TextField
              label="ホスト名"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <SelectField
              label="バージョン"
              options={tagOptions}
              selectedId={tag}
              onChange={(option) => setTag(option.id)}
            />
            {account ? (
              <div className="flex items-center gap-2">
                <Avatar>
                  <AvatarImage src={account.iconUrl} alt={account.name} />
                  <AvatarFallback>{account.name.charAt(0)}</AvatarFallback>
                </Avatar>
                <span className="text-sm font-medium">{account.name}</span>
              </div>
            ) : (
              <Button
                variant="outline"
                onClick={() => setIsAccountDialogOpen(true)}
              >
                ホストユーザを選択
              </Button>
            )}
            <Collapsible open={isAdvancedOpen} onOpenChange={setIsAdvancedOpen}>
              <CollapsibleTrigger className="flex w-full items-center justify-between py-2">
                <span className="text-sm font-medium">詳細設定(任意)</span>
                <ChevronDown className="h-4 w-4" />
              </CollapsibleTrigger>
              <CollapsibleContent className="space-y-2">
                <TextField
                  label="Universe ID"
                  value={universeId}
                  onChange={(e) => setUniverseId(e.target.value)}
                />
                <TextField
                  label="Username Override"
                  value={usernameOverride}
                  onChange={(e) => setUsernameOverride(e.target.value)}
                />
              </CollapsibleContent>
            </Collapsible>
          </div>
          <DialogFooter>
            <Button
              onClick={async () => {
                try {
                  await mutateStartHost({
                    name,
                    headlessAccountId: account?.id ?? "",
                    imageTag: tag,
                    startupConfig: {
                      universeId: universeId ? universeId : undefined,
                      usernameOverride: usernameOverride
                        ? usernameOverride
                        : undefined,
                    },
                  });
                  onClose();
                } catch (e) {
                  toast.error(
                    e instanceof Error
                      ? e.message
                      : "ホストの開始に失敗しました",
                  );
                }
              }}
              disabled={isPending || !account || !name || !tag}
            >
              開始
            </Button>
            <Button variant="outline" onClick={() => onClose()}>
              キャンセル
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <SelectHeadlessAccountDialog
        open={isAccountDialogOpen}
        onClose={(result) => {
          setIsAccountDialogOpen(false);
          if (result) {
            setAccount(result);
          }
        }}
      />
    </>
  );
}

const columns: ColumnDef<HeadlessHost>[] = [
  {
    accessorKey: "id",
    header: "ID",
  },
  {
    accessorKey: "name",
    header: "名前",
  },
  {
    accessorKey: "status",
    header: "ステータス",
    cell: ({ row }) => hostStatusToLabel(row.original.status),
  },
  {
    accessorKey: "resoniteVersion",
    header: "Resonite Ver",
  },
  {
    accessorKey: "fps",
    header: "fps",
  },
  {
    accessorKey: "accountName",
    header: "アカウント名",
  },
  {
    accessorKey: "storageUsedBytes",
    header: "ストレージ",
    cell: ({ row }) =>
      `${prettyBytes(Number(row.original.storageUsedBytes))}/${prettyBytes(
        Number(row.original.storageQuotaBytes),
      )}`,
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
