import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useAtomValue } from "jotai";
import { currentGroupIdAtom } from "../atoms/currentGroupAtom";
import { listGroups } from "../../pbgen/hdlctrl/v1/permission-GroupService_connectquery";
import {
  createScheduledSessionOperation,
  listHeadlessHost,
  startWorld,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  Alert,
  AlertDescription,
  Badge,
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
  makeSessionFormSchema,
  removeUndefined,
  searchParamsToFormValues,
  SessionFormValues,
} from "../libs/sessionFormUtils";
import { useTranslation } from "react-i18next";
import { HeadlessHostStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import { SelectField, TextField } from "./base";
import { useMemo, useState } from "react";
import SessionStartupFields from "./SessionStartupFields";
import { usePermissions } from "../hooks/usePermissions";
import { PERMISSION_KEYS } from "../libs/permissionUtils";
import { hostStatusToLabel } from "../libs/hostUtils";
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

// 停止しているホスト. セッション開始時に backend がホストを起動してから開始する.
const isStoppedHostStatus = (status: HeadlessHostStatus) =>
  status === HeadlessHostStatus.EXITED || status === HeadlessHostStatus.CRASHED;

export default function NewSessionForm() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const prefillValues = searchParamsToFormValues(searchParams);
  const { mutateAsync: mutateStart, isPending: isPendingStart } =
    useMutation(startWorld);
  const { mutateAsync: mutateSchedule, isPending: isPendingSchedule } =
    useMutation(createScheduledSessionOperation);
  const currentGroupId = useAtomValue(currentGroupIdAtom);
  const { data: hostList } = useQuery(listHeadlessHost, {
    groupId: currentGroupId ?? undefined,
  });
  const { data: groupsList } = useQuery(listGroups, {});
  const groupNameById = useMemo(() => {
    const m = new Map<string, string>();
    for (const g of groupsList?.groups ?? []) m.set(g.id, g.name);
    return m;
  }, [groupsList?.groups]);
  const { hasPermission } = usePermissions();
  const sessionFormSchema = useMemo(() => makeSessionFormSchema(t), [t]);

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

  // セッション開始には host:use + session:write が host.group_id に対して必要.
  // 権限を持たないグループのホストは選択肢に出さない.
  // 停止中のホストは開始時に backend が起動するため、追加で host:write も要求する.
  // 加えて、ヘッダーで選択中のグループでも絞り込み (backend 側も group_id でフィルタするが
  // listHeadlessHost の cache を別キーで使う他の画面と混ざらないように client 側でも明示する).
  const selectableHosts = useMemo(
    () =>
      (hostList?.hosts ?? [])
        .filter((host) => !currentGroupId || host.groupId === currentGroupId)
        .filter(
          (host) =>
            hasPermission(host.groupId, PERMISSION_KEYS.HOST_USE) &&
            hasPermission(host.groupId, PERMISSION_KEYS.SESSION_WRITE),
        )
        .filter(
          (host) =>
            host.status === HeadlessHostStatus.RUNNING ||
            (isStoppedHostStatus(host.status) &&
              hasPermission(host.groupId, PERMISSION_KEYS.HOST_WRITE)),
        )
        .map((host) => {
          const groupName = host.groupId
            ? (groupNameById.get(host.groupId) ?? host.groupId)
            : t("newSessionForm.noGroup");
          const stopped = isStoppedHostStatus(host.status);
          // 停止中のホストは resoniteVersion が空なので、空の要素は詰める.
          const baseLabel = [
            `${host.name} (${host.id.slice(0, 6)})`,
            host.accountName,
            host.resoniteVersion,
            `[${groupName}]`,
          ]
            .filter(Boolean)
            .join(" - ");
          const statusLabel = hostStatusToLabel(host.status);
          return {
            id: host.id,
            label: stopped ? `${baseLabel} (${statusLabel})` : baseLabel,
            baseLabel,
            statusLabel,
            stopped,
            value: host,
          };
        })
        // 稼働中 → 停止中の順に並べる (sort は安定なので各グループ内の順序は保たれる).
        .sort((a, b) => Number(a.stopped) - Number(b.stopped)),
    [hostList?.hosts, hasPermission, currentGroupId, groupNameById, t],
  );
  const isSelectedHostStopped =
    selectableHosts.find((host) => host.id === hostId)?.stopped ?? false;

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
      // 非同期 job として実行されるので「受け付けた」だけ通知し、
      // 完了は notificationDispatch 経由の JobCompletedEvent toast で出す.
      toast.success(
        isSelectedHostStopped
          ? t("newSessionForm.startAcceptedWithHostStart")
          : t("newSessionForm.startAccepted"),
      );
      navigate("/sessions");
    } catch (e) {
      toast.error(
        t("newSessionForm.startError", {
          message: e instanceof Error ? e.message : e,
        }),
      );
    }
  };

  const openScheduleDialog = async () => {
    // 予約実行はホストを起動しないので、停止中のホストでは予約させない.
    if (isSelectedHostStopped) {
      toast.error(t("newSessionForm.scheduleRequiresRunningHost"));
      return;
    }

    const ok = await validate();
    if (!ok) {
      toast.error(t("newSessionForm.checkInput"));
      return;
    }
    setScheduleOpen(true);
  };

  const submitSchedule = async () => {
    const data = getValues();
    const at = localDateTimeStringToDate(scheduledAt);
    if (Number.isNaN(at.getTime())) {
      toast.error(t("newSessionForm.invalidDateTime"));
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
      toast.success(t("newSessionForm.scheduleCreated"));
      setScheduleOpen(false);
      navigate("/scheduled");
    } catch (e) {
      toast.error(
        t("newSessionForm.scheduleError", {
          message: e instanceof Error ? e.message : e,
        }),
      );
    }
  };

  return (
    <>
      <Dialog open={!hostId}>
        <DialogContent
          showCloseButton={false}
          onInteractOutside={(e) => e.preventDefault()}
          onEscapeKeyDown={(e) => e.preventDefault()}
        >
          <DialogHeader>
            <DialogTitle>{t("newSessionForm.selectHostTitle")}</DialogTitle>
            <DialogDescription>
              {t("newSessionForm.selectHostDescription")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2 max-h-[70vh] overflow-y-auto">
            {selectableHosts.map((host) => (
              <Button
                key={host.id}
                variant="outline"
                className="w-full h-auto min-h-9 justify-start whitespace-normal text-left"
                onClick={() => setValue("hostId", host.id)}
              >
                <span className="flex-1">{host.baseLabel}</span>
                {host.stopped && (
                  <Badge variant="secondary">{host.statusLabel}</Badge>
                )}
              </Button>
            ))}
            {selectableHosts.length === 0 && (
              <div className="text-center py-4 space-y-3">
                <p className="text-muted-foreground">
                  {t("newSessionForm.noSelectableHosts")}
                </p>
                <Button variant="outline" asChild>
                  <Link to="/hosts">{t("newSessionForm.toHostList")}</Link>
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
              options={selectableHosts}
              selectedId={field.value || ""}
              onChange={(option) => field.onChange(option.value?.id ?? "")}
              minWidth="7rem"
              error={errors.hostId?.message}
            />
          )}
        />
        {isSelectedHostStopped && (
          <Alert>
            <AlertDescription>
              {t("newSessionForm.stoppedHostNotice")}
            </AlertDescription>
          </Alert>
        )}

        <SessionStartupFields
          control={control}
          errors={errors}
          watch={watch}
          setValue={setValue}
          hostRunning={!isSelectedHostStopped}
        />

        <div className="sticky bottom-0 border-t p-4 mt-8 bg-background flex gap-2">
          <Button
            type="submit"
            disabled={Object.keys(errors).length > 0 || isPendingStart}
          >
            {t("newSessionForm.startSession")}
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={openScheduleDialog}
            disabled={isPendingStart}
          >
            {t("newSessionForm.scheduleStart")}
          </Button>
        </div>
      </form>

      <Dialog open={scheduleOpen} onOpenChange={setScheduleOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("newSessionForm.scheduleStart")}</DialogTitle>
            <DialogDescription>
              {t("newSessionForm.scheduleDialogDescription")}
            </DialogDescription>
          </DialogHeader>
          <TextField
            label={t("newSessionForm.scheduledAt")}
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
              {t("common.cancel")}
            </Button>
            <Button onClick={submitSchedule} disabled={isPendingSchedule}>
              {t("newSessionForm.createSchedule")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
