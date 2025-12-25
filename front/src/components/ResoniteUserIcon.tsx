import { Avatar, AvatarFallback, AvatarImage } from "./ui";
import { DEFAULT_USER_ICON_URL, resolveUrl } from "@/libs/skyfrostUtils";

export function ResoniteUserIcon({
  iconUrl,
  alt,
  className,
}: {
  iconUrl?: string;
  alt?: string;
  className?: string;
}) {
  return (
    <Avatar className={className}>
      <AvatarImage src={resolveUrl(iconUrl)} alt={alt} />
      <AvatarFallback>
        <img
          src={DEFAULT_USER_ICON_URL}
          alt="デフォルトユーザーアイコン"
          className="size-full"
        />
      </AvatarFallback>
    </Avatar>
  );
}
