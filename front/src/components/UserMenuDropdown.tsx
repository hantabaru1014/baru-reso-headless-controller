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
import { Check, Languages, LogOut, Settings } from "lucide-react";
import { ResoniteUserIcon } from "./ResoniteUserIcon";
import { useTranslation } from "react-i18next";

const languages = [
  { code: "ja", label: "日本語" },
  { code: "en", label: "English" },
] as const;

export function UserMenuDropdown({
  user,
  signOut,
}: {
  user?: UserInfo;
  signOut: () => void;
}) {
  const { t, i18n } = useTranslation();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon">
          <ResoniteUserIcon
            iconUrl={user?.iconUrl}
            alt={user?.resoniteName}
            className="h-8 w-8"
          />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuLabel>
          <div className="font-medium">{user?.resoniteName}</div>
          <div className="text-muted-foreground">{user?.id}</div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/user-settings">
            <Settings className="h-4 w-4" />
            <span>{t("userMenu.userSettings")}</span>
          </Link>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuLabel className="flex items-center gap-2 text-muted-foreground">
          <Languages className="h-4 w-4" />
          <span>{t("userMenu.language")}</span>
        </DropdownMenuLabel>
        {languages.map((lang) => (
          <DropdownMenuItem
            key={lang.code}
            onClick={() => i18n.changeLanguage(lang.code)}
          >
            <span className="w-4">
              {i18n.resolvedLanguage === lang.code && (
                <Check className="h-4 w-4" />
              )}
            </span>
            <span>{lang.label}</span>
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={signOut}>
          <LogOut className="h-4 w-4" />
          <span>{t("userMenu.signOut")}</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
