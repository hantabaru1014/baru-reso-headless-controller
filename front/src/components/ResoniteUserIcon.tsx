import { Avatar, AvatarFallback, AvatarImage } from "./ui";
import { DEFAULT_USER_ICON_URL, resolveUrl } from "@/libs/skyfrostUtils";
import { useTranslation } from "react-i18next";

export function ResoniteUserIcon({
  iconUrl,
  alt,
  className,
}: {
  iconUrl?: string;
  alt?: string;
  className?: string;
}) {
  const { t } = useTranslation();
  return (
    <Avatar className={className}>
      <AvatarImage src={resolveUrl(iconUrl)} alt={alt} />
      <AvatarFallback>
        <img
          src={DEFAULT_USER_ICON_URL}
          alt={t("resoniteUserIcon.defaultUserIcon")}
          className="size-full"
        />
      </AvatarFallback>
    </Avatar>
  );
}
