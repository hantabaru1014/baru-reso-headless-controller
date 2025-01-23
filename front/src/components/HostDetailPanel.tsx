import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  getHeadlessHost,
  restartHeadlessHost,
  shutdownHeadlessHost,
  updateHeadlessHostSettings,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { Button, Grid2, Stack, Typography } from "@mui/material";
import EditableTextField from "./base/EditableTextField";
import ReadOnlyField from "./base/ReadOnlyField";
import prettyBytes from "../libs/prettyBytes";
import { HeadlessHostStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { hostStatusToLabel } from "../libs/hostUtils";
import { useNavigate } from "react-router";

export default function HostDetailPanel({ hostId }: { hostId: string }) {
  const navigate = useNavigate();
  const { data, isPending, refetch } = useQuery(getHeadlessHost, { hostId });
  const { mutateAsync: shutdownHost } = useMutation(shutdownHeadlessHost);
  const { mutateAsync: updateHost } = useMutation(updateHeadlessHostSettings);
  const { mutateAsync: restartHost } = useMutation(restartHeadlessHost);

  return (
    <Grid2 container spacing={2}>
      <Grid2 size={12} sx={{ justifyContent: "space-between" }}>
        <Stack direction="row" justifyContent="space-between">
          <EditableTextField
            label="Name"
            value={data?.host?.name}
            onSave={async (v) => {
              try {
                await updateHost({ hostId, name: v });
                refetch();
                return { ok: true };
              } catch (e) {
                return {
                  ok: false,
                  error: e instanceof Error ? e.message : `${e}`,
                };
              }
            }}
            isLoading={isPending}
          />
          <Stack direction="row" spacing={2} alignItems="center">
            <Typography>
              ステータス:{" "}
              {data?.host?.status
                ? hostStatusToLabel(data?.host?.status)
                : "不明"}
            </Typography>
            <Button
              variant="contained"
              color="warning"
              onClick={async () => {
                await shutdownHost({ hostId });
                setTimeout(() => {
                  refetch();
                }, 1000);
              }}
              disabled={
                isPending || data?.host?.status !== HeadlessHostStatus.RUNNING
              }
            >
              シャットダウン
            </Button>
            <Button
              variant="contained"
              color="primary"
              onClick={async () => {
                setTimeout(() => {
                  refetch();
                }, 1000);
                const result = await restartHost({ hostId, withUpdate: true });
                if (result.newHostId) {
                  navigate(`/hosts/${result.newHostId}`);
                }
              }}
              disabled={isPending}
            >
              再起動
            </Button>
          </Stack>
        </Stack>
      </Grid2>
      {data?.host?.status === HeadlessHostStatus.RUNNING && (
        <>
          <Grid2 size={6}>
            <ReadOnlyField
              label="アカウント"
              value={`${data?.host?.accountName} (${data?.host?.accountId})`}
              isLoading={isPending}
            />
          </Grid2>
          <Grid2 size={6}>
            <ReadOnlyField
              label="Resoniteバージョン"
              value={data?.host?.resoniteVersion}
              isLoading={isPending}
            />
          </Grid2>
          <Grid2 size={6}>
            <ReadOnlyField
              label="ストレージ"
              value={`${prettyBytes(Number(data?.host?.storageUsedBytes))}/${prettyBytes(Number(data?.host?.storageQuotaBytes))}`}
              isLoading={isPending}
            />
          </Grid2>
          <Grid2 size={6}>
            <ReadOnlyField
              label="fps"
              value={data?.host?.fps?.toString()}
              isLoading={isPending}
            />
          </Grid2>
        </>
      )}
    </Grid2>
  );
}
