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
