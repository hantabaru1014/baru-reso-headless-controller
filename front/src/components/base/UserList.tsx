import { Avatar, AvatarFallback, AvatarImage, Skeleton } from "../ui";

export type UserInfo = {
  id: string;
  name: string;
  iconUrl: string;
};

export function UserList({
  data,
  isLoading,
  renderActions,
}: {
  data: UserInfo[];
  isLoading?: boolean;
  renderActions?: (user: UserInfo) => React.ReactNode;
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
              className="flex items-center justify-between space-x-3 p-2 rounded-md hover:bg-accent"
            >
              <div className="flex items-center space-x-3">
                <Avatar>
                  <AvatarImage
                    src={user.iconUrl}
                    alt={`${user.name}のアイコン`}
                  />
                  <AvatarFallback>{user.name.charAt(0)}</AvatarFallback>
                </Avatar>
                <span className="text-sm font-medium">{user.name}</span>
              </div>
              {renderActions && renderActions(user)}
            </div>
          ))}
    </div>
  );
}
