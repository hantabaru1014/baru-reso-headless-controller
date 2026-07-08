import { ComponentProps, ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button, Tooltip, TooltipContent, TooltipTrigger } from "../ui";

/**
 * パーミッション判定の結果でボタンを非活性化し、無効な理由をツールチップで表示する.
 *
 * `allowed=true` の場合は通常の `<Button>` と同じ挙動.
 * `allowed=false` の場合は disabled になり、hover で `disabledReason` を表示する.
 */
export function PermissionGuardedButton({
  allowed,
  disabledReason,
  children,
  disabled,
  ...buttonProps
}: ComponentProps<typeof Button> & {
  allowed: boolean;
  disabledReason?: ReactNode;
}) {
  const { t } = useTranslation();
  const resolvedDisabledReason =
    disabledReason ?? t("permissionGuardedButton.noPermission");
  const isDisabled = disabled || !allowed;
  const button = (
    <Button {...buttonProps} disabled={isDisabled}>
      {children}
    </Button>
  );

  if (allowed) return button;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        {/* span でラップしないと disabled なボタンは hover を拾わないため */}
        <span className="inline-block">{button}</span>
      </TooltipTrigger>
      <TooltipContent>{resolvedDisabledReason}</TooltipContent>
    </Tooltip>
  );
}
