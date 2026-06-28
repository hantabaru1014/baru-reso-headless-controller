import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  createScheduledSessionOperation,
  listHeadlessHost,
  searchSessions,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  HeadlessHostStatus,
  ScheduledOperationSchema,
  ScheduledTrigger,
  ScheduledTriggerSchema,
  SessionStatus,
  SessionUserCountTriggerSchema,
  SessionUserCountTrigger_Comparator,
  StartWorldRequestSchema,
  StopSessionRequestSchema,
  TimeTriggerSchema,
  UpdateSessionExtraSettingsRequestSchema,
  UpdateSessionParametersRequestSchema as HdlUpdateSessionParametersRequestSchema,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import {
  AccessLevel,
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
import { RadioGroupField, SelectField, TextField, TextareaField } from "./base";
import {
  dateToTimestamp,
  defaultScheduledAtInputValue,
  localDateTimeStringToDate,
  operationKindLabel,
  OperationKind,
  TriggerKind,
  triggerKindLabel,
  UserCountComparator,
  userCountComparatorLabel,
} from "../libs/scheduledOperationUtils";
import { AccessLevels } from "../constants";
import {
  buildStartWorldParameters,
  DEFAULT_SESSION_FORM_VALUES,
  sessionFormSchema,
  SessionFormValues,
} from "../libs/sessionFormUtils";
import SessionStartupFields from "./SessionStartupFields";

type Props = {
  /** プリセレクト用: セッション詳細から開いたとき (トリガー監視/操作対象の両方の初期値になる) */
  defaultSessionId?: string;
  /** トリガー / アクション種別の初期値. セッション詳細の「ユーザー0人で停止」から開かれた場合等. */
  defaultTrigger?: TriggerKind;
  defaultOperation?: OperationKind;
  /** SESSION_USER_COUNT trigger の初期値. */
  defaultUserCountComparator?: UserCountComparator;
  defaultUserCountThreshold?: number;
};

const TRI_BOOL_OPTIONS = [
  { id: "_", label: "変更しない" },
  { id: "true", label: "はい" },
  { id: "false", label: "いいえ" },
];

const TRIGGER_OPTIONS: { label: string; value: TriggerKind }[] = [
  { label: triggerKindLabel("TIME"), value: "TIME" },
  {
    label: triggerKindLabel("SESSION_USER_COUNT"),
    value: "SESSION_USER_COUNT",
  },
];

const COMPARATOR_OPTIONS: { id: UserCountComparator; label: string }[] = [
  {
    id: "LESS_OR_EQUAL",
    label: userCountComparatorLabel("LESS_OR_EQUAL"),
  },
  {
    id: "GREATER_OR_EQUAL",
    label: userCountComparatorLabel("GREATER_OR_EQUAL"),
  },
];

const ACTION_OPTIONS: { label: string; value: OperationKind }[] = (
  [
    "START_SESSION",
    "STOP_SESSION",
    "UPDATE_PARAMETERS",
    "UPDATE_EXTRA_SETTINGS",
  ] as OperationKind[]
).map((k) => ({ label: operationKindLabel(k), value: k }));

type SessionOption = { id: string; label: string };

/** session list を取得して dropdown 用に整形する hook. trigger / action 両方で共有して 1 回だけ fetch. */
function useSessionOptions(defaultSessionId: string | undefined): {
  options: SessionOption[];
  hasRunningSessions: boolean;
} {
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
    const options: SessionOption[] = [{ id: "_", label: "選択..." }].concat(
      merged.map((s) => ({
        id: s.id,
        label: `${s.name || "(no name)"} (${s.id.slice(0, 8)}…)`,
      })),
    );
    return { options, hasRunningSessions: merged.length > 0 };
  }, [sessionsData, defaultSessionId]);
}

