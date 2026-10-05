import { Timestamp, timestampDate } from "@bufbuild/protobuf/wkt";
import { format } from "date-fns";

export const formatTimestamp = (value?: Timestamp) => {
  if (!value?.seconds) {
    return "";
  }
  const date = new Date(Number(value.seconds * 1000n));
  return format(date, "yyyy/MM/dd HH:mm:ss");
};

/** value が現在時刻より前なら true (未設定なら false). */
export const isPastTimestamp = (value?: Timestamp) =>
  !!value?.seconds && timestampDate(value).getTime() < Date.now();
