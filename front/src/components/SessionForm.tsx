import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  getSessionDetails,
  updateSessionParameters,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useAtom } from "jotai";
import { selectedHostAtom } from "../atoms/selectedHostAtom";
import { Button, Card, CardContent, Grid2, Stack } from "@mui/material";
import Loading from "./base/Loading";
import EditableTextField from "./base/EditableTextField";
import EditableSelectField from "./base/EditableSelectField";
import { AccessLevels } from "../constants";
import SessionControlButtons from "./SessionControlButtons";
import { ImageNotSupportedOutlined } from "@mui/icons-material";
import RefetchButton from "./base/RefetchButton";
import EditableCheckBox from "./base/EditableCheckBox";
import { useNotifications } from "@toolpad/core/useNotifications";

export default function SessionForm({ sessionId }: { sessionId: string }) {
  const [selectedHost] = useAtom(selectedHostAtom);
  const { data, status, refetch } = useQuery(getSessionDetails, {
    hostId: selectedHost?.id,
    sessionId,
  });
  const { mutateAsync: mutateSave } = useMutation(updateSessionParameters);
  const notifications = useNotifications();

  const handleSave = async <V,>(fieldName: string, value: V) => {
    try {
      await mutateSave({
        hostId: selectedHost?.id,
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

  const handleCopyUrl = () => {
    const url = data?.session?.sessionUrl;
    if (!url) {
      return;
    }
    navigator.clipboard.writeText(url);
    notifications.show("セッションURLをコピーしました", {
      severity: "info",
      autoHideDuration: 3000,
    });
  };

  return (
    <Loading loading={status === "pending"}>
      <Grid2 container spacing={2}>
        <Grid2 size={5}>
          <Card sx={{ height: "100%" }}>
            <CardContent sx={{ height: "100%" }}>
              {data?.session?.thumbnailUrl ? (
                <img
                  src={data?.session?.thumbnailUrl}
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
            <SessionControlButtons
              sessionId={sessionId}
              canSave={data?.session?.canSave}
              additionalButtons={
                <>
                  <Button variant="contained" onClick={handleCopyUrl}>
                    URLをコピー
                  </Button>
                  <RefetchButton refetch={refetch} />
                </>
              }
            />
          </Grid2>
          <Grid2 size={12}>
            <Stack component="form" noValidate autoComplete="off" spacing={2}>
              <EditableTextField
                label="セッション名"
                value={data?.session?.name || ""}
                onSave={(v) => handleSave("name", v)}
              />
              <EditableTextField
                label="説明"
                multiline
                value={data?.session?.description || ""}
                onSave={(v) => handleSave("description", v)}
              />
              <Stack direction="row" spacing={2}>
                <EditableTextField
                  label="最大ユーザー数"
                  type="number"
                  value={data?.session?.maxUsers?.toString() || "0"}
                  onSave={(v) => handleSave("maxUsers", parseInt(v))}
                />
                <EditableSelectField
                  label="アクセスレベル"
                  options={AccessLevels.map((l) => l)}
                  selectedId={`${data?.session?.accessLevel}` || "1"}
                  onSave={(v) => handleSave("accessLevel", v)}
                />
              </Stack>
            </Stack>
          </Grid2>
        </Grid2>
        <Grid2 size={12}>
          <Stack spacing={2}>
            <Stack direction="row" spacing={2}>
              <EditableTextField
                label="AFKキック時間(分)"
                type="number"
                value={data?.session?.awayKickMinutes}
                onSave={(v) => handleSave("awayKickMinutes", parseFloat(v))}
                helperText="-1で無効"
              />
              <EditableCheckBox
                label="セッションリストから隠す"
                checked={data?.session?.hideFromPublicListing}
                onSave={(v) => handleSave("hideFromPublicListing", v)}
              />
            </Stack>
            <Stack direction="row" spacing={2}>
              <EditableCheckBox
                label="セッション終了時に保存"
                checked={data?.session?.saveOnExit || false}
                onSave={(v) => handleSave("saveOnExit", v)}
              />
              <EditableTextField
                label="自動保存間隔(秒)"
                type="number"
                value={data?.session?.autoSaveIntervalSeconds}
                onSave={(v) =>
                  handleSave("autoSaveIntervalSeconds", parseInt(v))
                }
                helperText="-1で無効"
              />
            </Stack>
            <Stack direction="row" spacing={2}>
              {/* FIXME: 反応しないのでヘッドレス側を修正するまで一旦コメントアウト */}
              {/* <EditableCheckBox
                label="オートスリープ"
                checked={data?.session?.autoSleep}
                onSave={(v) => handleSave("autoSleep", v)}
              /> */}
              <EditableTextField
                label="アイドル時の自動再起動間隔(秒)"
                type="number"
                value={data?.session?.idleRestartIntervalSeconds}
                onSave={(v) =>
                  handleSave("idleRestartIntervalSeconds", parseInt(v))
                }
                helperText="-1で無効"
              />
            </Stack>
          </Stack>
        </Grid2>
      </Grid2>
    </Loading>
  );
}
