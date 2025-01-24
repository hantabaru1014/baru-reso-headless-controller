import { useQuery } from "@connectrpc/connect-query";
import { listHeadlessHost } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  Grid2,
  Skeleton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from "@mui/material";
import prettyBytes from "../libs/prettyBytes";
import { useNavigate } from "react-router";
import { hostStatusToLabel } from "../libs/hostUtils";
import RefetchButton from "./base/RefetchButton";

export default function HostList() {
  const { data, isPending, refetch } = useQuery(listHeadlessHost);
  const navigate = useNavigate();

  return (
    <Grid2 container>
      <Grid2 size={12}>
        <Stack direction="row" spacing={2} sx={{ justifyContent: "flex-end" }}>
          <RefetchButton refetch={refetch} />
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
