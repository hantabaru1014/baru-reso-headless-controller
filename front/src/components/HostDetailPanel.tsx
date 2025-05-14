import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  allowHostAccess,
  denyHostAccess,
  getHeadlessHost,
  restartHeadlessHost,
  shutdownHeadlessHost,
  updateHeadlessHostSettings,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Grid2,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import EditableTextField from "./base/EditableTextField";
import ReadOnlyField from "./base/ReadOnlyField";
import prettyBytes from "../libs/prettyBytes";
import { HeadlessHostStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { hostStatusToLabel } from "../libs/hostUtils";
import { useNavigate } from "react-router";
import FriendRequestList from "./FriendRequestList";
import { useNotifications } from "@toolpad/core/useNotifications";
import { DialogProps, useDialogs } from "@toolpad/core/useDialogs";
import { AllowedAccessEntry_AccessType } from "../../pbgen/headless/v1/headless_pb";
import { useState } from "react";
import ScrollBase from "./base/ScrollBase";
import SelectField from "./base/SelectField";

type AllowedAccessEntryType = {
  host: string;
  port: number;
  accessType: AllowedAccessEntry_AccessType;
};

function AllowedUrlHostsDialog({
  open,
  onClose,
  payload: { hostId, hosts: initHosts },
}: DialogProps<{ hostId: string; hosts: AllowedAccessEntryType[] }>) {
  const notifications = useNotifications();

  const { mutateAsync: allow } = useMutation(allowHostAccess);
  const { mutateAsync: deny } = useMutation(denyHostAccess);

  const [hosts, setHosts] = useState<AllowedAccessEntryType[]>(initHosts);
  const [newUrl, setNewUrl] = useState("");
  const [newPort, setNewPort] = useState("80");
  const [newAccessType, setNewAccessType] = useState(
    AllowedAccessEntry_AccessType.HTTP,
  );

  return (
    <Dialog open={open} onClose={() => onClose()} fullWidth maxWidth="sm">
      <DialogTitle>Allowed Url Hosts</DialogTitle>
      <DialogContent dividers>
        <Stack direction="column" spacing={2}>
          <Stack direction="row" spacing={1}>
            <TextField
              label="Host"
              value={newUrl}
              onChange={(e) => setNewUrl(e.target.value)}
              fullWidth
            />
            <TextField
              label="Port"
              type="number"
              value={newPort}
              onChange={(e) => setNewPort(e.target.value)}
            />
            <span style={{ width: "10rem" }}>
              <SelectField
                label="Access Type"
                options={[
                  {
                    id: AllowedAccessEntry_AccessType[
                      AllowedAccessEntry_AccessType.HTTP
                    ],
                    label: "HTTP",
                    value: AllowedAccessEntry_AccessType.HTTP,
                  },
                  {
                    id: AllowedAccessEntry_AccessType[
                      AllowedAccessEntry_AccessType.WEBSOCKET
                    ],
                    label: "WEBSOCKET",
                    value: AllowedAccessEntry_AccessType.WEBSOCKET,
                  },
                  {
                    id: AllowedAccessEntry_AccessType[
                      AllowedAccessEntry_AccessType.OSC_RECEIVING
                    ],
                    label: "OSC_RECEIVING",
                    value: AllowedAccessEntry_AccessType.OSC_RECEIVING,
                  },
                  {
                    id: AllowedAccessEntry_AccessType[
                      AllowedAccessEntry_AccessType.OSC_SENDING
                    ],
                    label: "OSC_SENDING",
                    value: AllowedAccessEntry_AccessType.OSC_SENDING,
                  },
                ]}
                selectedId={AllowedAccessEntry_AccessType[newAccessType]}
                onChange={(v) =>
                  setNewAccessType(
                    v.value ?? AllowedAccessEntry_AccessType.HTTP,
                  )
                }
              />
            </span>
            <Button
              variant="contained"
              onClick={async () => {
                try {
                  await allow({
                    hostId,
                    request: {
                      host: newUrl,
                      port: parseInt(newPort),
                      accessType: newAccessType,
                    },
                  });
                  setNewUrl("");
                  setNewPort("80");
                  setHosts((prev) => [
                    ...prev,
                    {
                      host: newUrl,
                      port: parseInt(newPort),
                      accessType: newAccessType,
                    },
                  ]);
                } catch (e) {
                  notifications.show(e instanceof Error ? e.message : `${e}`, {
                    severity: "error",
                  });
                }
              }}
              disabled={!newUrl}
            >
              Add
            </Button>
          </Stack>
          <ScrollBase height="60vh">
            <Stack direction="column" spacing={1}>
              {hosts.map((host) => (
                <Stack
                  key={host.host}
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  justifyContent="space-between"
                >
                  {`${host.host}:${host.port} (${AllowedAccessEntry_AccessType[host.accessType]})`}
                  <Button
                    variant="contained"
                    color="error"
                    onClick={async () => {
                      try {
                        await deny({ hostId, request: host });
                        setHosts((prev) =>
                          prev.filter(
                            (h) =>
                              h.host !== host.host ||
                              h.port !== host.port ||
                              h.accessType !== host.accessType,
                          ),
                        );
                      } catch (e) {
                        notifications.show(
                          e instanceof Error ? e.message : `${e}`,
                          {
                            severity: "error",
                          },
                        );
                      }
                    }}
                  >
                    Remove
                  </Button>
                </Stack>
              ))}
            </Stack>
          </ScrollBase>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => onClose()}>閉じる</Button>
      </DialogActions>
    </Dialog>
  );
}

