import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  createScheduledSessionOperation,
  listHeadlessHost,
  startWorld,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui";
import { Link, useNavigate, useSearchParams } from "react-router";
import {
  buildStartWorldParameters,
  DEFAULT_SESSION_FORM_VALUES,
  removeUndefined,
  searchParamsToFormValues,
  sessionFormSchema,
  SessionFormValues,
} from "../libs/sessionFormUtils";
import { HeadlessHostStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import { SelectField, TextField } from "./base";
import { useState } from "react";
import SessionStartupFields from "./SessionStartupFields";
import {
  ScheduledOperationSchema,
  ScheduledTriggerSchema,
  StartWorldRequestSchema,
  TimeTriggerSchema,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import { create } from "@bufbuild/protobuf";
import {
  dateToTimestamp,
  defaultScheduledAtInputValue,
  localDateTimeStringToDate,
} from "../libs/scheduledOperationUtils";

export default function NewSessionForm() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const prefillValues = searchParamsToFormValues(searchParams);
  const { mutateAsync: mutateStart, isPending: isPendingStart } =
    useMutation(startWorld);
  const { mutateAsync: mutateSchedule, isPending: isPendingSchedule } =
    useMutation(createScheduledSessionOperation);
  const { data: hostList } = useQuery(listHeadlessHost);

  const {
    control,
    handleSubmit,
    watch,
    setValue,
    getValues,
    trigger: validate,
    formState: { errors },
  } = useForm<SessionFormValues>({
    resolver: zodResolver(sessionFormSchema),
    mode: "onBlur",
    defaultValues: {
      ...DEFAULT_SESSION_FORM_VALUES,
      ...removeUndefined(prefillValues),
    },
  });

  const hostId = watch("hostId");

  const [scheduleOpen, setScheduleOpen] = useState(false);
  const [scheduledAt, setScheduledAt] = useState(
    defaultScheduledAtInputValue(),
  );

  const onSubmit = async (data: SessionFormValues) => {
    try {
      await mutateStart({
        hostId: data.hostId,
        parameters: buildStartWorldParameters(data),
      });
      toast.success("セッションを開始しました");
      navigate("/sessions");
    } catch (e) {
      toast.error(`エラー: ${e instanceof Error ? e.message : e}`);
    }
  };

  const openScheduleDialog = async () => {
    const ok = await validate();
    if (!ok) {
      toast.error("入力内容を確認してください");
      return;
    }
    setScheduleOpen(true);
  };

  const submitSchedule = async () => {
    const data = getValues();
    const at = localDateTimeStringToDate(scheduledAt);
    if (Number.isNaN(at.getTime())) {
      toast.error("実行日時が不正です");
      return;
    }
    try {
      const operation = create(ScheduledOperationSchema, {
        operation: {
          case: "startSession",
          value: create(StartWorldRequestSchema, {
            hostId: data.hostId,
            parameters: buildStartWorldParameters(data),
          }),
        },
      });
      const trigger = create(ScheduledTriggerSchema, {
        trigger: {
          case: "time",
          value: create(TimeTriggerSchema, {
            scheduledAt: dateToTimestamp(at),
          }),
        },
      });
      await mutateSchedule({ operation, trigger });
      toast.success("予約を作成しました");
      setScheduleOpen(false);
      navigate("/sessions/scheduled");
    } catch (e) {
      toast.error(`予約失敗: ${e instanceof Error ? e.message : e}`);
    }
  };

  const runningHosts =
    hostList?.hosts
      .filter((host) => host.status === HeadlessHostStatus.RUNNING)
      .map((host) => ({
        id: host.id,
        label: `${host.name} (${host.id.slice(0, 6)}) - ${host.accountName} - ${host.resoniteVersion}`,
        value: host,
      })) ?? [];

  return (
    <>
      <Dialog open={!hostId}>
        <DialogContent
          showCloseButton={false}
          onInteractOutside={(e) => e.preventDefault()}
          onEscapeKeyDown={(e) => e.preventDefault()}
        >
          <DialogHeader>
            <DialogTitle>ホストを選択</DialogTitle>
            <DialogDescription>
              セッションを開始するホストを選択してください
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2 max-h-[70vh] overflow-y-auto">
            {runningHosts.map((host) => (
              <Button
                key={host.id}
                variant="outline"
                className="w-full justify-start"
                onClick={() => setValue("hostId", host.id)}
              >
                {host.label}
              </Button>
            ))}
            {runningHosts.length === 0 && (
              <div className="text-center py-4 space-y-3">
                <p className="text-muted-foreground">
                  稼働中のホストがありません
                </p>
                <Button variant="outline" asChild>
                  <Link to="/hosts">ホスト一覧へ</Link>
                </Button>
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>

      <form className="space-y-4" onSubmit={handleSubmit(onSubmit)}>
        <Controller
          name="hostId"
          control={control}
          render={({ field }) => (
            <SelectField
              label="Host"
              options={runningHosts}
              selectedId={field.value || ""}
              onChange={(option) => field.onChange(option.value?.id ?? "")}
              minWidth="7rem"
              error={errors.hostId?.message}
            />
          )}
        />

        <SessionStartupFields
          control={control}
          errors={errors}
          watch={watch}
          setValue={setValue}
        />

        <div className="sticky bottom-0 border-t p-4 mt-8 bg-background flex gap-2">
          <Button
            type="submit"
            disabled={Object.keys(errors).length > 0 || isPendingStart}
          >
            セッション開始
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={openScheduleDialog}
            disabled={isPendingStart}
          >
            セッション開始を予約
          </Button>
        </div>
      </form>

      <Dialog open={scheduleOpen} onOpenChange={setScheduleOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>セッション開始を予約</DialogTitle>
            <DialogDescription>
              現在のフォームの設定でセッションを開始する日時を指定してください
            </DialogDescription>
          </DialogHeader>
          <TextField
            label="実行日時"
            type="datetime-local"
            value={scheduledAt}
            onChange={(e) => setScheduledAt(e.target.value)}
          />
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setScheduleOpen(false)}
              disabled={isPendingSchedule}
            >
              キャンセル
            </Button>
            <Button onClick={submitSchedule} disabled={isPendingSchedule}>
              予約を作成
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
