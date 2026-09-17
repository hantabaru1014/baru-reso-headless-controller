import { useTranslation } from "react-i18next";
import { ResoniteUserIcon } from "../ResoniteUserIcon";
import { Skeleton } from "../ui";

export type UserInfo = {
  id: string;
  name: string;
  iconUrl: string;
};

export function UserList({
  data,
  isLoading,
  renderActions,
  onUserClick,
  showId = false,
}: {
  data: UserInfo[];
  isLoading?: boolean;
  renderActions?: (user: UserInfo) => React.ReactNode;
  onUserClick?: (user: UserInfo) => void;
  showId?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <div className="space-y-2">
      {isLoading
        ? Array.from({ length: 3 }, (_, i) => (
            <div key={i} className="flex items-center space-x-3 p-2">
              <Skeleton className="h-10 w-10 rounded-full" />
              <Skeleton className="h-4 w-24" />
            </div>
          ))
        : data.map((user) => (
            <div
              key={user.id}
              className={`flex items-center justify-between space-x-3 p-2 rounded-md hover:bg-accent ${onUserClick ? "cursor-pointer" : ""}`}
              onClick={onUserClick ? () => onUserClick(user) : undefined}
            >
              <div className="flex items-center space-x-3">
                <ResoniteUserIcon
                  iconUrl={user.iconUrl}
                  alt={t("userList.iconAlt", { name: user.name })}
                />
                <div className="min-w-0">
                  <div className="text-sm font-medium">{user.name}</div>
                  {showId && (
                    <div className="text-muted-foreground font-mono text-xs">
                      {user.id}
                    </div>
                  )}
                </div>
              </div>
              {renderActions && renderActions(user)}
            </div>
          ))}
    </div>
  );
}
