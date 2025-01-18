import {
  Avatar,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  List,
  ListItem,
  ListItemAvatar,
  ListItemText,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
} from "@mui/material";
import { CheckOutlined, Refresh as RefreshIcon } from "@mui/icons-material";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  banUser,
  inviteUser,
  kickUser,
  listUsersInSession,
  searchUserInfo,
  updateUserRole,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useAtom } from "jotai";
import { selectedHostAtom } from "../atoms/selectedHostAtom";
import Loading from "./base/Loading";
import { UserRoles } from "../constants";
import EditableSelectField from "./base/EditableSelectField";
import { useState } from "react";
import { useNotifications } from "@toolpad/core/useNotifications";
import { useDialogs, DialogProps } from "@toolpad/core/useDialogs";

function UserInviteDialog({
  open,
  onClose,
  payload: { sessionId },
}: DialogProps<{ sessionId: string }>) {
  const [selectedHost] = useAtom(selectedHostAtom);
  const notifications = useNotifications();
  const [query, setQuery] = useState("");
  const {
    data: searchResult,
    mutateAsync: mutateSearch,
    isPending: isPendingSearch,
  } = useMutation(searchUserInfo);
  const { mutateAsync: mutateInviteUser } = useMutation(inviteUser);

  const handleQueryChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value.toLowerCase();
    setQuery(value);
    const isId = value.startsWith("u-");
    mutateSearch({
      hostId: selectedHost?.id,
      parameters: {
        user: {
          case: isId ? "userId" : "userName",
          value,
        },
        onlyInContacts: true,
        partialMatch: true,
      },
    });
  };

  const handleInviteUser = async (userId: string) => {
    try {
      await mutateInviteUser({
        hostId: selectedHost?.id,
        sessionId: sessionId,
        user: {
          case: "userId",
          value: userId,
        },
      });
      notifications.show("ユーザーを招待しました", {
        severity: "success",
        autoHideDuration: 1500,
      });
    } catch (e) {
      notifications.show(`ユーザーの招待に失敗しました: ${e}`, {
        severity: "error",
        autoHideDuration: 1500,
      });
    }
  };

  return (
    <Dialog open={open} onClose={() => onClose()} fullWidth maxWidth="md">
      <DialogTitle>ユーザーを招待</DialogTitle>
      <DialogContent dividers>
        <Stack spacing={2}>
          <TextField
            variant="filled"
            label="ユーザーID/名"
            value={query}
            onChange={handleQueryChange}
          />
          <Loading loading={isPendingSearch}>
            <List>
              {searchResult?.users.map((user) => (
                <ListItem
                  key={user.id}
                  secondaryAction={
                    <Button onClick={() => handleInviteUser(user.id)}>
                      Invite
                    </Button>
                  }
                >
                  <ListItemAvatar>
                    <Avatar></Avatar>
                  </ListItemAvatar>
                  <ListItemText primary={user.name} />
                </ListItem>
              ))}
            </List>
          </Loading>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => onClose()}>閉じる</Button>
      </DialogActions>
    </Dialog>
  );
}

export default function SessionUserList({ sessionId }: { sessionId: string }) {
  const [selectedHost] = useAtom(selectedHostAtom);
  const dialogs = useDialogs();
  const { data, status, refetch } = useQuery(listUsersInSession, {
    hostId: selectedHost?.id,
    sessionId,
  });
  const { mutateAsync: mutateUpdateRole } = useMutation(updateUserRole);
  const { mutateAsync: mutateKickUser } = useMutation(kickUser);
  const { mutateAsync: mutateBanUser } = useMutation(banUser);

  const handleUpdateRole = async (userId: string, role: string) => {
    try {
      await mutateUpdateRole({
        hostId: selectedHost?.id,
        parameters: {
          sessionId,
          user: {
            case: "userId",
            value: userId,
          },
          role,
        },
      });
      refetch();
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  const handleKickUser = async (userId: string) => {
    try {
      await mutateKickUser({
        hostId: selectedHost?.id,
        parameters: {
          sessionId,
          user: {
            case: "userId",
            value: userId,
          },
        },
      });
      refetch();
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  const handleBanUser = async (userId: string) => {
    try {
      await mutateBanUser({
        hostId: selectedHost?.id,
        parameters: {
          sessionId,
          user: {
            case: "userId",
            value: userId,
          },
        },
      });
      refetch();
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  const handleOpenInviteDialog = async () => {
    await dialogs.open(UserInviteDialog, { sessionId });
    refetch();
  };

  return (
    <Stack spacing={2}>
      <Stack direction="row" spacing={2} sx={{ justifyContent: "flex-end" }}>
        <Button
          variant="contained"
          color="primary"
          onClick={handleOpenInviteDialog}
        >
          Invite
        </Button>
        <IconButton aria-label="再読み込み" onClick={() => refetch()}>
          <RefreshIcon />
        </IconButton>
      </Stack>
      <Loading loading={status === "pending"}>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>ユーザー名</TableCell>
                <TableCell>権限</TableCell>
                <TableCell>離席中</TableCell>
                <TableCell>操作</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {data?.users.map((user) => (
                <TableRow key={user.id}>
                  <TableCell>{user.name}</TableCell>
                  <TableCell>
                    <EditableSelectField<string>
                      selectedId={user.role}
                      options={UserRoles.map((r) => r)}
                      onSave={(v) => handleUpdateRole(user.id, v)}
                    />
                  </TableCell>
                  <TableCell>{!user.isPresent && <CheckOutlined />}</TableCell>
                  <TableCell>
                    <Button onClick={() => handleKickUser(user.id)}>
                      Kick
                    </Button>
                    <Button onClick={() => handleBanUser(user.id)}>Ban</Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Loading>
    </Stack>
  );
}
