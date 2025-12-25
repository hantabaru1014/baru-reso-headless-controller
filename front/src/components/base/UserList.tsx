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
}: {
  data: UserInfo[];
  isLoading?: boolean;
  renderActions?: (user: UserInfo) => React.ReactNode;
  onUserClick?: (user: UserInfo) => void;
}) {
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
                  alt={`${user.name}のアイコン`}
                />
                <span className="text-sm font-medium">{user.name}</span>
              </div>
              {renderActions && renderActions(user)}
            </div>
          ))}
    </div>
  );
}
