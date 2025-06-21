import { Button } from "./ui/button";
import { useNavigate } from "react-router";
import {
  saveSessionWorld,
  stopSession,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useMutation } from "@connectrpc/connect-query";
import { toast } from "sonner";

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
      toast.success("ワールドを保存しました");
    } catch (e) {
      toast.error(`セッションの保存に失敗しました: ${e}`);
    }
  };

  const handleStop = async () => {
    try {
      await mutateStop({
        hostId,
        sessionId,
      });
      toast.success("セッションを停止しました");
      navigate("/sessions");
    } catch (e) {
      toast.error(`セッションの停止に失敗しました: ${e}`);
    }
  };

  return (
    <div className="flex items-center gap-2">
      <Button
        variant="outline"
        disabled={isPendingSave || !canSave}
        onClick={handleSave}
      >
        ワールド保存
      </Button>
      <Button
        variant="destructive"
        disabled={isPendingStop}
        onClick={handleStop}
      >
        停止
      </Button>
      {additionalButtons}
    </div>
  );
}
