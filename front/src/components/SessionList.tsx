import {
  Button,
  Skeleton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from "@mui/material";
import { useQuery } from "@connectrpc/connect-query";
import { listSessions } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useAtom } from "jotai";
import { selectedHostAtom } from "../atoms/selectedHostAtom";
import { useNavigate } from "react-router";
import { AccessLevels } from "../constants";
import RefetchButton from "./base/RefetchButton";

export default function SessionList() {
  const [selectedHost] = useAtom(selectedHostAtom);
  const { data, status, refetch } = useQuery(listSessions, {
    hostId: selectedHost?.id,
  });
  const navigate = useNavigate();

  return (
    <Stack spacing={2}>
      <Stack direction="row" spacing={2} sx={{ justifyContent: "flex-end" }}>
        <Button
          variant="contained"
          color="primary"
          onClick={() => navigate("/sessions/new")}
        >
          新規セッション
        </Button>
        <RefetchButton refetch={refetch} />
      </Stack>
      <TableContainer>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>セッション名</TableCell>
              <TableCell>アクセスレベル</TableCell>
              <TableCell>ユーザー数</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {data?.sessions.map((session) => (
              <TableRow
                key={session.id}
                onClick={() => navigate(`/sessions/${session.id}`)}
                hover
                sx={{ cursor: "pointer" }}
              >
                <TableCell>{session.name}</TableCell>
                <TableCell>
                  {AccessLevels[session.accessLevel - 1].label}
                </TableCell>
                <TableCell>
                  {session.usersCount}/{session.maxUsers}
                </TableCell>
              </TableRow>
            ))}
            {status === "pending" && (
              <>
                <TableRow>
                  <TableCell>
                    <Skeleton variant="rectangular" />
                  </TableCell>
                  <TableCell>
                    <Skeleton variant="rectangular" />
                  </TableCell>
                  <TableCell>
                    <Skeleton variant="rectangular" />
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell>
                    <Skeleton variant="rectangular" />
                  </TableCell>
                  <TableCell>
                    <Skeleton variant="rectangular" />
                  </TableCell>
                  <TableCell>
                    <Skeleton variant="rectangular" />
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell>
                    <Skeleton variant="rectangular" />
                  </TableCell>
                  <TableCell>
                    <Skeleton variant="rectangular" />
                  </TableCell>
                  <TableCell>
                    <Skeleton variant="rectangular" />
                  </TableCell>
                </TableRow>
              </>
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Stack>
  );
}
