import { useQuery } from "@connectrpc/connect-query";
import { ReactNode } from "react";
import { getResoniteUser } from "../../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { ResoniteUserIcon } from "../ResoniteUserIcon";
import { Skeleton } from "../ui";

/**
 * Resonite ID から GetResoniteUser でユーザー名・アイコンを取得して表示する.
 * アカウント登録前の招待中ユーザーなど、users に行が無い相手の表示に使う.
 */
export function ResoniteUserCell({
  resoniteId,
  badge,
  iconClassName,
}: {
  resoniteId: string;
  /** ユーザー名の横に表示する要素 (ステータスバッジ等) */
  badge?: ReactNode;
  iconClassName?: string;
}) {
  const { data, isPending } = useQuery(
    getResoniteUser,
    { resoniteId },
    {
      enabled: !!resoniteId,
      staleTime: 5 * 60 * 1000,
      retry: false,
    },
  );

  return (
    <div className="flex items-center gap-2 min-w-0">
      {isPending ? (
        <Skeleton className={iconClassName ?? "size-6 rounded-full"} />
      ) : (
        <ResoniteUserIcon
          iconUrl={data?.iconUrl}
          alt={data?.name ?? resoniteId}
          className={iconClassName ?? "size-6"}
        />
      )}
      <div className="flex flex-col min-w-0">
        <div className="flex items-center gap-1 min-w-0">
          {isPending ? (
            <Skeleton className="h-3 w-24" />
          ) : (
            <span className="text-xs truncate" title={data?.name}>
              {data?.name || resoniteId}
            </span>
          )}
          {badge}
        </div>
        <span
          className="text-[10px] text-muted-foreground font-mono truncate"
          title={resoniteId}
        >
          {resoniteId}
        </span>
      </div>
    </div>
  );
}
