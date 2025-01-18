import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  getSessionDetails,
  updateSessionParameters,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useAtom } from "jotai";
import { selectedHostAtom } from "../atoms/selectedHostAtom";
import { Stack } from "@mui/material";
import Loading from "./base/Loading";
import EditableTextField from "./base/EditableTextField";
import EditableSelectField from "./base/EditableSelectField";
import { AccessLevels } from "../constants";

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
      refetch();
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  return (
    <Loading loading={status === "pending"}>
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
    </Loading>
  );
}
