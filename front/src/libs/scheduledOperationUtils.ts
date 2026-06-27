import { Timestamp, timestampFromDate } from "@bufbuild/protobuf/wkt";
import { format } from "date-fns";
import { ScheduledOperationStatus } from "../../pbgen/hdlctrl/v1/controller_pb";

export type OperationKind =
  | "START_SESSION"
  | "STOP_SESSION"
  | "UPDATE_PARAMETERS"
  | "UPDATE_EXTRA_SETTINGS";

export const operationKindLabel = (k: OperationKind): string => {
  switch (k) {
    case "START_SESSION":
      return "セッション開始";
    case "STOP_SESSION":
      return "セッション終了";
    case "UPDATE_PARAMETERS":
      return "セッション設定変更";
    case "UPDATE_EXTRA_SETTINGS":
      return "追加設定変更";
  }
};

export const scheduledOperationStatusToLabel = (
  s: ScheduledOperationStatus,
): string => {
  switch (s) {
    case ScheduledOperationStatus.PENDING:
      return "予約済み";
    case ScheduledOperationStatus.RUNNING:
      return "実行中";
    case ScheduledOperationStatus.SUCCEEDED:
      return "完了";
    case ScheduledOperationStatus.FAILED:
      return "失敗";
    case ScheduledOperationStatus.CANCELED:
      return "キャンセル";
    default:
      return "不明";
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
