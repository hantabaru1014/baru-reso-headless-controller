import { useSearchParams } from "react-router";
import ScheduledOperationForm from "../../../components/ScheduledOperationForm";

export default function ScheduledOperationNew() {
  const [params] = useSearchParams();
  const defaultSessionId = params.get("sessionId") ?? undefined;

  return (
    <div className="container mx-auto p-4">
      <ScheduledOperationForm defaultSessionId={defaultSessionId} />
    </div>
  );
}
