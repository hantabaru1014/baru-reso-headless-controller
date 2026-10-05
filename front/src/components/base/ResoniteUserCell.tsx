import { useQuery } from "@connectrpc/connect-query";
import { ReactNode } from "react";
import { getResoniteUser } from "../../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { ResoniteUserIcon } from "../ResoniteUserIcon";
import { Skeleton } from "../ui";
import { cn } from "@/libs/cssUtils";

const sizeClasses = {
  // テーブルのセル用 (UserCell と同じ大きさ)
  sm: {
    root: "gap-2",
    icon: "size-6",
    name: "text-xs",
    nameSkeleton: "h-3",
    id: "text-[10px]",
  },
  // ユーザー選択リスト用 (メンバー追加モーダルの登録済みユーザー行と同じ大きさ)
  md: {
    root: "gap-3",
    icon: "size-8",
    name: "font-mono text-sm",
    nameSkeleton: "h-4",
    id: "text-xs",
  },
};

/**
 * Resonite ID から GetResoniteUser でユーザー名・アイコンを取得して表示する.
 * アカウント登録前の招待中ユーザーなど、users に行が無い相手の表示に使う.
 */
export function ResoniteUserCell({
  resoniteId,
  badge,
  size = "sm",
  iconClassName,
}: {
  resoniteId: string;
  /** ユーザー名の横に表示する要素 (ステータスバッジ等) */
  badge?: ReactNode;
  size?: keyof typeof sizeClasses;
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
  const c = sizeClasses[size];
  const icon = iconClassName ?? c.icon;

  return (
    <div className={cn("flex items-center min-w-0", c.root)}>
      {isPending ? (
        <Skeleton className={cn("rounded-full", icon)} />
      ) : (
        <ResoniteUserIcon
          iconUrl={data?.iconUrl}
          alt={data?.name ?? resoniteId}
          className={icon}
        />
      )}
      <div className="flex flex-col min-w-0">
        <div className="flex items-center gap-1 min-w-0">
          {isPending ? (
            <Skeleton className={cn("w-24", c.nameSkeleton)} />
          ) : (
            <span className={cn("truncate", c.name)} title={data?.name}>
              {data?.name || resoniteId}
            </span>
          )}
          {badge}
        </div>
        <span
          className={cn("text-muted-foreground font-mono truncate", c.id)}
          title={resoniteId}
        >
          {resoniteId}
        </span>
      </div>
    </div>
  );
}
