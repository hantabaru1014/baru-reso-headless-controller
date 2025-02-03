import {
  Button,
  Card,
  CardContent,
  CardHeader,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  TextField,
} from "@mui/material";
import { useDialogs, DialogProps } from "@toolpad/core/useDialogs";
import UserList from "./base/UserList";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  createHeadlessAccount,
  listHeadlessAccounts,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import RefetchButton from "./base/RefetchButton";
import { useEffect, useState } from "react";
import { useNotifications } from "@toolpad/core/useNotifications";

function NewAccountDialog({ open, onClose }: DialogProps) {
  const { mutateAsync: mutateCreateAccount, isPending } = useMutation(
    createHeadlessAccount,
  );
  const [userId, setUserId] = useState("U-");
  const [credential, setCredential] = useState("");
  const [password, setPassword] = useState("");
  const notifications = useNotifications();

  useEffect(() => {
    if (open) {
      setUserId("U-");
      setCredential("");
      setPassword("");
    }
  }, [open]);

  return (
    <Dialog open={open} onClose={() => onClose()} fullWidth maxWidth="md">
      <DialogTitle>ヘッドレスアカウントを追加</DialogTitle>
      <DialogContent dividers>
        <Stack spacing={2}>
          <TextField
            label="User ID"
            value={userId}
            onChange={(e) => setUserId(e.target.value)}
          />
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
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button
          onClick={async () => {
            try {
              await mutateCreateAccount({
                resoniteUserId: userId,
                credential,
                password,
              });
            } catch (e) {
              notifications.show(e instanceof Error ? e.message : `${e}`, {
                severity: "error",
              });
              return;
            }
            onClose();
          }}
          loading={isPending}
          variant="contained"
          color="primary"
        >
          追加
        </Button>
        <Button onClick={() => onClose()}>キャンセル</Button>
      </DialogActions>
    </Dialog>
  );
}

export default function HeadlessAccountList() {
  const { data, isPending, refetch } = useQuery(listHeadlessAccounts);
  const dialogs = useDialogs();

  const handleNewAccount = async () => {
    await dialogs.open(NewAccountDialog);
    refetch();
  };

  return (
    <Card variant="outlined">
      <CardHeader
        title="ヘッドレスアカウント"
        action={
          <Stack spacing={2} direction="row">
            <RefetchButton refetch={refetch} />
            <Button
              onClick={handleNewAccount}
              variant="contained"
              color="primary"
            >
              追加
            </Button>
          </Stack>
        }
      />
      <CardContent>
        <UserList
          data={
            data?.accounts.map((account) => ({
              id: account.userId,
              name: account.userName,
              iconUrl: account.iconUrl,
            })) ?? []
          }
          isLoading={isPending}
        />
      </CardContent>
    </Card>
  );
}
