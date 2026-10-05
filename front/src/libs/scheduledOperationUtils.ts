import { Timestamp, timestampFromDate } from "@bufbuild/protobuf/wkt";
import { format } from "date-fns";
import {
  CronTrigger,
  CronTrigger_Frequency,
  ScheduledOperation,
  ScheduledOperationStatus,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import i18n from "@/libs/i18n";

const HOST_OPERATION_KINDS = [
  "START_HOST",
  "RESTART_HOST",
  "SHUTDOWN_HOST",
] as const;

/** フォームに並べる順の操作種別. */
export const OPERATION_KINDS = [
  "START_SESSION",
  "STOP_SESSION",
  "UPDATE_PARAMETERS",
  "UPDATE_EXTRA_SETTINGS",
  "SEND_DYNAMIC_IMPULSE",
  ...HOST_OPERATION_KINDS,
] as const;

export type OperationKind = (typeof OPERATION_KINDS)[number];

export type HostOperationKind = (typeof HOST_OPERATION_KINDS)[number];

export const isHostOperationKind = (k: OperationKind): k is HostOperationKind =>
  (HOST_OPERATION_KINDS as readonly string[]).includes(k);

export const isOperationKind = (v: string | null): v is OperationKind =>
  (OPERATION_KINDS as readonly (string | null)[]).includes(v);

export const operationCaseToKind = (
  op?: ScheduledOperation,
): OperationKind | undefined => {
  switch (op?.operation.case) {
    case "startSession":
      return "START_SESSION";
    case "stopSession":
      return "STOP_SESSION";
    case "updateParameters":
      return "UPDATE_PARAMETERS";
    case "updateExtraSettings":
      return "UPDATE_EXTRA_SETTINGS";
    case "sendDynamicImpulse":
      return "SEND_DYNAMIC_IMPULSE";
    case "startHost":
      return "START_HOST";
    case "restartHost":
      return "RESTART_HOST";
    case "shutdownHost":
      return "SHUTDOWN_HOST";
    default:
      return undefined;
  }
};

export const operationKindLabel = (k: OperationKind): string => {
  switch (k) {
    case "START_SESSION":
      return i18n.t("scheduledOperationUtils.operationKind.startSession");
    case "STOP_SESSION":
      return i18n.t("scheduledOperationUtils.operationKind.stopSession");
    case "UPDATE_PARAMETERS":
      return i18n.t("scheduledOperationUtils.operationKind.updateParameters");
    case "UPDATE_EXTRA_SETTINGS":
      return i18n.t(
        "scheduledOperationUtils.operationKind.updateExtraSettings",
      );
    case "SEND_DYNAMIC_IMPULSE":
      return i18n.t("scheduledOperationUtils.operationKind.sendDynamicImpulse");
    case "START_HOST":
      return i18n.t("scheduledOperationUtils.operationKind.startHost");
    case "RESTART_HOST":
      return i18n.t("scheduledOperationUtils.operationKind.restartHost");
    case "SHUTDOWN_HOST":
      return i18n.t("scheduledOperationUtils.operationKind.shutdownHost");
  }
};

/** フォームに並べる順のトリガー種別. */
export const TRIGGER_KINDS = ["TIME", "CRON", "SESSION_USER_COUNT"] as const;

export type TriggerKind = (typeof TRIGGER_KINDS)[number];

export const isTriggerKind = (v: string | null): v is TriggerKind =>
  (TRIGGER_KINDS as readonly (string | null)[]).includes(v);

export const triggerKindLabel = (t: TriggerKind): string => {
  switch (t) {
    case "TIME":
      return i18n.t("scheduledOperationUtils.triggerKind.time");
    case "CRON":
      return i18n.t("scheduledOperationUtils.triggerKind.cron");
    case "SESSION_USER_COUNT":
      return i18n.t("scheduledOperationUtils.triggerKind.sessionUserCount");
  }
};

export const CRON_FREQUENCIES = [
  "DAILY",
  "WEEKLY",
  "MONTHLY",
  "YEARLY",
] as const satisfies readonly (keyof typeof CronTrigger_Frequency)[];

export type CronFrequency = (typeof CRON_FREQUENCIES)[number];

export const cronFrequencyLabel = (f: CronFrequency): string =>
  i18n.t(`scheduledOperationUtils.cronFrequency.${f.toLowerCase()}`);

// Intl.DateTimeFormat の生成は重いので、言語 + オプションごとに使い回す.
const dateFormatCache = new Map<string, Intl.DateTimeFormat>();
const cachedDateFormat = (options: Intl.DateTimeFormatOptions) => {
  const key = `${i18n.language}:${JSON.stringify(options)}`;
  let f = dateFormatCache.get(key);
  if (!f) {
    f = new Intl.DateTimeFormat(i18n.language, options);
    dateFormatCache.set(key, f);
  }
  return f;
};

/** 0=日曜 ... 6=土曜 の曜日名 (短縮形, 現在の言語). */
export const weekdayShortLabel = (w: number): string =>
  // 2023-01-01 は日曜日.
  cachedDateFormat({ weekday: "short" }).format(new Date(2023, 0, 1 + w));

export const monthLabel = (m: number): string =>
  cachedDateFormat({ month: "long" }).format(new Date(2023, m - 1, 1));

/** ブラウザのタイムゾーン (IANA 名). Cron trigger の基準にする. */
export const browserTimeZone = (): string =>
  Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";

/** Cron trigger を「毎週 月・木 04:30」のような文に整形する. */
export const describeCronTrigger = (c: CronTrigger): string => {
  const time = `${String(c.hour).padStart(2, "0")}:${String(c.minute).padStart(2, "0")}`;
  switch (c.frequency) {
    case CronTrigger_Frequency.DAILY:
      return i18n.t("scheduledOperationUtils.cronDescription.daily", { time });
    case CronTrigger_Frequency.WEEKLY:
      return i18n.t("scheduledOperationUtils.cronDescription.weekly", {
        time,
        weekdays: [...c.weekdays]
          .sort((a, b) => a - b)
          .map(weekdayShortLabel)
          .join(i18n.t("scheduledOperationUtils.cronDescription.separator")),
      });
    case CronTrigger_Frequency.MONTHLY:
      return i18n.t("scheduledOperationUtils.cronDescription.monthly", {
        time,
        day: c.dayOfMonth,
      });
    case CronTrigger_Frequency.YEARLY:
      return i18n.t("scheduledOperationUtils.cronDescription.yearly", {
        time,
        month: monthLabel(c.month),
        day: c.dayOfMonth,
      });
    default:
      return "-";
  }
};

export type UserCountComparator = "LESS_OR_EQUAL" | "GREATER_OR_EQUAL";

export const userCountComparatorLabel = (c: UserCountComparator): string => {
  switch (c) {
    case "LESS_OR_EQUAL":
      return i18n.t("scheduledOperationUtils.comparator.lessOrEqual");
    case "GREATER_OR_EQUAL":
      return i18n.t("scheduledOperationUtils.comparator.greaterOrEqual");
  }
};

export const scheduledOperationStatusToLabel = (
  s: ScheduledOperationStatus,
): string => {
  switch (s) {
    case ScheduledOperationStatus.PENDING:
      return i18n.t("scheduledOperationUtils.status.pending");
    case ScheduledOperationStatus.RUNNING:
      return i18n.t("scheduledOperationUtils.status.running");
    case ScheduledOperationStatus.SUCCEEDED:
      return i18n.t("scheduledOperationUtils.status.succeeded");
    case ScheduledOperationStatus.FAILED:
      return i18n.t("scheduledOperationUtils.status.failed");
    case ScheduledOperationStatus.CANCELED:
      return i18n.t("scheduledOperationUtils.status.canceled");
    default:
      return i18n.t("scheduledOperationUtils.status.unknown");
  }
};

// "YYYY-MM-DDTHH:mm" (datetime-local の value 形式) を Date に変換.
export const localDateTimeStringToDate = (s: string): Date => new Date(s);

// Date を datetime-local input の value にフォーマット.
export const dateToLocalDateTimeString = (d: Date): string =>
  format(d, "yyyy-MM-dd'T'HH:mm");

export const dateToTimestamp = (d: Date): Timestamp => timestampFromDate(d);

export const timestampToDate = (t?: Timestamp): Date | undefined => {
  if (!t?.seconds) {
    return undefined;
  }
  return new Date(Number(t.seconds * 1000n));
};

export const formatScheduled = (t?: Timestamp): string => {
  const d = timestampToDate(t);
  if (!d) return "";
  return format(d, "yyyy/MM/dd HH:mm");
};

/** datetime-local input の初期値: 今から10分後 (秒は0). */
export const defaultScheduledAtInputValue = (): string => {
  const d = new Date();
  d.setMinutes(d.getMinutes() + 10);
  d.setSeconds(0, 0);
  return dateToLocalDateTimeString(d);
};
