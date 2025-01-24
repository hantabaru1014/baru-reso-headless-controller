import {
  Avatar,
  List,
  ListItem,
  ListItemAvatar,
  ListItemText,
  Skeleton,
} from "@mui/material";
import { UserInfo } from "../../../pbgen/headless/v1/headless_pb";

export default function UserList({
  data,
  isLoading,
  renderActions,
}: {
  data: UserInfo[];
  isLoading?: boolean;
  renderActions?: (user: UserInfo) => React.ReactNode;
}) {
  return (
    <List>
      {isLoading
        ? Array.from({ length: 3 }, (_, i) => (
            <ListItem key={i}>
              <Skeleton variant="circular" />
            </ListItem>
          ))
        : data.map((user) => (
            <ListItem
              key={user.id}
              secondaryAction={renderActions && renderActions(user)}
            >
              <ListItemAvatar>
                <Avatar alt={`${user.name}のアイコン`} src={user.iconUrl} />
              </ListItemAvatar>
              <ListItemText primary={user.name} />
            </ListItem>
          ))}
    </List>
  );
}
