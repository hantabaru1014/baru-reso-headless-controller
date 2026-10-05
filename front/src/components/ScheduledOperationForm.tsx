import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  createScheduledSessionOperation,
  listHeadlessHost,
  searchSessions,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  CronTrigger_Frequency,
  CronTriggerSchema,
  HeadlessHostStatus,
  RestartHeadlessHostRequestSchema,
  ScheduledOperationSchema,
  ScheduledStartHostOperationSchema,
  ScheduledTrigger,
  ScheduledTriggerSchema,
  SendDynamicImpulseRequestSchema as HdlSendDynamicImpulseRequestSchema,
  SessionStatus,
  SessionUserCountTriggerSchema,
  SessionUserCountTrigger_Comparator,
  ShutdownHeadlessHostRequestSchema,
  StartWorldRequestSchema,
  StopSessionRequestSchema,
  TimeTriggerSchema,
  UpdateSessionExtraSettingsRequestSchema,
  UpdateSessionParametersRequestSchema as HdlUpdateSessionParametersRequestSchema,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import {
  AccessLevel,
  SendDynamicImpulseRequestSchema,
  UpdateSessionParametersRequestSchema,
} from "../../pbgen/headless/v1/headless_pb";
import { create } from "@bufbuild/protobuf";
import { useMemo, useState } from "react";
import { Button } from "./ui/button";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { Controller, useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  CheckboxField,
  RadioGroupField,
  SelectField,
  TextField,
  TextareaField,
} from "./base";
import { Checkbox, Label } from "./ui";
import {
  browserTimeZone,
  CRON_FREQUENCIES,
  CronFrequency,
  cronFrequencyLabel,
  dateToTimestamp,
  defaultScheduledAtInputValue,
  HostOperationKind,
  isHostOperationKind,
  localDateTimeStringToDate,
  monthLabel,
  OPERATION_KINDS,
  operationKindLabel,
  OperationKind,
  TRIGGER_KINDS,
  TriggerKind,
  triggerKindLabel,
  UserCountComparator,
  userCountComparatorLabel,
  weekdayShortLabel,
} from "../libs/scheduledOperationUtils";
import { hostStatusToLabel } from "../libs/hostUtils";
import { buildImpulseValue, IMPULSE_VALUE_TYPES } from "../libs/sessionUtils";
import { usePermissions } from "../hooks/usePermissions";
import { PERMISSION_KEYS } from "../libs/permissionUtils";
import { AccessLevels } from "../constants";
import {
  buildStartWorldParameters,
  DEFAULT_SESSION_FORM_VALUES,
  makeSessionFormSchema,
  SessionFormValues,
} from "../libs/sessionFormUtils";
import SessionStartupFields from "./SessionStartupFields";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";

type Props = {
  /** プリセレクト用: セッション詳細から開いたとき (トリガー監視/操作対象の両方の初期値になる) */
  defaultSessionId?: string;
  /** プリセレクト用: ホスト詳細から開いたとき (ホスト操作の対象の初期値になる) */
  defaultHostId?: string;
  /** トリガー / アクション種別の初期値. セッション詳細の「ユーザー0人で停止」から開かれた場合等. */
  defaultTrigger?: TriggerKind;
  defaultOperation?: OperationKind;
  /** SESSION_USER_COUNT trigger の初期値. */
  defaultUserCountComparator?: UserCountComparator;
  defaultUserCountThreshold?: number;
};

const makeTriBoolOptions = (t: TFunction) => [
  { id: "_", label: t("scheduledOperationForm.keepUnchanged") },
  { id: "true", label: t("common.yes") },
  { id: "false", label: t("common.no") },
];

const COMPARATOR_KINDS: UserCountComparator[] = [
  "LESS_OR_EQUAL",
  "GREATER_OR_EQUAL",
];

const WEEKDAYS = [0, 1, 2, 3, 4, 5, 6];
const DAYS_OF_MONTH = Array.from({ length: 31 }, (_, i) => i + 1);
const MONTHS = Array.from({ length: 12 }, (_, i) => i + 1);

type SessionOption = { id: string; label: string };

