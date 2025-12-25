import {
  Button,
  DropdownMenu,
  DropdownMenuItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "./ui";
import { UserInfo } from "@/atoms/sessionAtom";
import { LogOut } from "lucide-react";
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
        <DropdownMenuItem onClick={signOut}>
          <LogOut className="h-4 w-4" />
          <span>サインアウト</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
