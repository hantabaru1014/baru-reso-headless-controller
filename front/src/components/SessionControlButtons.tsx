import { Link, useNavigate } from "react-router";
import {
  createScheduledSessionOperation,
  prepareSessionWorldDownload,
  saveSessionWorld,
  stopSession,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useMutation } from "@connectrpc/connect-query";
import { toast } from "sonner";
import { DropdownMenuItem, DropdownMenuSeparator } from "./ui/dropdown-menu";
import {
  SaveSessionWorldRequest_SaveMode,
  ScheduledOperationSchema,
  ScheduledTriggerSchema,
  SessionUserCountTriggerSchema,
  SessionUserCountTrigger_Comparator,
  StopSessionRequestSchema,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import { WorldBinaryFormat } from "../../pbgen/headless/v1/headless_pb";
import { SplitButton } from "./base/SplitButton";
import { create } from "@bufbuild/protobuf";
import { useTranslation } from "react-i18next";

export default function SessionControlButtons({
  sessionId,
  canSaveOverride,
  canSaveAs,
  leadingButtons,
  additionalButtons,
}: {
  sessionId: string;
  canSaveOverride?: boolean;
  canSaveAs?: boolean;
  leadingButtons?: React.ReactNode;
  additionalButtons?: React.ReactNode;
}) {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { mutateAsync: mutateSave, isPending: isPendingSave } =
    useMutation(saveSessionWorld);
  const { mutateAsync: mutateStop, isPending: isPendingStop } =
    useMutation(stopSession);
  const { mutateAsync: mutatePrepareDownload, isPending: isPendingDownload } =
    useMutation(prepareSessionWorldDownload);
  const {
    mutateAsync: mutateScheduleStopWhenEmpty,
    isPending: isPendingScheduleStop,
  } = useMutation(createScheduledSessionOperation);

  const handleSave = async (saveMode: SaveSessionWorldRequest_SaveMode) => {
    try {
      await mutateSave({
        sessionId,
        saveMode,
      });

      toast.success(t("sessionControlButtons.worldSaved"));
    } catch (e) {
      toast.error(t("sessionControlButtons.worldSaveFailed", { error: e }));
    }
  };

  const handleStop = async () => {
    try {
      await mutateStop({
        sessionId,
      });
      // 非同期 job として実行されるので「受け付けた」だけ通知し、
      // 完了は notificationDispatch 経由の JobCompletedEvent toast で出す.
      toast.success(t("sessionControlButtons.stopAccepted"));
      navigate("/sessions");
    } catch (e) {
      toast.error(t("sessionControlButtons.stopFailed", { error: e }));
    }
  };

  const handleScheduleStopWhenEmpty = async () => {
    try {
      const operation = create(ScheduledOperationSchema, {
        operation: {
          case: "stopSession",
          value: create(StopSessionRequestSchema, { sessionId }),
        },
      });
      const trigger = create(ScheduledTriggerSchema, {
        trigger: {
          case: "sessionUserCount",
          value: create(SessionUserCountTriggerSchema, {
            sessionId,
            comparator: SessionUserCountTrigger_Comparator.LESS_OR_EQUAL,
            threshold: 0,
          }),
        },
      });
      await mutateScheduleStopWhenEmpty({ operation, trigger });
      toast.success(t("sessionControlButtons.scheduleStopWhenEmptyCreated"));
    } catch (e) {
      toast.error(
        t("sessionControlButtons.scheduleCreateFailed", { error: e }),
      );
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
      toast.success(t("sessionControlButtons.downloadStarted"));
    } catch (e) {
      toast.error(
        t("sessionControlButtons.downloadPrepareFailed", { error: e }),
      );
    }
  };

  return (
    <div className="flex items-center gap-2">
      {leadingButtons}
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
              {t("sessionControlButtons.saveAs")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => handleSave(SaveSessionWorldRequest_SaveMode.COPY)}
              disabled={isPendingSave || !canSaveAs}
            >
              {t("sessionControlButtons.saveCopy")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={() =>
                handleDownload(WorldBinaryFormat.WORLD_BINARY_FORMAT_7ZBSON)
              }
              disabled={isPendingDownload || !canSaveOverride}
            >
              {t("sessionControlButtons.download", { format: "7zbson" })}
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() =>
                handleDownload(WorldBinaryFormat.WORLD_BINARY_FORMAT_BRSON)
              }
              disabled={isPendingDownload || !canSaveOverride}
            >
              {t("sessionControlButtons.download", { format: "brson" })}
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() =>
                handleDownload(
                  WorldBinaryFormat.WORLD_BINARY_FORMAT_RESONITEPACKAGE,
                )
              }
              disabled={isPendingDownload || !canSaveOverride}
            >
              {t("sessionControlButtons.download", {
                format: "resonitepackage",
              })}
            </DropdownMenuItem>
          </>
        }
      >
        {t("sessionControlButtons.saveWorld")}
      </SplitButton>
      <SplitButton
        variant="destructive"
        disabled={isPendingStop}
        onClick={handleStop}
        dropdownContent={
          <>
            <DropdownMenuItem
              onClick={handleScheduleStopWhenEmpty}
              disabled={isPendingScheduleStop}
            >
              {t("sessionControlButtons.stopWhenEmpty")}
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <Link to={`/sessions/scheduled/new?sessionId=${sessionId}`}>
                {t("sessionControlButtons.createOtherSchedule")}
              </Link>
            </DropdownMenuItem>
          </>
        }
      >
        {t("sessionControlButtons.stop")}
      </SplitButton>
      {additionalButtons}
    </div>
  );
}
