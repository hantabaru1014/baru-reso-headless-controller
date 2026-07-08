import { useMutation } from "@connectrpc/connect-query";
import { useEffect } from "react";
import { toast } from "sonner";
import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
} from "./ui";
import { issueResoniteLinkConnection } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { formatTimestamp } from "../libs/datetimeUtils";
import { useTranslation } from "react-i18next";

export function ResoniteLinkConnectionDialog({
  sessionId,
  open,
  onOpenChange,
}: {
  sessionId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  const { mutate, data, isPending, reset } = useMutation(
    issueResoniteLinkConnection,
    {
      onError: (e) =>
        toast.error(
          t("resoniteLinkConnectionDialog.issueFailed", { error: e.message }),
        ),
    },
  );

  const wsUrl = data
    ? `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}${data.wsPath}`
    : "";

  useEffect(() => {
    if (open && !data && !isPending) {
      mutate({ sessionId });
    }
    if (!open) {
      reset();
    }
  }, [open, data, isPending, mutate, reset, sessionId]);

  const handleCopy = () => {
    if (!wsUrl) return;
    navigator.clipboard.writeText(wsUrl);
    toast.success(t("resoniteLinkConnectionDialog.urlCopied"));
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>{t("resoniteLinkConnectionDialog.title")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          <p className="text-sm text-muted-foreground">
            {t("resoniteLinkConnectionDialog.description")}
          </p>
          <div className="flex space-x-2">
            <Input
              value={wsUrl}
              readOnly
              placeholder={
                isPending ? t("resoniteLinkConnectionDialog.issuing") : ""
              }
              className="font-mono text-xs"
            />
            <Button variant="outline" onClick={handleCopy} disabled={!wsUrl}>
              {t("resoniteLinkConnectionDialog.copy")}
            </Button>
          </div>
          {data?.expiresAt && (
            <p className="text-xs text-muted-foreground">
              {t("resoniteLinkConnectionDialog.expiresAt", {
                time: formatTimestamp(data.expiresAt),
              })}
            </p>
          )}
        </div>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => mutate({ sessionId })}
            disabled={isPending}
          >
            {t("resoniteLinkConnectionDialog.reissue")}
          </Button>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            {t("resoniteLinkConnectionDialog.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
