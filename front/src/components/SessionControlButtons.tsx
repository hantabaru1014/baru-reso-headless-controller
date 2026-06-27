import { Button } from "./ui/button";
import { useNavigate } from "react-router";
import {
  prepareSessionWorldDownload,
  saveSessionWorld,
  stopSession,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useMutation } from "@connectrpc/connect-query";
import { toast } from "sonner";
import { DropdownMenuItem, DropdownMenuSeparator } from "./ui/dropdown-menu";
import { SaveSessionWorldRequest_SaveMode } from "../../pbgen/hdlctrl/v1/controller_pb";
import { WorldBinaryFormat } from "../../pbgen/headless/v1/headless_pb";
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
  const { mutateAsync: mutatePrepareDownload, isPending: isPendingDownload } =
    useMutation(prepareSessionWorldDownload);

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
      // 非同期 job として実行されるので「受け付けた」だけ通知し、
      // 完了は notificationDispatch 経由の JobCompletedEvent toast で出す.
      toast.success("セッションの停止を受け付けました");
      navigate("/sessions");
    } catch (e) {
      toast.error(`セッションの停止に失敗しました: ${e}`);
    }
  };

  const handleDownload = async (format: WorldBinaryFormat) => {
    try {
      const res = await mutatePrepareDownload({ sessionId, format });
      const a = document.createElement("a");
      a.href = res.downloadUrl;
      a.download = res.filename;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      toast.success("ワールドのダウンロードを開始しました");
    } catch (e) {
      toast.error(`ワールドのダウンロード準備に失敗しました: ${e}`);
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
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={() =>
                handleDownload(WorldBinaryFormat.WORLD_BINARY_FORMAT_7ZBSON)
              }
              disabled={isPendingDownload || !canSaveOverride}
            >
              ダウンロード (7zbson)
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() =>
                handleDownload(WorldBinaryFormat.WORLD_BINARY_FORMAT_BRSON)
              }
              disabled={isPendingDownload || !canSaveOverride}
            >
              ダウンロード (brson)
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() =>
                handleDownload(
                  WorldBinaryFormat.WORLD_BINARY_FORMAT_RESONITEPACKAGE,
                )
              }
              disabled={isPendingDownload || !canSaveOverride}
            >
              ダウンロード (resonitepackage)
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
