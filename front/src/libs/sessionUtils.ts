import { SessionStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import i18n from "@/libs/i18n";

export const sessionStatusToLabel = (status: SessionStatus) => {
  switch (status) {
    case SessionStatus.STARTING:
      return i18n.t("sessionUtils.starting");
    case SessionStatus.RUNNING:
      return i18n.t("sessionUtils.running");
    case SessionStatus.ENDED:
      return i18n.t("sessionUtils.ended");
    case SessionStatus.CRASHED:
      return i18n.t("sessionUtils.crashed");
    default:
      return i18n.t("sessionUtils.unknown");
  }
};

export const IMPULSE_VALUE_TYPES = ["none", "string", "int", "float"] as const;

export type ImpulseValueType = (typeof IMPULSE_VALUE_TYPES)[number];

/** DynamicImpulse の値の種類と入力文字列から、SendDynamicImpulseRequest.value を組み立てる. */
export const buildImpulseValue = (
  type: ImpulseValueType,
  raw: string,
):
  | { case: "stringValue"; value: string }
  | { case: "intValue"; value: number }
  | { case: "floatValue"; value: number }
  | undefined => {
  switch (type) {
    case "string":
      return { case: "stringValue", value: raw };
    case "int":
      return { case: "intValue", value: parseInt(raw, 10) || 0 };
    case "float":
      return { case: "floatValue", value: parseFloat(raw) || 0 };
    case "none":
      return undefined;
  }
};
