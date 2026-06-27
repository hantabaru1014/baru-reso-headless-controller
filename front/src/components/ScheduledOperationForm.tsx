import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  createScheduledSessionOperation,
  listHeadlessHost,
  searchSessions,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  HeadlessHostStatus,
  ScheduledOperationSchema,
  ScheduledTriggerSchema,
  SessionStatus,
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
  /** プリセレクト用: セッション詳細から開いたとき */
  defaultSessionId?: string;
};

const TRI_BOOL_OPTIONS = [
  { id: "_", label: "変更しない" },
  { id: "true", label: "はい" },
  { id: "false", label: "いいえ" },
];

const KIND_OPTIONS: { label: string; value: OperationKind }[] = [
  "START_SESSION",
  "STOP_SESSION",
  "UPDATE_PARAMETERS",
  "UPDATE_EXTRA_SETTINGS",
].map((k) => ({
  label: operationKindLabel(k as OperationKind),
  value: k as OperationKind,
}));

export default function ScheduledOperationForm({ defaultSessionId }: Props) {
  const [kind, setKind] = useState<OperationKind>(
    defaultSessionId ? "STOP_SESSION" : "START_SESSION",
  );

  return (
    <div className="space-y-4">
      <RadioGroupField
        label="操作種別"
        options={KIND_OPTIONS.map((o) => ({ label: o.label, value: o.value }))}
        value={kind}
        onValueChange={(v) => setKind(v as OperationKind)}
        className="flex flex-row flex-wrap gap-4"
      />

      {kind === "START_SESSION" ? (
        <StartSessionScheduleForm />
      ) : (
        <OtherKindScheduleForm
          kind={kind}
          defaultSessionId={defaultSessionId}
        />
      )}
    </div>
  );
}

/* ============================================================
 * START_SESSION 予約: NewSessionForm と同等のフィールド + 実行日時
 * ============================================================ */

function StartSessionScheduleForm() {
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

  const [scheduledAt, setScheduledAt] = useState(
    defaultScheduledAtInputValue(),
  );
  const [scheduledAtError, setScheduledAtError] = useState<string | undefined>(
    undefined,
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
    if (!scheduledAt) {
      setScheduledAtError("実行日時を指定してください");
      return;
    }
    const at = localDateTimeStringToDate(scheduledAt);
    if (Number.isNaN(at.getTime())) {
      setScheduledAtError("実行日時が不正です");
      return;
    }
    setScheduledAtError(undefined);

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
      await mutateAsync({ operation, trigger });
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

      <TextField
        label="実行日時"
        type="datetime-local"
        value={scheduledAt}
        onChange={(e) => setScheduledAt(e.target.value)}
        error={scheduledAtError}
      />

      <div className="sticky bottom-0 border-t p-4 mt-8 bg-background flex gap-2">
        <Button
          type="submit"
          disabled={isPending || isSubmitting || Object.keys(errors).length > 0}
        >
          予約を作成
        </Button>
        <Button
          type="button"
          variant="outline"
          onClick={() => navigate(-1)}
          disabled={isPending || isSubmitting}
        >
          キャンセル
        </Button>
      </div>
    </form>
  );
}

/* ============================================================
 * STOP / UPDATE_* 予約: シンプルな別フォーム
 * ============================================================ */

const otherFormSchema = z.object({
  scheduledAt: z.string().min(1, "実行日時を指定してください"),
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

function OtherKindScheduleForm({
  kind,
  defaultSessionId,
}: {
  kind: Exclude<OperationKind, "START_SESSION">;
  defaultSessionId?: string;
}) {
  const navigate = useNavigate();
  const { mutateAsync, isPending } = useMutation(
    createScheduledSessionOperation,
  );

  const { data: sessionsData } = useQuery(searchSessions, {
    parameters: { status: SessionStatus.RUNNING },
    page: { pageIndex: 0, pageSize: 100 },
  });

  const sessionOptions = useMemo(() => {
    const list = sessionsData?.sessions ?? [];
    const merged = list.slice();
    if (defaultSessionId && !list.find((s) => s.id === defaultSessionId)) {
      merged.unshift({
        id: defaultSessionId,
        name: defaultSessionId,
      } as (typeof list)[number]);
    }
    return merged.map((s) => ({
      id: s.id,
      label: `${s.name || "(no name)"} (${s.id.slice(0, 8)}…)`,
    }));
  }, [sessionsData, defaultSessionId]);

  const {
    control,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<OtherFormValues>({
    resolver: zodResolver(otherFormSchema),
    mode: "onBlur",
    defaultValues: {
      scheduledAt: defaultScheduledAtInputValue(),
      sessionId: defaultSessionId ?? "",
    },
  });

  const onSubmit = handleSubmit(async (values) => {
    try {
      const at = localDateTimeStringToDate(values.scheduledAt);
      if (Number.isNaN(at.getTime())) {
        toast.error("実行日時が不正です");
        return;
      }

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

      const trigger = create(ScheduledTriggerSchema, {
        trigger: {
          case: "time",
          value: create(TimeTriggerSchema, {
            scheduledAt: dateToTimestamp(at),
          }),
        },
      });

      await mutateAsync({ operation, trigger });
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
            label="対象セッション"
            options={[{ id: "_", label: "選択..." }].concat(sessionOptions)}
            selectedId={field.value || "_"}
            onChange={(o) => field.onChange(o.id === "_" ? "" : o.id)}
            error={errors.sessionId?.message}
          />
        )}
      />

      <Controller
        name="scheduledAt"
        control={control}
        render={({ field }) => (
          <TextField
            label="実行日時"
            type="datetime-local"
            error={errors.scheduledAt?.message}
            {...field}
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

      <div className="sticky bottom-0 border-t p-4 mt-8 bg-background flex gap-2">
        <Button
          type="submit"
          disabled={isPending || isSubmitting || Object.keys(errors).length > 0}
        >
          予約を作成
        </Button>
        <Button
          type="button"
          variant="outline"
          onClick={() => navigate(-1)}
          disabled={isPending || isSubmitting}
        >
          キャンセル
        </Button>
      </div>
    </form>
  );
}