function AutoSpawnItemsDialog({
  open,
  onClose,
  payload: { hostId, items: initItems },
}: DialogProps<{ hostId: string; items: string[] }>) {
  const notifications = useNotifications();
  const { mutateAsync: updateHost } = useMutation(updateHeadlessHostSettings);

  const [items, setItems] = useState<string[]>(initItems);
  const [newItemUri, setNewItemUri] = useState("");

  return (
    <Dialog open={open} onClose={() => onClose()} fullWidth maxWidth="sm">
      <DialogTitle>Auto Spawn Items</DialogTitle>
      <DialogContent dividers>
        <Stack direction="column" spacing={2}>
          <Stack direction="row" spacing={1}>
            <TextField
              label="Item URI"
              value={newItemUri}
              onChange={(e) => setNewItemUri(e.target.value)}
              fullWidth
            />
            <Button
              variant="contained"
              onClick={async () => {
                try {
                  await updateHost({
                    hostId,
                    autoSpawnItems: [...items, newItemUri],
                  });
                  setNewItemUri("");
                  setItems((prev) => [...prev, newItemUri]);
                } catch (e) {
                  notifications.show(e instanceof Error ? e.message : `${e}`, {
                    severity: "error",
                  });
                }
              }}
              disabled={!newItemUri}
            >
              Add
            </Button>
          </Stack>
          <ScrollBase height="60vh">
            <Stack direction="column" spacing={1}>
              {items.map((item) => (
                <Stack
                  key={item}
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  justifyContent="space-between"
                >
                  <Typography>{item}</Typography>
                  <Button
                    variant="contained"
                    color="error"
                    onClick={async () => {
                      try {
                        await updateHost({
                          hostId,
                          autoSpawnItems: items.filter((i) => i !== item),
                        });
                        setItems((prev) => prev.filter((i) => i !== item));
                      } catch (e) {
                        notifications.show(
                          e instanceof Error ? e.message : `${e}`,
                          {
                            severity: "error",
                          },
                        );
                      }
                    }}
                  >
                    Remove
                  </Button>
                </Stack>
              ))}
            </Stack>
          </ScrollBase>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => onClose()}>閉じる</Button>
      </DialogActions>
    </Dialog>
  );
}

