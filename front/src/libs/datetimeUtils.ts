import { Timestamp } from "@bufbuild/protobuf/wkt";
import { format } from "date-fns";

export const formatTimestamp = (value?: Timestamp) => {
  if (!value?.seconds) {
    return "";
  }
  const date = new Date(Number(value.seconds * 1000n));
  return format(date, "yyyy/MM/dd HH:mm:ss");
};
