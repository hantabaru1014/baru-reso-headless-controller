import { useMutation } from "@connectrpc/connect-query";
import {
  fetchWorldInfo,
  startWorld,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useAtom } from "jotai";
import { useNotifications } from "@toolpad/core/useNotifications";
import { selectedHostAtom } from "../atoms/selectedHostAtom";
import {
  Button,
  Checkbox,
  FormControl,
  FormControlLabel,
  FormLabel,
  Grid2,
  Radio,
  RadioGroup,
  Stack,
  TextField,
} from "@mui/material";
import { useState } from "react";
import SelectField from "./base/SelectField";
import { useNavigate } from "react-router";
import { AccessLevels } from "../constants";

export default function NewSessionForm() {
  const navigate = useNavigate();
  const notifications = useNotifications();
  const { mutateAsync: mutateStart, isPending: isPendingStart } =
    useMutation(startWorld);
  const { mutateAsync: mutateFetchInfo, isPending: isPendingFetchInfo } =
    useMutation(fetchWorldInfo);

  const [selectedHost] = useAtom(selectedHostAtom);

  const [useWorldUrl, setUseWorldUrl] = useState(true);
  const [worldUrl, setWorldUrl] = useState("");
  const [worldTemplate, setWorldTemplate] = useState("grid");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [maxUsers, setMaxUsers] = useState(15);
  const [accessLevel, setAccessLevel] = useState(1);
  const [customSessionId, setCustomSessionId] = useState("");
  const [hideFromPublicListing, setHideFromPublicListing] = useState(false);
  const [awayKickMinutes, setAwayKickMinutes] = useState(-1);
  const [idleRestartIntervalSeconds, setIdleRestartIntervalSeconds] =
    useState(-1);
  const [saveOnExit, setSaveOnExit] = useState(false);
  const [autoSaveIntervalSeconds, setAutoSaveIntervalSeconds] = useState(-1);
  const [autoSleep, setAutoSleep] = useState(false);

  const handleFetchInfo = async () => {
    try {
      const data = await mutateFetchInfo({
        hostId: selectedHost?.id,
        url: worldUrl,
      });
      setName(data.name);
      setDescription(data.description);
    } catch (e) {
      console.error(e);
    }
  };

  const handleStartSession = async () => {
    try {
      await mutateStart({
        hostId: selectedHost?.id,
        parameters: {
          loadWorld: useWorldUrl
            ? { case: "loadWorldUrl", value: worldUrl }
            : { case: "loadWorldPresetName", value: worldTemplate },
          name,
          description,
          maxUsers,
          accessLevel,
          customSessionId,
          hideFromPublicListing,
          awayKickMinutes,
          idleRestartIntervalSeconds,
          saveOnExit,
          autoSaveIntervalSeconds,
          autoSleep,
        },
      });
      notifications.show("セッションを開始しました", {
        severity: "success",
        autoHideDuration: 3000,
      });
      navigate("/sessions");
    } catch (e) {
      notifications.show(`エラー: ${e instanceof Error ? e.message : e}`, {
        severity: "error",
        autoHideDuration: 3000,
      });
    }
  };

  return (
    <Stack component="form" noValidate autoComplete="off" spacing={2}>
      <FormControl>
        <FormLabel id="session-form-use-world-url">ワールド指定方法</FormLabel>
        <RadioGroup
          aria-labelledby="session-form-use-world-url"
          row
          value={useWorldUrl ? "url" : "template"}
          onChange={(e) => setUseWorldUrl(e.target.value === "url")}
        >
          <FormControlLabel
            value={"url"}
            control={<Radio />}
            label="レコードURLを指定"
          />
          <FormControlLabel
            value={"template"}
            control={<Radio />}
            label="テンプレートを指定"
          />
        </RadioGroup>
      </FormControl>

      {useWorldUrl ? (
        <Grid2 container spacing={2}>
          <Grid2 size={10}>
            <TextField
              label="レコードURL"
              fullWidth
              value={worldUrl}
              onChange={(e) => setWorldUrl(e.target.value)}
            />
          </Grid2>
          <Grid2 size={2} sx={{ alignItems: "center" }} container>
            <Button
              variant="outlined"
              size="large"
              onClick={handleFetchInfo}
              loading={isPendingFetchInfo}
            >
              情報取得
            </Button>
          </Grid2>
        </Grid2>
      ) : (
        <SelectField
          label="ワールドテンプレート"
          options={[
            { id: "grid", label: "Grid" },
            { id: "platform", label: "Platform" },
            { id: "blank", label: "Blank" },
          ]}
          selectedId={worldTemplate}
          onChange={(option) => setWorldTemplate(option.id)}
        />
      )}

      <TextField
        label="セッション名"
        fullWidth
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      <TextField
        label="説明"
        multiline
        fullWidth
        value={description}
        onChange={(e) => setDescription(e.target.value)}
      />
      <Stack direction="row" spacing={2}>
        <TextField
          label="最大ユーザー数"
          type="number"
          value={maxUsers}
          onChange={(e) => setMaxUsers(parseInt(e.target.value))}
        />
        <SelectField
          label="アクセスレベル"
          options={AccessLevels.map((l) => l)}
          selectedId={`${accessLevel}`}
          onChange={(option) => setAccessLevel(option.value as number)}
        />
        <FormControlLabel
          label="セッションリストから隠す"
          control={
            <Checkbox
              checked={hideFromPublicListing}
              onChange={(e) => setHideFromPublicListing(e.target.checked)}
            />
          }
        />
      </Stack>
      <TextField
        label="カスタムセッションID"
        fullWidth
        value={customSessionId}
        onChange={(e) => setCustomSessionId(e.target.value)}
      />
      <TextField
        label="AFKキック時間(分)"
        type="number"
        value={awayKickMinutes}
        onChange={(e) => setAwayKickMinutes(parseFloat(e.target.value))}
        helperText="-1で無効"
      />
      <Stack direction="row" spacing={2}>
        <TextField
          label="自動保存間隔(秒)"
          type="number"
          value={autoSaveIntervalSeconds}
          onChange={(e) => setAutoSaveIntervalSeconds(parseInt(e.target.value))}
          helperText="-1で無効"
        />
        <FormControlLabel
          label="セッション終了時に保存"
          control={
            <Checkbox
              checked={saveOnExit}
              onChange={(e) => setSaveOnExit(e.target.checked)}
            />
          }
        />
      </Stack>
      <Stack direction="row" spacing={2}>
        <TextField
          label="アイドル時の自動再起動間隔(秒)"
          type="number"
          value={idleRestartIntervalSeconds}
          onChange={(e) =>
            setIdleRestartIntervalSeconds(parseInt(e.target.value))
          }
          helperText="-1で無効"
        />
        <FormControlLabel
          label="自動スリープ"
          control={
            <Checkbox
              checked={autoSleep}
              onChange={(e) => setAutoSleep(e.target.checked)}
            />
          }
        />
      </Stack>

      <Button
        variant="contained"
        onClick={handleStartSession}
        loading={isPendingStart}
      >
        セッション開始
      </Button>
    </Stack>
  );
}
