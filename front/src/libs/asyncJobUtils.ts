import {
  AsyncJob,
  AsyncJobStatus,
  AsyncJobType,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import i18n from "@/libs/i18n";

export const asyncJobTypeToLabel = (t: AsyncJobType): string => {
  switch (t) {
    case AsyncJobType.START_HOST:
      return i18n.t("asyncJobUtils.jobType.startHost");
    case AsyncJobType.SHUTDOWN_HOST:
      return i18n.t("asyncJobUtils.jobType.shutdownHost");
    case AsyncJobType.RESTART_HOST:
      return i18n.t("asyncJobUtils.jobType.restartHost");
    case AsyncJobType.START_SESSION:
      return i18n.t("asyncJobUtils.jobType.startSession");
    case AsyncJobType.STOP_SESSION:
      return i18n.t("asyncJobUtils.jobType.stopSession");
    case AsyncJobType.BUILD_IMAGE:
      return i18n.t("asyncJobUtils.jobType.buildImage");
    default:
      return i18n.t("asyncJobUtils.jobType.unknown");
  }
};

export const asyncJobStatusToLabel = (s: AsyncJobStatus): string => {
  switch (s) {
    case AsyncJobStatus.PENDING:
      return i18n.t("asyncJobUtils.status.pending");
    case AsyncJobStatus.RUNNING:
      return i18n.t("asyncJobUtils.status.running");
    case AsyncJobStatus.SUCCEEDED:
      return i18n.t("asyncJobUtils.status.succeeded");
    case AsyncJobStatus.FAILED:
      return i18n.t("asyncJobUtils.status.failed");
    default:
      return i18n.t("asyncJobUtils.status.unknown");
  }
};

/** まだ完了していない (ポーリングを続けるべき) job か. */
export const isAsyncJobActive = (job: AsyncJob): boolean =>
  job.status === AsyncJobStatus.PENDING ||
  job.status === AsyncJobStatus.RUNNING;

export type AsyncJobTarget = { hostId?: string; sessionId?: string };

/**
 * result_payload (完了時の戻り値 JSON) から発番された ID を取り出す.
 * backend 由来だが JSON として壊れている可能性は握りつぶす.
 */
const parseResultTarget = (resultPayload: string): AsyncJobTarget => {
  try {
    const payload: unknown = JSON.parse(resultPayload);
    if (typeof payload !== "object" || payload === null) {
      return {};
    }
    const { host_id: hostId, session_id: sessionId } = payload as Record<
      string,
      unknown
    >;
    return {
      hostId: typeof hostId === "string" && hostId ? hostId : undefined,
      sessionId:
        typeof sessionId === "string" && sessionId ? sessionId : undefined,
    };
  } catch {
    return {};
  }
};

/**
 * 一覧の「対象」表示に使う host_id / session_id を解決する.
 *
 * 成功した job では result_payload を優先する. START_SESSION は投入時点で
 * host_id が入っている一方、成功後は result_payload に発番された session_id が
 * 入るので、優先しないと「起動したセッション」へ辿れないため.
 * START_HOST は投入時に host_id が空で result_payload 側にだけ入る.
 */
export const resolveAsyncJobTarget = (job: AsyncJob): AsyncJobTarget => {
  if (job.status === AsyncJobStatus.SUCCEEDED && job.resultPayload) {
    const fromResult = parseResultTarget(job.resultPayload);
    if (fromResult.hostId || fromResult.sessionId) {
      return fromResult;
    }
  }
  return { hostId: job.hostId, sessionId: job.sessionId };
};