export default function ScheduledOperationForm({
  defaultSessionId,
  defaultTrigger,
  defaultOperation,
  defaultUserCountComparator,
  defaultUserCountThreshold,
}: Props) {
  const [trigger, setTrigger] = useState<TriggerKind>(defaultTrigger ?? "TIME");
  const [kind, setKind] = useState<OperationKind>(
    defaultOperation ?? (defaultSessionId ? "STOP_SESSION" : "START_SESSION"),
  );

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

  const { options: sessionOptions, hasRunningSessions } =
    useSessionOptions(defaultSessionId);

  /**
   * 現在のトリガー設定を ScheduledTrigger proto に変換する.
   * 失敗時は toast を出して null を返す (action の submit handler から呼ばれる前提).
   */
  const buildTrigger = (): ScheduledTrigger | null => {
    if (trigger === "TIME") {
      if (!scheduledAt) {
        toast.error("実行日時を指定してください");
        return null;
      }
      const at = localDateTimeStringToDate(scheduledAt);
      if (Number.isNaN(at.getTime())) {
        toast.error("実行日時が不正です");
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
    if (!monitorSessionId) {
      toast.error("監視対象セッションを選択してください");
      return null;
    }
    const parsed = parseIntOrUndef(threshold);
    if (parsed === undefined || parsed < 0) {
      toast.error("ユーザー数のしきい値は 0 以上の整数で指定してください");
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
        <h3 className="text-sm font-semibold">1. トリガー条件</h3>
        <RadioGroupField
          label="どんな時に予約を発火させるか"
          options={TRIGGER_OPTIONS}
          value={trigger}
          onValueChange={(v) => setTrigger(v as TriggerKind)}
          className="flex flex-row flex-wrap gap-4"
        />

        {trigger === "TIME" ? (
          <TextField
            label="実行日時"
            type="datetime-local"
            value={scheduledAt}
            onChange={(e) => setScheduledAt(e.target.value)}
          />
        ) : (
          <div className="space-y-3">
            <SelectField
              label="監視対象セッション"
              options={sessionOptions}
              selectedId={monitorSessionId || "_"}
              onChange={(o) => setMonitorSessionId(o.id === "_" ? "" : o.id)}
              helperText={
                hasRunningSessions
                  ? undefined
                  : "実行中のセッションがありません. セッションIDが分かっている場合は URL の ?sessionId= から事前指定できます."
              }
            />
            <div className="flex gap-2">
              <TextField
                label="ユーザー数"
                type="number"
                className="w-32"
                value={threshold}
                onChange={(e) => setThreshold(e.target.value)}
              />
              <SelectField
                label="条件"
                options={COMPARATOR_OPTIONS}
                selectedId={comparator}
                onChange={(o) => setComparator(o.id as UserCountComparator)}
              />
            </div>
          </div>
        )}
      </section>

      <section className="space-y-3 rounded-md border p-4">
        <h3 className="text-sm font-semibold">2. その時に何が起こるか</h3>
        <RadioGroupField
          label="操作種別"
          options={ACTION_OPTIONS}
          value={kind}
          onValueChange={(v) => setKind(v as OperationKind)}
          className="flex flex-row flex-wrap gap-4"
        />

        {kind === "START_SESSION" ? (
          <StartSessionActionForm buildTrigger={buildTrigger} />
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
  const navigate = useNavigate();
  const { mutateAsync, isPending } = useMutation(
    createScheduledSessionOperation,
  );

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
      toast.success("予約を作成しました");
      navigate("/sessions/scheduled");
    } catch (err) {
      toast.error(`作成失敗: ${(err as Error).message}`);
    }
  });

  return (
    <form className="space-y-4" onSubmit={onSubmit}>
      <Controller
        name="hostId"
        control={control}
        render={({ field }) => (
          <SelectField
            label="ホスト"
            options={[{ id: "_", label: "選択..." }].concat(
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
 * STOP / UPDATE_* action: target session + 任意の update params
 * ============================================================ */

const otherFormSchema = z.object({
  // action target
  sessionId: z.string().min(1, "対象セッションを選択してください"),

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
});

type OtherFormValues = z.infer<typeof otherFormSchema>;

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
  kind: Exclude<OperationKind, "START_SESSION">;
  sessionOptions: SessionOption[];
  hasRunningSessions: boolean;
  defaultSessionId?: string;
  buildTrigger: () => ScheduledTrigger | null;
}) {
  const navigate = useNavigate();
  const { mutateAsync, isPending } = useMutation(
    createScheduledSessionOperation,
  );

  const {
    control,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<OtherFormValues>({
    resolver: zodResolver(otherFormSchema),
    mode: "onBlur",
    defaultValues: {
      sessionId: defaultSessionId ?? "",
    },
  });

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
      toast.success("予約を作成しました");
      navigate("/sessions/scheduled");
    } catch (err) {
      toast.error(`作成失敗: ${(err as Error).message}`);
    }
  });

  return (
    <form className="space-y-4" onSubmit={onSubmit}>
      <Controller
        name="sessionId"
        control={control}
        render={({ field }) => (
          <SelectField
            label="操作対象セッション"
            options={sessionOptions}
            selectedId={field.value || "_"}
            onChange={(o) => field.onChange(o.id === "_" ? "" : o.id)}
            error={errors.sessionId?.message}
            helperText={
              hasRunningSessions
                ? undefined
                : "実行中のセッションがありません. セッションIDが分かっている場合は URL の ?sessionId= から事前指定できます."
            }
          />
        )}
      />

      {kind === "UPDATE_PARAMETERS" && (
        <div className="space-y-4 border-t pt-4">
          <p className="text-sm text-muted-foreground">
            空欄/「変更しない」のフィールドはそのままです
          </p>
          <Controller
            name="updName"
            control={control}
            render={({ field }) => (
              <TextField label="セッション名" {...field} />
            )}
          />
          <Controller
            name="updDescription"
            control={control}
            render={({ field }) => <TextareaField label="説明" {...field} />}
          />
          <Controller
            name="updTags"
            control={control}
            render={({ field }) => (
              <TextField
                label="タグ"
                helperText="カンマ区切り。空欄なら変更しない"
                {...field}
              />
            )}
          />
          <Controller
            name="updMaxUsers"
            control={control}
            render={({ field }) => (
              <TextField
                label="最大ユーザー数"
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
                label="アクセスレベル"
                options={[{ id: "_", label: "変更しない" }].concat(
                  AccessLevels.map((l) => ({
                    id: String(l.value),
                    label: l.label,
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
                label="セッションリストから隠す"
                options={TRI_BOOL_OPTIONS}
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
                label="AFK キック時間 (分)"
                type="number"
                helperText="-1 で無効"
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
                label="アイドル時の自動再起動間隔 (秒)"
                type="number"
                helperText="-1 で無効"
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
                label="セッション終了時に保存"
                options={TRI_BOOL_OPTIONS}
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
                label="自動保存間隔 (秒)"
                type="number"
                helperText="-1 で無効"
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
                label="オートスリープ"
                options={TRI_BOOL_OPTIONS}
                selectedId={field.value || "_"}
                onChange={(o) => field.onChange(o.id === "_" ? "" : o.id)}
              />
            )}
          />
        </div>
      )}

      {kind === "UPDATE_EXTRA_SETTINGS" && (
        <div className="space-y-4 border-t pt-4">
          <p className="text-sm text-muted-foreground">
            空欄/「変更しない」のフィールドはそのままです
          </p>
          <Controller
            name="extraAutoUpgrade"
            control={control}
            render={({ field }) => (
              <SelectField
                label="自動アップデート"
                options={TRI_BOOL_OPTIONS}
                selectedId={field.value || "_"}
                onChange={(o) => field.onChange(o.id === "_" ? "" : o.id)}
              />
            )}
          />
          <Controller
            name="extraMemo"
            control={control}
            render={({ field }) => (
              <TextareaField label="管理者メモ" {...field} />
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
  return (
    <div className="sticky bottom-0 border-t p-4 mt-8 bg-background flex gap-2">
      <Button type="submit" disabled={disabled}>
        予約を作成
      </Button>
      <Button
        type="button"
        variant="outline"
        onClick={() => navigate(-1)}
        disabled={cancelDisabled}
      >
        キャンセル
      </Button>
    </div>
  );
}
