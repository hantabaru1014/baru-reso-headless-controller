import { useQuery } from "@connectrpc/connect-query";
import { listHeadlessHost } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  Grid2,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from "@mui/material";
import prettyBytes from "../libs/prettyBytes";

export default function HostList() {
  const { data, isPending } = useQuery(listHeadlessHost);

  return (
    <Grid2 container spacing={2}>
      <TableContainer>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>ID</TableCell>
              <TableCell>名前</TableCell>
              <TableCell>Resoniteバージョン</TableCell>
              <TableCell>fps</TableCell>
              <TableCell>アカウント名</TableCell>
              <TableCell>ストレージ</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {data?.hosts.map((host) => (
              <TableRow key={host.id}>
                <TableCell>{host.id}</TableCell>
                <TableCell>{host.name}</TableCell>
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
  );
}
