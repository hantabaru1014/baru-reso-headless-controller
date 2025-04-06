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
import SelectField from "./base/SelectField";
import { ReactNode, useState } from "react";
import { SessionStatus } from "../../pbgen/hdlctrl/v1/controller_pb";

export default function SessionList() {
  const [filterState, setFilterState] = useState<{
    label: ReactNode;
    id: string;
    value: SessionStatus | undefined;
  }>({
    label: "全て",
    id: "ALL",
    value: undefined,
  });
  const [filterHostId, setFilterHostId] = useState("ALL");
  const { data: hosts } = useQuery(listHeadlessHost);
  const { data, status, refetch } = useQuery(searchSessions, {
    parameters: {
      status: filterState.value,
      hostId: filterHostId === "ALL" ? undefined : filterHostId,
    },
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
      <Stack
        direction="row"
        spacing={2}
        sx={{ justifyContent: "space-between" }}
      >
        <Stack direction="row" spacing={1}>
          <SelectField
            label="状態"
            options={[
              { label: "全て", id: "ALL", value: undefined },
              { label: "実行中", id: "RUNNING", value: SessionStatus.RUNNING },
              { label: "終了済み", id: "ENDED", value: SessionStatus.ENDED },
            ]}
            onChange={(o) =>
              setFilterState({
                label: o.label,
                id: o.id,
                value: o.value,
              })
            }
            selectedId={filterState.id}
          />
          <SelectField
            label="ホスト"
            options={[{ label: "全て", id: "ALL" }].concat(
              hosts?.hosts.map((host) => ({
                label: host.name,
                id: host.id,
              })) || [],
            )}
            onChange={(o) => setFilterHostId(o.id)}
            selectedId={filterHostId}
          />
        </Stack>
        <div>
          <Button
            variant="contained"
            color="primary"
            onClick={() => navigate("/sessions/new")}
          >
            新規セッション
          </Button>
          <RefetchButton refetch={refetch} />
        </div>
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