export default function HostDetailPanel({ hostId }: { hostId: string }) {
  const navigate = useNavigate();
  const { data, isPending, refetch } = useQuery(getHeadlessHost, { hostId });
  const { mutateAsync: shutdownHost, isPending: isPendingShutdown } =
    useMutation(shutdownHeadlessHost);
  const { mutateAsync: updateHost } = useMutation(updateHeadlessHostSettings);
  const { mutateAsync: restartHost, isPending: isPendingRestart } =
    useMutation(restartHeadlessHost);
  const notifications = useNotifications();
  const dialogs = useDialogs();

  const settings = data?.settings;

  const handleRestart = async () => {
    try {
      const result = await restartHost({
        hostId,
        withUpdate: true,
        withWorldRestart: true,
      });
      setTimeout(() => {
        refetch();
      }, 1000);
      if (result.newHostId) {
        navigate(`/hosts/${result.newHostId}`);
      }
    } catch (e) {
      notifications.show(e instanceof Error ? e.message : `${e}`, {
        severity: "error",
      });
    }
  };

  const handleShutdown = async () => {
    try {
      await shutdownHost({ hostId });
      setTimeout(() => {
        refetch();
      }, 1000);
    } catch (e) {
      notifications.show(e instanceof Error ? e.message : `${e}`, {
        severity: "error",
      });
    }
  };

  const handleSave = async <V,>(fieldName: string, value: V) => {
    try {
      await updateHost({ hostId, [fieldName]: value });
      // すぐには反映されない項目もあるので、ちょっと待ってから再取得する
      setTimeout(() => refetch(), 500);
      return { ok: true };
    } catch (e) {
      return {
        ok: false,
        error: e instanceof Error ? e.message : `${e}`,
      };
    }
  };

  return (
    <Grid2 container spacing={2}>
      <Grid2 size={12} sx={{ justifyContent: "space-between" }}>
        <Stack direction="row" justifyContent="space-between">
          <EditableTextField
            label="Name"
            value={data?.host?.name}
            onSave={(v) => handleSave("name", v)}
            isLoading={isPending}
            fullWidth
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
              onClick={handleShutdown}
              disabled={
                isPending ||
                isPendingShutdown ||
                data?.host?.status !== HeadlessHostStatus.RUNNING
              }
              loading={isPendingShutdown}
            >
              シャットダウン
            </Button>
            <Button
              variant="contained"
              color="primary"
              onClick={handleRestart}
              disabled={isPending || isPendingRestart}
              loading={isPendingRestart}
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
              label="FPS (Current)"
              value={data?.host?.fps?.toString()}
              isLoading={isPending}
            />
          </Grid2>
          <Grid2 size={6}>
            <FriendRequestList hostId={hostId} />
          </Grid2>
          {settings && (
            <Grid2 size={6}>
              <Stack spacing={2} direction="column">
                <ReadOnlyField
                  label="Universe ID"
                  value={settings.universeId}
                  isLoading={isPending}
                />
                <EditableTextField
                  label="Username override"
                  value={settings.usernameOverride}
                  onSave={(v) => handleSave("usernameOverride", v)}
                  isLoading={isPending}
                />
                <EditableTextField
                  label="Tick Rate (Target FPS)"
                  type="number"
                  value={settings.tickRate}
                  onSave={(v) => handleSave("tickRate", parseFloat(v))}
                  isLoading={isPending}
                />
                <EditableTextField
                  label="Max concurrent asset transfers"
                  type="number"
                  value={settings.maxConcurrentAssetTransfers}
                  onSave={(v) =>
                    handleSave("maxConcurrentAssetTransfers", parseInt(v))
                  }
                  isLoading={isPending}
                />
                <Button
                  variant="outlined"
                  onClick={async () => {
                    await dialogs.open(AllowedUrlHostsDialog, {
                      hostId,
                      hosts: settings.allowedUrlHosts
                        .flatMap((e) =>
                          e.ports.map((p) => ({
                            host: e.host,
                            port: p,
                            accessTypes: e.accessTypes,
                          })),
                        )
                        .flatMap((e) =>
                          e.accessTypes
                            .filter(
                              (a) =>
                                a !== AllowedAccessEntry_AccessType.UNSPECIFIED, // TODO
                            )
                            .map((a) => ({
                              host: e.host,
                              port: e.port,
                              accessType: a,
                            })),
                        ),
                    });
                    refetch();
                  }}
                >
                  Allowed Url Hosts
                </Button>
                <Button
                  variant="outlined"
                  onClick={async () => {
                    await dialogs.open(AutoSpawnItemsDialog, {
                      hostId,
                      items: settings.autoSpawnItems,
                    });
                    refetch();
                  }}
                >
                  Auto Spawn Items
                </Button>
              </Stack>
            </Grid2>
          )}
        </>
      )}
    </Grid2>
  );
}
