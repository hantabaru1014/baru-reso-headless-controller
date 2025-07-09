import { Button } from "./ui/button";
import { useNavigate } from "react-router";
import {
  saveSessionWorld,
  stopSession,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useMutation } from "@connectrpc/connect-query";
import { toast } from "sonner";
import { DropdownMenuItem } from "./ui/dropdown-menu";
import { SaveSessionWorldRequest_SaveMode } from "../../pbgen/hdlctrl/v1/controller_pb";
import { SplitButton } from "./base/SplitButton";

export default function SessionControlButtons({
  sessionId,
  canSaveOverride,
  canSaveAs,
  additionalButtons,
}: {
  sessionId: string;
  canSaveOverride?: boolean;
  canSaveAs?: boolean;
  additionalButtons?: React.ReactNode;
}) {
  const navigate = useNavigate();
  const { mutateAsync: mutateSave, isPending: isPendingSave } =
    useMutation(saveSessionWorld);
  const { mutateAsync: mutateStop, isPending: isPendingStop } =
    useMutation(stopSession);

  const handleSave = async (saveMode: SaveSessionWorldRequest_SaveMode) => {
    try {
      await mutateSave({
        sessionId,
        saveMode,
      });

      toast.success("ワールドを保存しました");
    } catch (e) {
      toast.error(`セッションの保存に失敗しました: ${e}`);
    }
  };

  const handleStop = async () => {
    try {
      await mutateStop({
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
      <SplitButton
        variant="outline"
        disabled={isPendingSave || !canSaveOverride}
        onClick={() => handleSave(SaveSessionWorldRequest_SaveMode.OVERWRITE)}
        dropdownContent={
          <>
            <DropdownMenuItem
              onClick={() =>
                handleSave(SaveSessionWorldRequest_SaveMode.SAVE_AS)
              }
              disabled={isPendingSave || !canSaveAs}
            >
              名前を付けて保存
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => handleSave(SaveSessionWorldRequest_SaveMode.COPY)}
              disabled={isPendingSave || !canSaveAs}
            >
              コピーして保存
            </DropdownMenuItem>
          </>
        }
      >
        ワールド保存
      </SplitButton>
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
