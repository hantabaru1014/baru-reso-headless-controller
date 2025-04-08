import { Stack, Button } from "@mui/material";
import { useNotifications } from "@toolpad/core/useNotifications";
import { useNavigate } from "react-router";
import {
  saveSessionWorld,
  stopSession,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useMutation } from "@connectrpc/connect-query";

export default function SessionControlButtons({
  hostId,
  sessionId,
  canSave,
  additionalButtons,
}: {
  hostId: string;
  sessionId: string;
  canSave?: boolean;
  additionalButtons?: React.ReactNode;
}) {
  const navigate = useNavigate();
  const notifications = useNotifications();
  const { mutateAsync: mutateSave, isPending: isPendingSave } =
    useMutation(saveSessionWorld);
  const { mutateAsync: mutateStop, isPending: isPendingStop } =
    useMutation(stopSession);

  const handleSave = async () => {
    try {
      await mutateSave({
        hostId,
        sessionId,
      });
      notifications.show("ワールドを保存しました", {
        severity: "success",
        autoHideDuration: 3000,
      });
    } catch (e) {
      notifications.show(`セッションの保存に失敗しました: ${e}`, {
        severity: "error",
        autoHideDuration: 3000,
      });
    }
  };

  const handleStop = async () => {
    try {
      await mutateStop({
        hostId,
        sessionId,
      });
      notifications.show("セッションを停止しました", {
        severity: "success",
        autoHideDuration: 3000,
      });
      navigate("/sessions");
    } catch (e) {
      notifications.show(`セッションの停止に失敗しました: ${e}`, {
        severity: "error",
        autoHideDuration: 3000,
      });
    }
  };

  return (
    <Stack direction="row" spacing={1} sx={{ alignItems: "center" }}>
      <Button
        variant="contained"
        loading={isPendingSave}
        onClick={handleSave}
        disabled={!canSave}
      >
        ワールド保存
      </Button>
      <Button
        variant="contained"
        color="warning"
        loading={isPendingStop}
        onClick={handleStop}
      >
        セッション停止
      </Button>
      {additionalButtons}
    </Stack>
  );
}
