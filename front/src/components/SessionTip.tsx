import { useQuery } from "@connectrpc/connect-query";
import { Skeleton } from "./ui";
import { Link } from "react-router";
import { getSessionDetails } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";

export default function SessionTip({ sessionId }: { sessionId?: string }) {
  const { data, isPending } = useQuery(
    getSessionDetails,
    { sessionId },
    { enabled: !!sessionId },
  );

  if (!sessionId) {
    return <span>-</span>;
  }

  const name = data?.session?.name || data?.session?.startupParameters?.name;

  return (
    <span>
      {isPending ? (
        <Skeleton className="h-4 w-32" />
      ) : (
        <Link
          to={`/sessions/${sessionId}`}
          className="hover:underline"
          title={sessionId}
        >
          {name ? `${name} (${sessionId.slice(0, 8)}…)` : sessionId}
        </Link>
      )}
    </span>
  );
}
