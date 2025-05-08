import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  deleteEndedSession,
  getSessionDetails,
  listHeadlessHost,
  startWorld,
  updateSessionExtraSettings,
  updateSessionParameters,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  Button,
  Card,
  CardContent,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Grid2,
  Stack,
} from "@mui/material";
import Loading from "./base/Loading";
import EditableTextField from "./base/EditableTextField";
import EditableSelectField from "./base/EditableSelectField";
import { AccessLevels } from "../constants";
import SessionControlButtons from "./SessionControlButtons";
import { ImageNotSupportedOutlined } from "@mui/icons-material";
import RefetchButton from "./base/RefetchButton";
import EditableCheckBox from "./base/EditableCheckBox";
import { useNotifications } from "@toolpad/core/useNotifications";
import {
  HeadlessHostStatus,
  SessionStatus,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import { DialogProps, useDialogs } from "@toolpad/core/useDialogs";
import { useState } from "react";
import SelectField from "./base/SelectField";
import { useNavigate } from "react-router";
import { formatTimestamp } from "../libs/datetimeUtils";

function SelectHostDialog({
  open,
  onClose,
  payload: { title, primaryButtonLabel },
}: DialogProps<{ title: string; primaryButtonLabel: string }, string | null>) {
  const { data: hostList } = useQuery(listHeadlessHost);
  const [selectedHostId, setSelectedHostId] = useState("");

  return (
    <Dialog open={open} onClose={() => onClose(null)} fullWidth maxWidth="sm">
      <DialogTitle>{title}</DialogTitle>
      <DialogContent dividers>
        <SelectField
          label="Host"
          options={
            hostList?.hosts
              .filter((host) => host.status === HeadlessHostStatus.RUNNING)
              .map((host) => ({
                id: host.id,
                label: `${host.name} (${host.id.slice(0, 6)})`,
                value: host,
              })) ?? []
          }
          selectedId={selectedHostId || ""}
          onChange={(option) => setSelectedHostId(option.value?.id ?? "")}
          minWidth="7rem"
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={() => onClose(selectedHostId)}>
          {primaryButtonLabel}
        </Button>
        <Button onClick={() => onClose(null)}>キャンセル</Button>
      </DialogActions>
    </Dialog>
  );
}

export default function SessionForm({ sessionId }: { sessionId: string }) {
  const { data, status, refetch } = useQuery(getSessionDetails, {
    sessionId,
  });
  const { mutateAsync: mutateSave } = useMutation(updateSessionParameters);
  const { mutateAsync: mutateSaveExtra } = useMutation(
    updateSessionExtraSettings,
  );
  const { mutateAsync: mutateStartWorld, isPending: isPendingStartWorld } =
    useMutation(startWorld);
  const { mutateAsync: mutateDelete } = useMutation(deleteEndedSession);
  const notifications = useNotifications();
  const dialogs = useDialogs();
  const navigate = useNavigate();

  const hostId = data?.session?.hostId;
  const isRunning = data?.session?.status === SessionStatus.RUNNING;
  const sessionState = data?.session?.currentState;
  const startupParams = data?.session?.startupParameters;

  const handleSave = async <V,>(fieldName: string, value: V) => {
    try {
      await mutateSave({
        hostId,
        parameters: {
          sessionId,
          [fieldName]: value,
        },
      });
      // すぐには反映されない項目もあるので、ちょっと待ってから再取得する
      setTimeout(() => refetch(), 500);
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  const handleSaveExtra = async <V,>(fieldName: string, value: V) => {
    try {
      await mutateSaveExtra({
        sessionId,
        [fieldName]: value,
      });
      refetch();
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  const handleCopyUrl = () => {
    const url = sessionState?.sessionUrl;
    if (!url) {
      return;
    }
    navigator.clipboard.writeText(url);
    notifications.show("セッションURLをコピーしました", {
      severity: "info",
      autoHideDuration: 3000,
    });
  };

  const handleCopyWorldUrl = () => {
    const url = sessionState?.worldUrl;
    if (!url) {
      return;
    }
    navigator.clipboard.writeText(url);
    notifications.show("ワールドURLをコピーしました", {
      severity: "info",
      autoHideDuration: 3000,
    });
  };

  const handleRestartSession = async () => {
    const selectedHostId = await dialogs.open(SelectHostDialog, {
      title: "セッションを開始するホストを選択",
      primaryButtonLabel: "開始",
    });
    if (!selectedHostId) {
      return;
    }
    try {
      const result = await mutateStartWorld({
        hostId: selectedHostId,
        parameters: startupParams,
      });
      if (result.openedSession) {
        notifications.show("セッションを開始しました", {
          severity: "success",
          autoHideDuration: 3000,
        });
        navigate(`/sessions/${result.openedSession.id}`);
      }
    } catch (e) {
      notifications.show(`セッションの開始に失敗しました: ${e}`, {
        severity: "error",
        autoHideDuration: 3000,
      });
    }
  };

  const handleDeleteSession = async () => {
    try {
      await mutateDelete({ sessionId });
      notifications.show("セッションを削除しました", {
        severity: "success",
        autoHideDuration: 3000,
      });
      navigate("/sessions");
    } catch (e) {
      notifications.show(`セッションの削除に失敗しました: ${e}`, {
        severity: "error",
        autoHideDuration: 3000,
      });
    }
  };

  return (
    <Loading loading={status === "pending"}>
      <Grid2 container spacing={2}>
        <Grid2 size={5}>
          <Card sx={{ height: "100%" }}>
            <CardContent sx={{ height: "100%" }}>
              {sessionState?.thumbnailUrl ? (
                <img
                  src={sessionState?.thumbnailUrl}
                  alt="セッションサムネイル"
                  style={{
                    width: "100%",
                    height: "auto",
                  }}
                />
              ) : (
                <div style={{ height: "100%" }}>
                  <ImageNotSupportedOutlined
                    sx={{
                      position: "relative",
                      top: "calc(50% - 0.5em)",
                      left: "calc(50% - 0.5em)",
                    }}
                  />
                </div>
              )}
            </CardContent>
          </Card>
        </Grid2>
        <Grid2 size={7} container sx={{ justifyContent: "flex-start" }}>
          <Grid2 size={12} container sx={{ justifyContent: "flex-end" }}>
            {isRunning ? (
              <SessionControlButtons
                hostId={hostId ?? ""}
                sessionId={sessionId}
                canSave={sessionState?.canSave}
                additionalButtons={
                  <>
                    <Button variant="contained" onClick={handleCopyUrl}>
                      URLをコピー
                    </Button>
                    {sessionState?.worldUrl && (
                      <Button variant="contained" onClick={handleCopyWorldUrl}>
                        ワールドURLをコピー
                      </Button>
                    )}
                    <RefetchButton refetch={refetch} />
                  </>
                }
              />
            ) : (
              <Stack direction="row" spacing={2}>
                <Button
                  variant="contained"
                  loading={isPendingStartWorld}
                  onClick={handleRestartSession}
                >
                  同設定で開始
                </Button>
                <Button
                  variant="contained"
                  color="warning"
                  onClick={handleDeleteSession}
                >
                  削除
                </Button>
              </Stack>
            )}
          </Grid2>
          <Grid2 size={12}>
            <Stack component="form" noValidate autoComplete="off" spacing={2}>
              <EditableTextField
                label="セッション名"
                value={sessionState?.name || startupParams?.name || ""}
                onSave={(v) => handleSave("name", v)}
                readonly={!isRunning}
              />
              <EditableTextField
                label="説明"
                multiline
                value={
                  sessionState?.description || startupParams?.description || ""
                }
                onSave={(v) => handleSave("description", v)}
                readonly={!isRunning}
              />
              <Stack direction="row" spacing={2}>
                <EditableTextField
                  label="最大ユーザー数"
                  type="number"
                  value={
                    sessionState?.maxUsers?.toString() ||
                    startupParams?.maxUsers?.toString() ||
                    "0"
                  }
                  onSave={(v) => handleSave("maxUsers", parseInt(v))}
                  readonly={!isRunning}
                />
                <EditableSelectField
                  label="アクセスレベル"
                  options={AccessLevels.map((l) => l)}
                  selectedId={
                    `${sessionState?.accessLevel || startupParams?.accessLevel}` ||
                    "1"
                  }
                  onSave={(v) => handleSave("accessLevel", v)}
                  readOnly={!isRunning}
                />
              </Stack>
            </Stack>
          </Grid2>
        </Grid2>
        <Grid2 size={5}>
          <Stack spacing={2}>
            <Stack direction="column">
              <span>開始: {formatTimestamp(data?.session?.startedAt)}</span>
              {data?.session?.ownerId && (
                <span>オーナー: {data?.session?.ownerId}</span>
              )}
              {data?.session?.endedAt && (
                <span>終了: {formatTimestamp(data?.session?.endedAt)}</span>
              )}
              {sessionState?.lastSavedAt && sessionState.canSave && (
                <span>
                  最終保存: {formatTimestamp(sessionState.lastSavedAt)}
                </span>
              )}
            </Stack>
            {/* <EditableCheckBox
              label="自動アップデート"
              checked={data?.session?.autoUpgrade || false}
              onSave={(v) => handleSaveExtra("autoUpgrade", v)}
              helperText="新しいバージョンが出た場合にユーザがいなければ自動で新しいバージョンのホストに移行します"
            /> */}
            <EditableTextField
              label="管理者メモ"
              multiline
              minRows={3}
              value={data?.session?.memo || ""}
              onSave={(v) => handleSaveExtra("memo", v)}
            />
          </Stack>
        </Grid2>
        <Grid2 size={7}>
          <Stack spacing={2}>
            <Stack direction="row" spacing={2}>
              <EditableTextField
                label="AFKキック時間(分)"
                type="number"
                value={
                  sessionState?.awayKickMinutes ||
                  startupParams?.awayKickMinutes ||
                  -1
                }
                onSave={(v) => handleSave("awayKickMinutes", parseFloat(v))}
                helperText="-1で無効"
                readonly={!isRunning}
              />
              <EditableCheckBox
                label="セッションリストから隠す"
                checked={
                  sessionState?.hideFromPublicListing ||
                  startupParams?.hideFromPublicListing ||
                  false
                }
                onSave={(v) => handleSave("hideFromPublicListing", v)}
                readonly={!isRunning}
              />
            </Stack>
            <Stack direction="row" spacing={2}>
              <EditableCheckBox
                label="セッション終了時に保存"
                checked={
                  sessionState?.saveOnExit || startupParams?.saveOnExit || false
                }
                onSave={(v) => handleSave("saveOnExit", v)}
                readonly={!isRunning}
              />
              <EditableTextField
                label="自動保存間隔(秒)"
                type="number"
                value={
                  sessionState?.autoSaveIntervalSeconds ||
                  startupParams?.autoSaveIntervalSeconds ||
                  -1
                }
                onSave={(v) =>
                  handleSave("autoSaveIntervalSeconds", parseInt(v))
                }
                helperText="-1で無効"
                readonly={!isRunning}
              />
            </Stack>
            <Stack direction="row" spacing={2}>
              {/* FIXME: 反応しないのでヘッドレス側を修正するまで一旦コメントアウト */}
              {/* <EditableCheckBox
                label="オートスリープ"
                checked={sessionState?.autoSleep || startupParams?.autoSleep || false}
                onSave={(v) => handleSave("autoSleep", v)}
                readonly={!isRunning}
              /> */}
              <EditableTextField
                label="アイドル時の自動再起動間隔(秒)"
                type="number"
                value={
                  sessionState?.idleRestartIntervalSeconds ||
                  startupParams?.idleRestartIntervalSeconds ||
                  -1
                }
                onSave={(v) =>
                  handleSave("idleRestartIntervalSeconds", parseInt(v))
                }
                helperText="-1で無効"
                readonly={!isRunning}
              />
            </Stack>
          </Stack>
        </Grid2>
      </Grid2>
    </Loading>
  );
}
