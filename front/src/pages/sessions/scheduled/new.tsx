import { useSearchParams } from "react-router";
import ScheduledOperationForm from "../../../components/ScheduledOperationForm";
import {
  OperationKind,
  TriggerKind,
  UserCountComparator,
} from "../../../libs/scheduledOperationUtils";

const isTriggerKind = (v: string | null): v is TriggerKind =>
  v === "TIME" || v === "SESSION_USER_COUNT";

const isOperationKind = (v: string | null): v is OperationKind =>
  v === "START_SESSION" ||
  v === "STOP_SESSION" ||
  v === "UPDATE_PARAMETERS" ||
  v === "UPDATE_EXTRA_SETTINGS";

const isComparator = (v: string | null): v is UserCountComparator =>
  v === "LESS_OR_EQUAL" || v === "GREATER_OR_EQUAL";

export default function ScheduledOperationNew() {
  const [params] = useSearchParams();
  const defaultSessionId = params.get("sessionId") ?? undefined;

  const triggerRaw = params.get("trigger");
  const operationRaw = params.get("operation");
  const comparatorRaw = params.get("comparator");
  const thresholdRaw = params.get("threshold");

  const defaultTrigger = isTriggerKind(triggerRaw) ? triggerRaw : undefined;
  const defaultOperation = isOperationKind(operationRaw)
    ? operationRaw
    : undefined;
  const defaultUserCountComparator = isComparator(comparatorRaw)
    ? comparatorRaw
    : undefined;
  const parsedThreshold = thresholdRaw ? Number(thresholdRaw) : NaN;
  const defaultUserCountThreshold =
    Number.isFinite(parsedThreshold) && parsedThreshold >= 0
      ? parsedThreshold
      : undefined;

  return (
    <div className="container mx-auto p-4">
      <ScheduledOperationForm
        defaultSessionId={defaultSessionId}
        defaultTrigger={defaultTrigger}
        defaultOperation={defaultOperation}
        defaultUserCountComparator={defaultUserCountComparator}
        defaultUserCountThreshold={defaultUserCountThreshold}
      />
    </div>
  );
}
