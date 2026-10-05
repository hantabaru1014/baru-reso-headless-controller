import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { useTranslation } from "react-i18next";
import { Badge } from "../ui";
import { isPastTimestamp } from "../../libs/datetimeUtils";

/** アカウント登録前 (招待中) のユーザーを示すバッジ. 招待リンクの期限切れも表す. */
export function InvitedBadge({ expiresAt }: { expiresAt?: Timestamp }) {
  const { t } = useTranslation();
  const expired = isPastTimestamp(expiresAt);
  return (
    <Badge
      variant={expired ? "destructive" : "secondary"}
      className="shrink-0 px-1.5 py-0 text-[10px]"
      title={expired ? t("invitedBadge.expiredHint") : undefined}
    >
      {expired ? t("invitedBadge.expired") : t("invitedBadge.invited")}
    </Badge>
  );
}
