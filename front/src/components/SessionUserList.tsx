import {
  Button,
  IconButton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from "@mui/material";
import { CheckOutlined, Refresh as RefreshIcon } from "@mui/icons-material";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  listUsersInSession,
  updateUserRole,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useAtom } from "jotai";
import { selectedHostAtom } from "../atoms/selectedHostAtom";
import Loading from "./base/Loading";
import { UserRoles } from "../constants";
import EditableSelectField from "./base/EditableSelectField";

export default function SessionUserList({ sessionId }: { sessionId: string }) {
  const [selectedHost] = useAtom(selectedHostAtom);
  const { data, status, refetch } = useQuery(listUsersInSession, {
    hostId: selectedHost?.id,
    sessionId,
  });
  const { mutateAsync: mutateUpdateRole } = useMutation(updateUserRole);

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

  return (
    <Stack spacing={2}>
      <Stack direction="row" spacing={2} sx={{ justifyContent: "flex-end" }}>
        <Button variant="contained" color="primary">
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
                    <Button>Kick</Button>
                    <Button>Ban</Button>
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
