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
        },
      });
      notifications.show("セッションを開始しました", {
        severity: "success",
        autoHideDuration: 3000,
      });
      navigate("/sessions");
    } catch (e) {
      console.error(e);
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
      </Stack>
      <TextField
        label="カスタムセッションID"
        fullWidth
        value={customSessionId}
        onChange={(e) => setCustomSessionId(e.target.value)}
      />

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
