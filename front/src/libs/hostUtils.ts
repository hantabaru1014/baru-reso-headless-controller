import { HeadlessHostStatus } from "../../pbgen/hdlctrl/v1/controller_pb";

export const hostStatusToLabel = (status: HeadlessHostStatus) => {
  switch (status) {
    case HeadlessHostStatus.STARTING:
      return "起動中...";
    case HeadlessHostStatus.RUNNING:
      return "実行中";
    case HeadlessHostStatus.STOPPING:
      return "停止中...";
    case HeadlessHostStatus.EXITED:
      return "停止済み";
    case HeadlessHostStatus.CRASHED:
      return "クラッシュ";
    default:
      return "不明";
  }
};
