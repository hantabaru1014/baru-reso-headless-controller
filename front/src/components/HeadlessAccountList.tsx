import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  Button,
  Avatar,
  AvatarImage,
  AvatarFallback,
} from "./ui";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  createHeadlessAccount,
  listHeadlessAccounts,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { RefetchButton } from "./base/RefetchButton";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { DataTable, TextField } from "./base";
import { ColumnDef } from "@tanstack/react-table";
import { HeadlessAccount } from "front/pbgen/hdlctrl/v1/controller_pb";
import prettyBytes from "@/libs/prettyBytes";
import { resolveUrl } from "@/libs/skyfrostUtils";

function NewAccountDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { mutateAsync: mutateCreateAccount, isPending } = useMutation(
    createHeadlessAccount,
  );
  const [credential, setCredential] = useState("");
  const [password, setPassword] = useState("");

  useEffect(() => {
    if (open) {
      setCredential("");
      setPassword("");
    }
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>ヘッドレスアカウントを追加</DialogTitle>
        </DialogHeader>
        <div className="grid gap-4 py-4">
          <TextField
            label="Credential"
            value={credential}
            onChange={(e) => setCredential(e.target.value)}
          />
          <TextField
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        <DialogFooter>
          <Button
            onClick={async () => {
              try {
                await mutateCreateAccount({
                  credential,
                  password,
                });
                toast.success("アカウントを追加しました");
              } catch (e) {
                toast.error(
                  e instanceof Error
                    ? e.message
                    : "アカウントの追加に失敗しました",
                );
                return;
              }
              onClose();
            }}
            disabled={isPending}
          >
            追加
          </Button>
          <Button variant="outline" onClick={() => onClose()}>
            キャンセル
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const columns: ColumnDef<HeadlessAccount>[] = [
  {
    accessorKey: "iconUrl",
    header: "アイコン",
    cell: ({ row }) => {
      return (
        <Avatar>
          <AvatarImage
            src={resolveUrl(row.original.iconUrl)}
            alt={`${row.original.userName}のアイコン`}
          />
          <AvatarFallback>{row.original.userName.charAt(0)}</AvatarFallback>
        </Avatar>
      );
    },
  },
  {
    accessorKey: "userName",
    header: "ユーザ名",
  },
  {
    header: "ストレージ",
    cell: ({ row }) => (
      <span>
        {prettyBytes(Number(row.original.storageUsedBytes))}/
        {prettyBytes(Number(row.original.storageQuotaBytes))}
      </span>
    ),
  },
];

export default function HeadlessAccountList() {
  const { data, isPending, refetch } = useQuery(listHeadlessAccounts);
  const [isDialogOpen, setIsDialogOpen] = useState(false);

  const handleNewAccount = () => {
    setIsDialogOpen(true);
  };

  const handleCloseDialog = () => {
    setIsDialogOpen(false);
    refetch();
  };

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <Button onClick={handleNewAccount}>追加</Button>
      </div>
      <DataTable
        columns={columns}
        data={data?.accounts || []}
        isLoading={isPending}
      />
      <NewAccountDialog open={isDialogOpen} onClose={handleCloseDialog} />
    </div>
  );
}
