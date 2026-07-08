import { HeadlessHostStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import i18n from "@/libs/i18n";

export const hostStatusToLabel = (status: HeadlessHostStatus) => {
  switch (status) {
    case HeadlessHostStatus.STARTING:
      return i18n.t("hostUtils.starting");
    case HeadlessHostStatus.RUNNING:
      return i18n.t("hostUtils.running");
    case HeadlessHostStatus.STOPPING:
      return i18n.t("hostUtils.stopping");
    case HeadlessHostStatus.EXITED:
      return i18n.t("hostUtils.exited");
    case HeadlessHostStatus.CRASHED:
      return i18n.t("hostUtils.crashed");
    default:
      return i18n.t("hostUtils.unknown");
  }
};
