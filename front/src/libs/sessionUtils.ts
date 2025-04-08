import { SessionStatus } from "../../pbgen/hdlctrl/v1/controller_pb";

export const sessionStatusToLabel = (status: SessionStatus) => {
  switch (status) {
    case SessionStatus.STARTING:
      return "開始中";
    case SessionStatus.RUNNING:
      return "実行中";
    case SessionStatus.ENDED:
      return "終了済み";
    case SessionStatus.CRASHED:
      return "クラッシュ";
    default:
      return "不明";
  }
};
