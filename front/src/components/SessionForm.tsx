import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  getSessionDetails,
  updateSessionParameters,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useAtom } from "jotai";
import { selectedHostAtom } from "../atoms/selectedHostAtom";
import { Grid2, IconButton, Stack } from "@mui/material";
import Loading from "./base/Loading";
import EditableTextField from "./base/EditableTextField";
import EditableSelectField from "./base/EditableSelectField";
import { AccessLevels } from "../constants";
import SessionControlButtons from "./SessionControlButtons";
import { RefreshOutlined } from "@mui/icons-material";

export default function SessionForm({ sessionId }: { sessionId: string }) {
  const [selectedHost] = useAtom(selectedHostAtom);
  const { data, status, refetch } = useQuery(getSessionDetails, {
    hostId: selectedHost?.id,
    sessionId,
  });
  const { mutateAsync: mutateSave } = useMutation(updateSessionParameters);

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

  return (
    <Loading loading={status === "pending"}>
      <Grid2 container>
        <Grid2 size={8}>
          <EditableTextField
            label="セッション名"
            value={data?.session?.name || ""}
            onSave={(v) => handleSave("name", v)}
          />
        </Grid2>
        <Grid2 size={4} container sx={{ justifyContent: "flex-end" }}>
          <SessionControlButtons
            sessionId={sessionId}
            canSave={data?.session?.canSave}
            additionalButtons={
              <IconButton aria-label="再読み込み" onClick={() => refetch()}>
                <RefreshOutlined />
              </IconButton>
            }
          />
        </Grid2>
        <Grid2 size={12}>
          <Stack component="form" noValidate autoComplete="off" spacing={2}>
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
    </Loading>
  );
}
