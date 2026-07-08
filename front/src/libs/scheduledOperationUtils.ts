import { Timestamp, timestampFromDate } from "@bufbuild/protobuf/wkt";
import { format } from "date-fns";
import { ScheduledOperationStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import i18n from "@/libs/i18n";

export type OperationKind =
  | "START_SESSION"
  | "STOP_SESSION"
  | "UPDATE_PARAMETERS"
  | "UPDATE_EXTRA_SETTINGS";

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
  }
};

export type TriggerKind = "TIME" | "SESSION_USER_COUNT";

export const triggerKindLabel = (t: TriggerKind): string => {
  switch (t) {
    case "TIME":
      return i18n.t("scheduledOperationUtils.triggerKind.time");
    case "SESSION_USER_COUNT":
      return i18n.t("scheduledOperationUtils.triggerKind.sessionUserCount");
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
