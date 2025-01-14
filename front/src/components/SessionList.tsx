import {
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
import Loading from "./Loading";
import { useNavigate } from "react-router";

const accessLevels = [
  "",
  "Private",
  "LAN",
  "Contacts",
  "Contacts Plus",
  "Registered User",
  "Anyone",
] as const;

export default function SessionList() {
  const [selectedHost] = useAtom(selectedHostAtom);
  const { data, status } = useQuery(listSessions, { hostId: selectedHost?.id });
  const navigate = useNavigate();

  return (
    <Loading loading={status === "pending"}>
      <TableContainer>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>AccessLevel</TableCell>
              <TableCell>User Count</TableCell>
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
                <TableCell>{accessLevels[session.accessLevel]}</TableCell>
                <TableCell>?/{session.startupParameters?.maxUsers}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Loading>
  );
}
