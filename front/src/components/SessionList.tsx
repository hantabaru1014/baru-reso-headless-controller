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
import {
  listHeadlessHost,
  searchSessions,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useNavigate } from "react-router";
import { AccessLevels } from "../constants";
import RefetchButton from "./base/RefetchButton";
import { sessionStatusToLabel } from "../libs/sessionUtils";

export default function SessionList() {
  const { data: hosts } = useQuery(listHeadlessHost);
  const { data, status, refetch } = useQuery(searchSessions, {
    parameters: {},
  });
  const navigate = useNavigate();

  const hostNameMap =
    hosts?.hosts.reduce(
      (acc, host) => {
        acc[host.id] = host.name;
        return acc;
      },
      {} as Record<string, string>,
    ) || {};

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
              <TableCell>ホスト名</TableCell>
              <TableCell>状態</TableCell>
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
                <TableCell>{hostNameMap[session.hostId]}</TableCell>
                <TableCell>{sessionStatusToLabel(session.status)}</TableCell>
                <TableCell>
                  {session.currentState
                    ? AccessLevels[session.currentState.accessLevel - 1].label
                    : ""}
                </TableCell>
                <TableCell>
                  {session.currentState
                    ? session.currentState.usersCount +
                      "/" +
                      session.currentState.maxUsers
                    : ""}
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