/** session list を取得して dropdown 用に整形する hook. trigger / action 両方で共有して 1 回だけ fetch. */
function useSessionOptions(defaultSessionId: string | undefined): {
  options: SessionOption[];
  hasRunningSessions: boolean;
} {
  const { t } = useTranslation();
  const { data: sessionsData } = useQuery(searchSessions, {
    parameters: { status: SessionStatus.RUNNING },
    page: { pageIndex: 0, pageSize: 100 },
  });

  return useMemo(() => {
    const list = sessionsData?.sessions ?? [];
    const merged = list.slice();
    if (defaultSessionId && !list.find((s) => s.id === defaultSessionId)) {
      merged.unshift({
        id: defaultSessionId,
        name: defaultSessionId,
      } as (typeof list)[number]);
    }
    const options: SessionOption[] = [
      { id: "_", label: t("scheduledOperationForm.selectPlaceholder") },
    ].concat(
      merged.map((s) => ({
        id: s.id,
        label: `${s.name || t("scheduledOperationForm.noName")} (${s.id.slice(0, 8)}…)`,
      })),
    );
    return { options, hasRunningSessions: merged.length > 0 };
  }, [sessionsData, defaultSessionId, t]);
}

export default function ScheduledOperationForm({
  defaultSessionId,
  defaultHostId,
  defaultTrigger,
  defaultOperation,
  defaultUserCountComparator,
  defaultUserCountThreshold,
}: Props) {
  // useTranslation を購読しておくことで言語切替時に再レンダーされ、
  // 下記の *Label 系 (i18n.t を内部で呼ぶ) が最新の言語で再評価される.
  const { t } = useTranslation();
  const [trigger, setTrigger] = useState<TriggerKind>(defaultTrigger ?? "TIME");
  const [kind, setKind] = useState<OperationKind>(
    defaultOperation ??
      (defaultHostId
        ? "RESTART_HOST"
        : defaultSessionId
          ? "STOP_SESSION"
          : "START_SESSION"),
  );

  const triggerOptions = TRIGGER_KINDS.map((k) => ({
    label: triggerKindLabel(k),
    value: k,
  }));
  // t は言語切替で参照が変わるので、言語ごとに 1 度だけ組み立てる.
  const { cronFrequencyOptions, dayOfMonthOptions, monthOptions } = useMemo(
    () => ({
      cronFrequencyOptions: CRON_FREQUENCIES.map((f) => ({
        id: f,
        label: cronFrequencyLabel(f),
      })),
      dayOfMonthOptions: DAYS_OF_MONTH.map((d) => ({
        id: String(d),
        label: t("scheduledOperationForm.dayOfMonthOption", { day: d }),
      })),
      monthOptions: MONTHS.map((m) => ({
        id: String(m),
        label: monthLabel(m),
      })),
    }),
    [t],
  );
  const comparatorOptions = COMPARATOR_KINDS.map((c) => ({
    id: c,
    label: userCountComparatorLabel(c),
  }));
  const actionOptions = OPERATION_KINDS.map((k) => ({
    label: operationKindLabel(k),
    value: k,
  }));

  // ▼ Section ① のトリガー設定 (全 action から参照されるので parent owned)
  const [scheduledAt, setScheduledAt] = useState(
    defaultScheduledAtInputValue(),
  );
  const [monitorSessionId, setMonitorSessionId] = useState(
    defaultSessionId ?? "",
  );
  const [comparator, setComparator] = useState<UserCountComparator>(
    defaultUserCountComparator ?? "LESS_OR_EQUAL",
  );
  const [threshold, setThreshold] = useState(
    defaultUserCountThreshold !== undefined
      ? String(defaultUserCountThreshold)
      : "0",
  );
  const [cronFrequency, setCronFrequency] = useState<CronFrequency>("DAILY");
  const [cronTime, setCronTime] = useState("04:00");
  const [cronWeekdays, setCronWeekdays] = useState<number[]>([1]);
  const [cronDayOfMonth, setCronDayOfMonth] = useState("1");
  const [cronMonth, setCronMonth] = useState("1");
  const timeZone = useMemo(() => browserTimeZone(), []);
  const usesDayOfMonth =
    cronFrequency === "MONTHLY" || cronFrequency === "YEARLY";

  const { options: sessionOptions, hasRunningSessions } =
    useSessionOptions(defaultSessionId);

  /**
   * 現在のトリガー設定を ScheduledTrigger proto に変換する.
   * 失敗時は toast を出して null を返す (action の submit handler から呼ばれる前提).
   */
  const buildTrigger = (): ScheduledTrigger | null => {
    if (trigger === "TIME") {
      if (!scheduledAt) {
        toast.error(t("scheduledOperationForm.specifyDateTime"));
        return null;
      }
      const at = localDateTimeStringToDate(scheduledAt);
      if (Number.isNaN(at.getTime())) {
        toast.error(t("scheduledOperationForm.invalidDateTime"));
        return null;
      }
      return create(ScheduledTriggerSchema, {
        trigger: {
          case: "time",
          value: create(TimeTriggerSchema, {
            scheduledAt: dateToTimestamp(at),
          }),
        },
      });
    }
    if (trigger === "CRON") {
      const [hour, minute] = cronTime.split(":").map((v) => parseInt(v, 10));
      if (!Number.isFinite(hour) || !Number.isFinite(minute)) {
        toast.error(t("scheduledOperationForm.specifyTime"));
        return null;
      }
      if (cronFrequency === "WEEKLY" && cronWeekdays.length === 0) {
        toast.error(t("scheduledOperationForm.selectWeekdays"));
        return null;
      }
      // 頻度に関係のないフィールドはサーバー側で捨てられる.
      return create(ScheduledTriggerSchema, {
        trigger: {
          case: "cron",
          value: create(CronTriggerSchema, {
            frequency: CronTrigger_Frequency[cronFrequency],
            hour,
            minute,
            weekdays: cronWeekdays,
            dayOfMonth: parseInt(cronDayOfMonth, 10),
            month: parseInt(cronMonth, 10),
            timezone: timeZone,
          }),
        },
      });
    }
    if (!monitorSessionId) {
      toast.error(t("scheduledOperationForm.selectMonitorSession"));
      return null;
    }
    const parsed = parseIntOrUndef(threshold);
    if (parsed === undefined || parsed < 0) {
      toast.error(t("scheduledOperationForm.invalidThreshold"));
      return null;
    }
    return create(ScheduledTriggerSchema, {
      trigger: {
        case: "sessionUserCount",
        value: create(SessionUserCountTriggerSchema, {
          sessionId: monitorSessionId,
          comparator:
            comparator === "GREATER_OR_EQUAL"
              ? SessionUserCountTrigger_Comparator.GREATER_OR_EQUAL
              : SessionUserCountTrigger_Comparator.LESS_OR_EQUAL,
          threshold: parsed,
        }),
      },
    });
  };

  return (
    <div className="space-y-6">
      <section className="space-y-3 rounded-md border p-4">
        <h3 className="text-sm font-semibold">
          {t("scheduledOperationForm.triggerSectionTitle")}
        </h3>
        <RadioGroupField
          label={t("scheduledOperationForm.triggerQuestion")}
          options={triggerOptions}
          value={trigger}
          onValueChange={(v) => setTrigger(v as TriggerKind)}
          className="flex flex-row flex-wrap gap-4"
        />

        {trigger === "TIME" ? (
          <TextField
            label={t("scheduledOperationForm.scheduledAt")}
            type="datetime-local"
            value={scheduledAt}
            onChange={(e) => setScheduledAt(e.target.value)}
          />
        ) : trigger === "CRON" ? (
          <div className="space-y-3">
            <div className="flex flex-wrap gap-2">
              <SelectField
                label={t("scheduledOperationForm.frequency")}
                options={cronFrequencyOptions}
                selectedId={cronFrequency}
                onChange={(o) => setCronFrequency(o.id as CronFrequency)}
              />
              {cronFrequency === "YEARLY" && (
                <SelectField
                  label={t("scheduledOperationForm.month")}
                  options={monthOptions}
                  selectedId={cronMonth}
                  onChange={(o) => setCronMonth(o.id)}
                />
              )}
              {usesDayOfMonth && (
                <SelectField
                  label={t("scheduledOperationForm.dayOfMonth")}
                  options={dayOfMonthOptions}
                  selectedId={cronDayOfMonth}
                  onChange={(o) => setCronDayOfMonth(o.id)}
                />
              )}
              <TextField
                label={t("scheduledOperationForm.time")}
                type="time"
                className="w-32"
                value={cronTime}
                onChange={(e) => setCronTime(e.target.value)}
              />
            </div>
            {cronFrequency === "WEEKLY" && (
              <div className="space-y-2">
                <p className="text-sm font-medium">
                  {t("scheduledOperationForm.weekdays")}
                </p>
                <div className="flex flex-wrap gap-4">
                  {WEEKDAYS.map((w) => (
                    <div key={w} className="flex items-center gap-2">
                      <Checkbox
                        id={`cron-weekday-${w}`}
                        checked={cronWeekdays.includes(w)}
                        onCheckedChange={(checked) =>
                          setCronWeekdays((prev) =>
                            checked
                              ? [...prev, w].sort((a, b) => a - b)
                              : prev.filter((v) => v !== w),
                          )
                        }
                      />
                      <Label htmlFor={`cron-weekday-${w}`}>
                        {weekdayShortLabel(w)}
                      </Label>
                    </div>
                  ))}
                </div>
              </div>
            )}
            <p className="text-sm text-muted-foreground">
              {t("scheduledOperationForm.cronNote", { timeZone })}
              {usesDayOfMonth &&
                Number(cronDayOfMonth) >= 29 &&
                ` ${t("scheduledOperationForm.cronSkipNote")}`}
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            <SelectField
              label={t("scheduledOperationForm.monitorSession")}
              options={sessionOptions}
              selectedId={monitorSessionId || "_"}
              onChange={(o) => setMonitorSessionId(o.id === "_" ? "" : o.id)}
              helperText={
                hasRunningSessions
                  ? undefined
                  : t("scheduledOperationForm.noRunningSessionsHelper")
              }
            />
            <div className="flex gap-2">
              <TextField
                label={t("scheduledOperationForm.userCount")}
                type="number"
                className="w-32"
                value={threshold}
                onChange={(e) => setThreshold(e.target.value)}
              />
              <SelectField
                label={t("scheduledOperationForm.condition")}
                options={comparatorOptions}
                selectedId={comparator}
                onChange={(o) => setComparator(o.id as UserCountComparator)}
              />
            </div>
          </div>
        )}
      </section>

      <section className="space-y-3 rounded-md border p-4">
        <h3 className="text-sm font-semibold">
          {t("scheduledOperationForm.actionSectionTitle")}
        </h3>
        <RadioGroupField
          label={t("scheduledOperationForm.actionKind")}
          options={actionOptions}
          value={kind}
          onValueChange={(v) => setKind(v as OperationKind)}
          className="flex flex-row flex-wrap gap-4"
        />

        {kind === "START_SESSION" ? (
          <StartSessionActionForm buildTrigger={buildTrigger} />
        ) : isHostOperationKind(kind) ? (
          <HostActionForm
            kind={kind}
            defaultHostId={defaultHostId}
            buildTrigger={buildTrigger}
          />
        ) : (
          <OtherKindActionForm
            kind={kind}
            sessionOptions={sessionOptions}
            hasRunningSessions={hasRunningSessions}
            defaultSessionId={defaultSessionId}
            buildTrigger={buildTrigger}
          />
        )}
      </section>
    </div>
  );
}

