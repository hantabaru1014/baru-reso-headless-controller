import {
  Avatar,
  AvatarFallback,
  AvatarImage,
  Button,
  DropdownMenu,
  DropdownMenuItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "./ui";
import { UserInfo } from "@/atoms/sessionAtom";
import { resolveUrl } from "@/libs/skyfrostUtils";
import { LogOut } from "lucide-react";

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
          <Avatar className="h-8 w-8">
            <AvatarImage src={resolveUrl(user?.image)} alt={user?.name} />
            <AvatarFallback>{user?.name?.charAt(0) || "U"}</AvatarFallback>
          </Avatar>
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
