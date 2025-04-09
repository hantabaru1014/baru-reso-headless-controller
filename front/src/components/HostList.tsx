import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  listHeadlessAccounts,
  listHeadlessHost,
  listHeadlessHostImageTags,
  startHeadlessHost,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  Avatar,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Grid2,
  Skeleton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from "@mui/material";
import prettyBytes from "../libs/prettyBytes";
import { useNavigate } from "react-router";
import { hostStatusToLabel } from "../libs/hostUtils";
import RefetchButton from "./base/RefetchButton";
import { DialogProps, useDialogs } from "@toolpad/core/useDialogs";
import UserList, { UserInfo } from "./base/UserList";
import { useEffect, useMemo, useState } from "react";
import { useNotifications } from "@toolpad/core/useNotifications";
import SelectField from "./base/SelectField";

function SelectHeadlessAccountDialog({
  open,
  onClose,
}: DialogProps<undefined, UserInfo | null>) {
  const { data, isPending } = useQuery(listHeadlessAccounts);

  return (
    <Dialog open={open} onClose={() => onClose(null)} fullWidth maxWidth="sm">
      <DialogTitle>ヘッドレスアカウントを選択</DialogTitle>
      <DialogContent dividers>
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
      </DialogContent>
      <DialogActions>
        <Button onClick={() => onClose(null)}>キャンセル</Button>
      </DialogActions>
    </Dialog>
  );
}

function NewHostDialog({ open, onClose }: DialogProps) {
  const { data: tags } = useQuery(listHeadlessHostImageTags);
  const { mutateAsync: mutateStartHost, isPending } =
    useMutation(startHeadlessHost);
  const [name, setName] = useState("");
  const [tag, setTag] = useState("");
  const [account, setAccount] = useState<UserInfo | null>(null);
  const notifications = useNotifications();
  const dialogs = useDialogs();

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
      setTag(releaseTags.at(-1) ?? "");
    }
  }, [tags]);

  return (
    <Dialog open={open} onClose={() => onClose()} fullWidth maxWidth="md">
      <DialogTitle>ヘッドレスを開始</DialogTitle>
      <DialogContent dividers>
        <Stack spacing={2}>
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
            <Stack spacing={2} direction="row" alignItems="center">
              <Avatar src={account.iconUrl} />
              <Typography>{account.name}</Typography>
            </Stack>
          ) : (
            <Button
              onClick={async () => {
                const result = await dialogs.open(SelectHeadlessAccountDialog);
                if (result) {
                  setAccount(result);
                }
              }}
            >
              ホストユーザを選択
            </Button>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button
          onClick={async () => {
            try {
              await mutateStartHost({
                name: name,
                headlessAccountId: account?.id ?? "",
                imageTag: tag,
              });
              onClose();
            } catch (e) {
              notifications.show(e instanceof Error ? e.message : `${e}`, {
                severity: "error",
              });
            }
          }}
          disabled={isPending || !account || !name || !tag}
        >
          開始
        </Button>
        <Button onClick={() => onClose()}>キャンセル</Button>
      </DialogActions>
    </Dialog>
  );
}

export default function HostList() {
  const { data, isPending, refetch } = useQuery(listHeadlessHost);
  const navigate = useNavigate();
  const dialogs = useDialogs();

  return (
    <Grid2 container>
      <Grid2 size={12}>
        <Stack direction="row" spacing={2} sx={{ justifyContent: "flex-end" }}>
          <RefetchButton refetch={refetch} />
          <Button
            variant="contained"
            color="primary"
            onClick={async () => {
              await dialogs.open(NewHostDialog);
              refetch();
            }}
          >
            ヘッドレスを開始
          </Button>
        </Stack>
      </Grid2>
      <Grid2 size={12}>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>ID</TableCell>
                <TableCell>名前</TableCell>
                <TableCell>ステータス</TableCell>
                <TableCell>Resoniteバージョン</TableCell>
                <TableCell>fps</TableCell>
                <TableCell>アカウント名</TableCell>
                <TableCell>ストレージ</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {data?.hosts.map((host) => (
                <TableRow
                  key={host.id}
                  onClick={() => navigate(`/hosts/${host.id}`)}
                  hover
                  sx={{ cursor: "pointer" }}
                >
                  <TableCell>{host.id.substring(0, 12)}</TableCell>
                  <TableCell>{host.name}</TableCell>
                  <TableCell>{hostStatusToLabel(host.status)}</TableCell>
                  <TableCell>{host.resoniteVersion}</TableCell>
                  <TableCell>{host.fps}</TableCell>
                  <TableCell>{host.accountName}</TableCell>
                  <TableCell>{`${prettyBytes(Number(host.storageUsedBytes))}/${prettyBytes(Number(host.storageQuotaBytes))}`}</TableCell>
                </TableRow>
              ))}
              {isPending && (
                <>
                  <TableRow>
                    <Skeleton variant="rectangular" />
                  </TableRow>
                  <TableRow>
                    <Skeleton variant="rectangular" />
                  </TableRow>
                  <TableRow>
                    <Skeleton variant="rectangular" />
                  </TableRow>
                </>
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Grid2>
    </Grid2>
  );
}