/* ============================================================
 * START_SESSION action: 既存 NewSessionForm 相当のフィールド
 * (host + startup parameters). トリガーは時刻 / 条件いずれにも対応.
 * ============================================================ */

function StartSessionActionForm({
  buildTrigger,
}: {
  buildTrigger: () => ScheduledTrigger | null;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { mutateAsync, isPending } = useMutation(
    createScheduledSessionOperation,
  );
  const sessionFormSchema = useMemo(() => makeSessionFormSchema(t), [t]);

  const { data: hostList } = useQuery(listHeadlessHost);
  const runningHostOptions = useMemo(
    () =>
      hostList?.hosts
        .filter((h) => h.status === HeadlessHostStatus.RUNNING)
        .map((h) => ({
          id: h.id,
          label: `${h.name} (${h.id.slice(0, 6)}) - ${h.accountName} - ${h.resoniteVersion}`,
          value: h,
        })) ?? [],
    [hostList],
  );

  const {
    control,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<SessionFormValues>({
    resolver: zodResolver(sessionFormSchema),
    mode: "onBlur",
    defaultValues: DEFAULT_SESSION_FORM_VALUES,
  });

  const onSubmit = handleSubmit(async (data) => {
    const triggerMsg = buildTrigger();
    if (!triggerMsg) return;

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
      await mutateAsync({ operation, trigger: triggerMsg });
      toast.success(t("scheduledOperationForm.scheduleCreated"));
      navigate("/scheduled");
    } catch (err) {
      toast.error(
        t("scheduledOperationForm.createError", {
          message: (err as Error).message,
        }),
      );
    }
  });

  return (
    <form className="space-y-4" onSubmit={onSubmit}>
      <Controller
        name="hostId"
        control={control}
        render={({ field }) => (
          <SelectField
            label={t("scheduledOperationForm.host")}
            options={[
              { id: "_", label: t("scheduledOperationForm.selectPlaceholder") },
            ].concat(
              runningHostOptions.map((h) => ({ id: h.id, label: h.label })),
            )}
            selectedId={field.value || "_"}
            onChange={(o) => field.onChange(o.id === "_" ? "" : o.id)}
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

      <FormFooter
        navigate={navigate}
        disabled={isPending || isSubmitting || Object.keys(errors).length > 0}
        cancelDisabled={isPending || isSubmitting}
      />
    </form>
  );
}

/* ============================================================
 * START_HOST / RESTART_HOST / SHUTDOWN_HOST action: 対象ホスト
 * ============================================================ */

function HostActionForm({
  kind,
  defaultHostId,
  buildTrigger,
}: {
  kind: HostOperationKind;
  defaultHostId?: string;
  buildTrigger: () => ScheduledTrigger | null;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { mutateAsync, isPending } = useMutation(
    createScheduledSessionOperation,
  );
  const [hostId, setHostId] = useState(defaultHostId ?? "");
  const [withWorldRestart, setWithWorldRestart] = useState(true);

  const { data: hostList } = useQuery(listHeadlessHost);
  const { hasPermission } = usePermissions();
  const hostOptions = useMemo(
    () =>
      [
        { id: "_", label: t("scheduledOperationForm.selectPlaceholder") },
      ].concat(
        hostList?.hosts
          .filter((h) => hasPermission(h.groupId, PERMISSION_KEYS.HOST_WRITE))
          .map((h) => ({
            id: h.id,
            label: `${h.name} (${h.id.slice(0, 6)}) - ${hostStatusToLabel(h.status)}`,
          })) ?? [],
      ),
    [hostList, hasPermission, t],
  );

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!hostId) {
      toast.error(t("scheduledOperationForm.selectTargetHost"));
      return;
    }
    const triggerMsg = buildTrigger();
    if (!triggerMsg) return;

    let operation;
    switch (kind) {
      case "START_HOST":
        operation = create(ScheduledOperationSchema, {
          operation: {
            case: "startHost",
            value: create(ScheduledStartHostOperationSchema, {
              hostId,
              withWorldRestart,
            }),
          },
        });
        break;
      case "RESTART_HOST":
        // ホスト詳細の「再起動」と同じく、最新版に更新して再起動する.
        operation = create(ScheduledOperationSchema, {
          operation: {
            case: "restartHost",
            value: create(RestartHeadlessHostRequestSchema, {
              hostId,
              withUpdate: true,
              withWorldRestart,
            }),
          },
        });
        break;
      case "SHUTDOWN_HOST":
        operation = create(ScheduledOperationSchema, {
          operation: {
            case: "shutdownHost",
            value: create(ShutdownHeadlessHostRequestSchema, { hostId }),
          },
        });
        break;
    }

    try {
      await mutateAsync({ operation, trigger: triggerMsg });
      toast.success(t("scheduledOperationForm.scheduleCreated"));
      navigate("/scheduled");
    } catch (err) {
      toast.error(
        t("scheduledOperationForm.createError", {
          message: (err as Error).message,
        }),
      );
    }
  };

  return (
    <form className="space-y-4" onSubmit={onSubmit}>
      <SelectField
        label={t("scheduledOperationForm.targetHost")}
        options={hostOptions}
        selectedId={hostId || "_"}
        onChange={(o) => setHostId(o.id === "_" ? "" : o.id)}
      />
      {kind !== "SHUTDOWN_HOST" && (
        <CheckboxField
          label={t("scheduledOperationForm.withWorldRestart")}
          checked={withWorldRestart}
          onCheckedChange={(v) => setWithWorldRestart(v === true)}
        />
      )}
      <p className="text-sm text-muted-foreground">
        {t(
          kind === "START_HOST"
            ? "scheduledOperationForm.startHostNote"
            : kind === "RESTART_HOST"
              ? "scheduledOperationForm.restartHostNote"
              : "scheduledOperationForm.shutdownHostNote",
        )}
      </p>
      <FormFooter
        navigate={navigate}
        disabled={isPending}
        cancelDisabled={isPending}
      />
    </form>
  );
}

/* ============================================================
 * STOP / UPDATE_* / SEND_DYNAMIC_IMPULSE action: target session + 操作ごとのフィールド
 * ============================================================ */

const makeOtherFormSchema = (t: TFunction) =>
  z.object({
    // action target
    sessionId: z
      .string()
      .min(1, t("scheduledOperationForm.selectTargetSession")),

    // UPDATE_PARAMETERS
    updName: z.string().optional(),
    updDescription: z.string().optional(),
    updTags: z.string().optional(),
    updMaxUsers: z.string().optional(),
    updAccessLevel: z.string().optional(),
    updHideFromPublicListing: z.string().optional(),
    updAwayKickMinutes: z.string().optional(),
    updIdleRestartIntervalSeconds: z.string().optional(),
    updSaveOnExit: z.string().optional(),
    updAutoSaveIntervalSeconds: z.string().optional(),
    updAutoSleep: z.string().optional(),

    // UPDATE_EXTRA_SETTINGS
    extraAutoUpgrade: z.string().optional(),
    extraMemo: z.string().optional(),

    // SEND_DYNAMIC_IMPULSE
    impulseTag: z.string().optional(),
    impulseValueType: z.enum(IMPULSE_VALUE_TYPES).optional(),
    impulseValue: z.string().optional(),
  });

type OtherFormValues = z.infer<ReturnType<typeof makeOtherFormSchema>>;

const parseIntOrUndef = (s?: string) => {
  if (!s) return undefined;
  const n = parseInt(s, 10);
  return Number.isFinite(n) ? n : undefined;
};

const parseFloatOrUndef = (s?: string) => {
  if (!s) return undefined;
  const n = parseFloat(s);
  return Number.isFinite(n) ? n : undefined;
};

const parseBoolOrUndef = (s?: string) => {
  if (s === "true") return true;
  if (s === "false") return false;
  return undefined;
};

function OtherKindActionForm({
  kind,
  sessionOptions,
  hasRunningSessions,
  defaultSessionId,
  buildTrigger,
}: {
  kind: Exclude<OperationKind, "START_SESSION" | HostOperationKind>;
  sessionOptions: SessionOption[];
  hasRunningSessions: boolean;
  defaultSessionId?: string;
  buildTrigger: () => ScheduledTrigger | null;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { mutateAsync, isPending } = useMutation(
    createScheduledSessionOperation,
  );
  const otherFormSchema = useMemo(() => makeOtherFormSchema(t), [t]);
  const triBoolOptions = useMemo(() => makeTriBoolOptions(t), [t]);

  const {
    control,
    handleSubmit,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<OtherFormValues>({
    resolver: zodResolver(otherFormSchema),
    mode: "onBlur",
    defaultValues: {
      sessionId: defaultSessionId ?? "",
      impulseValueType: "none",
    },
  });
  const impulseValueType = watch("impulseValueType") ?? "none";

  const onSubmit = handleSubmit(async (values) => {
    const triggerMsg = buildTrigger();
    if (!triggerMsg) return;

    try {
      let operation;
      switch (kind) {
        case "STOP_SESSION":
          operation = create(ScheduledOperationSchema, {
            operation: {
              case: "stopSession",
              value: create(StopSessionRequestSchema, {
                sessionId: values.sessionId,
              }),
            },
          });
          break;
        case "UPDATE_PARAMETERS": {
          const tagsList = values.updTags
            ?.split(",")
            .map((t) => t.trim())
            .filter((t) => t);
          const maxUsers = parseIntOrUndef(values.updMaxUsers);
          const accessLevel = parseIntOrUndef(values.updAccessLevel);
          const hideFromPublicListing = parseBoolOrUndef(
            values.updHideFromPublicListing,
          );
          const awayKickMinutes = parseFloatOrUndef(values.updAwayKickMinutes);
          const idleRestartIntervalSeconds = parseIntOrUndef(
            values.updIdleRestartIntervalSeconds,
          );
          const saveOnExit = parseBoolOrUndef(values.updSaveOnExit);
          const autoSaveIntervalSeconds = parseIntOrUndef(
            values.updAutoSaveIntervalSeconds,
          );
          const autoSleep = parseBoolOrUndef(values.updAutoSleep);

          const innerInit: Record<string, unknown> = {
            sessionId: values.sessionId,
          };
          if (values.updName) innerInit.name = values.updName;
          if (values.updDescription)
            innerInit.description = values.updDescription;
          if (tagsList && tagsList.length > 0) {
            innerInit.updateTags = true;
            innerInit.tags = tagsList;
          }
          if (maxUsers !== undefined) innerInit.maxUsers = maxUsers;
          if (accessLevel !== undefined)
            innerInit.accessLevel = accessLevel as AccessLevel;
          if (hideFromPublicListing !== undefined)
            innerInit.hideFromPublicListing = hideFromPublicListing;
          if (awayKickMinutes !== undefined)
            innerInit.awayKickMinutes = awayKickMinutes;
          if (idleRestartIntervalSeconds !== undefined)
            innerInit.idleRestartIntervalSeconds = idleRestartIntervalSeconds;
          if (saveOnExit !== undefined) innerInit.saveOnExit = saveOnExit;
          if (autoSaveIntervalSeconds !== undefined)
            innerInit.autoSaveIntervalSeconds = autoSaveIntervalSeconds;
          if (autoSleep !== undefined) innerInit.autoSleep = autoSleep;

          operation = create(ScheduledOperationSchema, {
            operation: {
              case: "updateParameters",
              value: create(HdlUpdateSessionParametersRequestSchema, {
                parameters: create(
                  UpdateSessionParametersRequestSchema,
                  innerInit,
                ),
              }),
            },
          });
          break;
        }
        case "SEND_DYNAMIC_IMPULSE": {
          const tag = values.impulseTag?.trim();
          if (!tag) {
            toast.error(t("scheduledOperationForm.specifyImpulseTag"));
            return;
          }
          const value = buildImpulseValue(
            values.impulseValueType ?? "none",
            values.impulseValue ?? "",
          );
          operation = create(ScheduledOperationSchema, {
            operation: {
              case: "sendDynamicImpulse",
              value: create(HdlSendDynamicImpulseRequestSchema, {
                parameters: create(SendDynamicImpulseRequestSchema, {
                  sessionId: values.sessionId,
                  tag,
                  value,
                }),
              }),
            },
          });
          break;
        }
        case "UPDATE_EXTRA_SETTINGS": {
          const autoUpgrade = parseBoolOrUndef(values.extraAutoUpgrade);
          operation = create(ScheduledOperationSchema, {
            operation: {
              case: "updateExtraSettings",
              value: create(UpdateSessionExtraSettingsRequestSchema, {
                sessionId: values.sessionId,
                ...(autoUpgrade !== undefined ? { autoUpgrade } : {}),
                ...(values.extraMemo ? { memo: values.extraMemo } : {}),
              }),
            },
          });
          break;
        }
      }

      await mutateAsync({ operation, trigger: triggerMsg });
      toast.success(t("scheduledOperationForm.scheduleCreated"));
      navigate("/scheduled");
    } catch (err) {
      toast.error(
        t("scheduledOperationForm.createError", {
          message: (err as Error).message,
        }),
      );
    }
  });

  return (
    <form className="space-y-4" onSubmit={onSubmit}>
      <Controller
        name="sessionId"
        control={control}
        render={({ field }) => (
          <SelectField
            label={t("scheduledOperationForm.targetSession")}
            options={sessionOptions}
            selectedId={field.value || "_"}
            onChange={(o) => field.onChange(o.id === "_" ? "" : o.id)}
            error={errors.sessionId?.message}
            helperText={
              hasRunningSessions
                ? undefined
                : t("scheduledOperationForm.noRunningSessionsHelper")
            }
          />
        )}
      />

      {kind === "UPDATE_PARAMETERS" && (
        <div className="space-y-4 border-t pt-4">
          <p className="text-sm text-muted-foreground">
            {t("scheduledOperationForm.unchangedFieldsNote")}
          </p>
          <Controller
            name="updName"
            control={control}
            render={({ field }) => (
              <TextField
                label={t("scheduledOperationForm.sessionName")}
                {...field}
              />
            )}
          />
          <Controller
            name="updDescription"
            control={control}
            render={({ field }) => (
              <TextareaField label={t("common.description")} {...field} />
            )}
          />
          <Controller
            name="updTags"
            control={control}
            render={({ field }) => (
              <TextField
                label={t("scheduledOperationForm.tags")}
                helperText={t("scheduledOperationForm.commaSeparatedUnchanged")}
                {...field}
              />
            )}
          />
          <Controller
            name="updMaxUsers"
            control={control}
            render={({ field }) => (
              <TextField
                label={t("scheduledOperationForm.maxUsers")}
                type="number"
                {...field}
                value={field.value ?? ""}
              />
            )}
          />
          <Controller
            name="updAccessLevel"
            control={control}
            render={({ field }) => (
              <SelectField
                label={t("scheduledOperationForm.accessLevel")}
                options={[
                  {
                    id: "_",
                    label: t("scheduledOperationForm.keepUnchanged"),
                  },
                ].concat(
                  AccessLevels.map((l) => ({
                    id: String(l.value),
                    label: t(l.labelKey),
                  })),
                )}
                selectedId={field.value || "_"}
                onChange={(o) => field.onChange(o.id === "_" ? "" : o.id)}
              />
            )}
          />
          <Controller
            name="updHideFromPublicListing"
            control={control}
            render={({ field }) => (
              <SelectField
                label={t("scheduledOperationForm.hideFromPublicListing")}
                options={triBoolOptions}
                selectedId={field.value || "_"}
                onChange={(o) => field.onChange(o.id === "_" ? "" : o.id)}
              />
            )}
          />
          <Controller
            name="updAwayKickMinutes"
            control={control}
            render={({ field }) => (
              <TextField
                label={t("scheduledOperationForm.awayKickMinutes")}
                type="number"
                helperText={t("scheduledOperationForm.disableWithMinusOne")}
                {...field}
                value={field.value ?? ""}
              />
            )}
          />
          <Controller
            name="updIdleRestartIntervalSeconds"
            control={control}
            render={({ field }) => (
              <TextField
                label={t("scheduledOperationForm.idleRestartIntervalSeconds")}
                type="number"
                helperText={t("scheduledOperationForm.disableWithMinusOne")}
                {...field}
                value={field.value ?? ""}
              />
            )}
          />
          <Controller
            name="updSaveOnExit"
            control={control}
            render={({ field }) => (
              <SelectField
                label={t("scheduledOperationForm.saveOnExit")}
                options={triBoolOptions}
                selectedId={field.value || "_"}
                onChange={(o) => field.onChange(o.id === "_" ? "" : o.id)}
              />
            )}
          />
          <Controller
            name="updAutoSaveIntervalSeconds"
            control={control}
            render={({ field }) => (
              <TextField
                label={t("scheduledOperationForm.autoSaveIntervalSeconds")}
                type="number"
                helperText={t("scheduledOperationForm.disableWithMinusOne")}
                {...field}
                value={field.value ?? ""}
              />
            )}
          />
          <Controller
            name="updAutoSleep"
            control={control}
            render={({ field }) => (
              <SelectField
                label={t("scheduledOperationForm.autoSleep")}
                options={triBoolOptions}
                selectedId={field.value || "_"}
                onChange={(o) => field.onChange(o.id === "_" ? "" : o.id)}
              />
            )}
          />
        </div>
      )}

      {kind === "SEND_DYNAMIC_IMPULSE" && (
        <div className="space-y-4 border-t pt-4">
          <Controller
            name="impulseTag"
            control={control}
            render={({ field }) => (
              <TextField
                label="Tag"
                placeholder={t("sessionOpsMenu.tagPlaceholder")}
                {...field}
                value={field.value ?? ""}
              />
            )}
          />
          <Controller
            name="impulseValueType"
            control={control}
            render={({ field }) => (
              <RadioGroupField
                label={t("sessionOpsMenu.valueTypeLabel")}
                options={IMPULSE_VALUE_TYPES.map((v) => ({
                  label: v,
                  value: v,
                }))}
                value={field.value ?? "none"}
                onValueChange={field.onChange}
                className="flex flex-row flex-wrap gap-4"
              />
            )}
          />
          {impulseValueType !== "none" && (
            <Controller
              name="impulseValue"
              control={control}
              render={({ field }) => (
                <TextField
                  label={t("sessionOpsMenu.valueLabel", {
                    type: impulseValueType,
                  })}
                  type={impulseValueType === "string" ? "text" : "number"}
                  {...field}
                  value={field.value ?? ""}
                />
              )}
            />
          )}
        </div>
      )}

      {kind === "UPDATE_EXTRA_SETTINGS" && (
        <div className="space-y-4 border-t pt-4">
          <p className="text-sm text-muted-foreground">
            {t("scheduledOperationForm.unchangedFieldsNote")}
          </p>
          <Controller
            name="extraAutoUpgrade"
            control={control}
            render={({ field }) => (
              <SelectField
                label={t("scheduledOperationForm.autoUpgrade")}
                options={triBoolOptions}
                selectedId={field.value || "_"}
                onChange={(o) => field.onChange(o.id === "_" ? "" : o.id)}
              />
            )}
          />
          <Controller
            name="extraMemo"
            control={control}
            render={({ field }) => (
              <TextareaField
                label={t("scheduledOperationForm.adminMemo")}
                {...field}
              />
            )}
          />
        </div>
      )}

      <FormFooter
        navigate={navigate}
        disabled={isPending || isSubmitting || Object.keys(errors).length > 0}
        cancelDisabled={isPending || isSubmitting}
      />
    </form>
  );
}

function FormFooter({
  navigate,
  disabled,
  cancelDisabled,
}: {
  navigate: ReturnType<typeof useNavigate>;
  disabled: boolean;
  cancelDisabled: boolean;
}) {
  const { t } = useTranslation();
  return (
    <div className="sticky bottom-0 border-t p-4 mt-8 bg-background flex gap-2">
      <Button type="submit" disabled={disabled}>
        {t("scheduledOperationForm.createSchedule")}
      </Button>
      <Button
        type="button"
        variant="outline"
        onClick={() => navigate(-1)}
        disabled={cancelDisabled}
      >
        {t("common.cancel")}
      </Button>
    </div>
  );
}
