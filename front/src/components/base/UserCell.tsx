import { useQuery } from "@connectrpc/connect-query";
import { getUser } from "../../../pbgen/hdlctrl/v1/user-UserService_connectquery";
import { ResoniteUserIcon } from "../ResoniteUserIcon";
import { Skeleton } from "../ui";

/**
 * user_id から GetUser でユーザー情報を取得し、アイコン + ID を表示する.
 *
 * TanStack Query のキャッシュにより同じ user_id への複数同時呼び出しは自動的に
 * 重複排除される (request deduplication). 取得結果は 5 分間キャッシュされ、
 * 同一ページ内で同じユーザーを複数箇所に表示しても 1 リクエストで済む.
 */
export function UserCell({
  userId,
  showId = true,
  iconClassName,
}: {
  userId: string;
  /** ユーザー ID 文字列も表示するか (デフォルト true) */
  showId?: boolean;
  iconClassName?: string;
}) {
  const { data, isPending, isError } = useQuery(
    getUser,
    { userId },
    {
      enabled: !!userId,
      staleTime: 5 * 60 * 1000,
      retry: false,
    },
  );

  const user = data?.user;

  return (
    <div className="flex items-center gap-2 min-w-0">
      {isPending ? (
        <Skeleton className={iconClassName ?? "size-6 rounded-full"} />
      ) : (
        <ResoniteUserIcon
          iconUrl={user?.iconUrl}
          alt={user?.id ?? userId}
          className={iconClassName ?? "size-6"}
        />
      )}
      <div className="flex flex-col min-w-0">
        {isPending ? (
          <Skeleton className="h-3 w-24" />
        ) : isError || !user ? (
          <span
            className="font-mono text-xs text-muted-foreground truncate"
            title={userId}
          >
            {userId}
          </span>
        ) : (
          <>
            {showId && (
              <span className="font-mono text-xs truncate" title={user.id}>
                {user.id}
              </span>
            )}
            <span
              className="text-[10px] text-muted-foreground font-mono truncate"
              title={user.resoniteId}
            >
              {user.resoniteId}
            </span>
          </>
        )}
      </div>
    </div>
  );
}
