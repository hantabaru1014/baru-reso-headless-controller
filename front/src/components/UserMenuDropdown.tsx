import {
  Avatar,
  AvatarFallback,
  AvatarImage,
  Button,
  DropdownMenu,
  DropdownMenuItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./ui";
import { UserInfo } from "@/atoms/sessionAtom";
import { resolveUrl } from "@/libs/skyfrostUtils";
import { LogOut, Settings } from "lucide-react";
import { useNavigate } from "react-router";

export function UserMenuDropdown({
  user,
  signOut,
}: {
  user?: UserInfo;
  signOut: () => void;
}) {
  const navigate = useNavigate();

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
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => navigate("/settings")}>
          <Settings className="h-4 w-4" />
          <span>設定</span>
        </DropdownMenuItem>
        <DropdownMenuItem onClick={signOut}>
          <LogOut className="h-4 w-4" />
          <span>サインアウト</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
