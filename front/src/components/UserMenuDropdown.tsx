import { Link } from "react-router";
import {
  Button,
  DropdownMenu,
  DropdownMenuItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./ui";
import { UserInfo } from "@/atoms/sessionAtom";
import { LogOut, Settings } from "lucide-react";
import { ResoniteUserIcon } from "./ResoniteUserIcon";

export function UserMenuDropdown({
  user,
  signOut,
}: {
  user?: UserInfo;
  signOut: () => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon">
          <ResoniteUserIcon
            iconUrl={user?.image}
            alt={user?.name}
            className="h-8 w-8"
          />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuLabel>
          <div className="font-medium">{user?.name}</div>
          <div className="text-muted-foreground">{user?.email}</div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/user-settings">
            <Settings className="h-4 w-4" />
            <span>ユーザー設定</span>
          </Link>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={signOut}>
          <LogOut className="h-4 w-4" />
          <span>サインアウト</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
