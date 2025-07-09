import { useQuery } from "@connectrpc/connect-query";
import { Skeleton } from "./ui";
import { Link } from "react-router";
import { getHeadlessHost } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";

export default function HostTip({ hostId }: { hostId?: string }) {
  const { data, isPending } = useQuery(getHeadlessHost, {
    hostId,
  });

  return (
    <span>
      {isPending ? (
        <Skeleton className="h-4 w-32" />
      ) : (
        <Link to={`/hosts/${data?.host?.id}`} className="hover:underline">
          {data?.host?.name} {`(${data?.host?.accountName})`}
        </Link>
      )}
    </span>
  );
}
