import { useQuery } from "@connectrpc/connect-query";
import { Skeleton } from "./ui";
import { Link } from "react-router";
import { getHeadlessHost } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";

export default function HostTip({ hostId }: { hostId?: string }) {
  const { data, isPending } = useQuery(
    getHeadlessHost,
    { hostId },
    { enabled: !!hostId },
  );

  if (!hostId) {
    return <span>-</span>;
  }

  const host = data?.host;

  return (
    <span>
      {isPending ? (
        <Skeleton className="h-4 w-32" />
      ) : (
        // 削除済みホストでは host が引けない (ジョブ履歴や予約操作は対象ホストが
        // 消えた後も残る). リンク先は引数の hostId を使い、表示は生 ID に落とす.
        <Link
          to={`/hosts/${hostId}`}
          className="hover:underline"
          title={hostId}
        >
          {host?.name ? `${host.name} (${host.accountName})` : hostId}
        </Link>
      )}
    </span>
  );
}
