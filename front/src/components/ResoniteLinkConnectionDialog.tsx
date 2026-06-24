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

export function ResoniteLinkConnectionDialog({
  sessionId,
  open,
  onOpenChange,
}: {
  sessionId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { mutate, data, isPending, reset } = useMutation(
    issueResoniteLinkConnection,
    {
      onError: (e) => toast.error(`接続URLの発行に失敗しました: ${e.message}`),
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
    toast.success("接続URLをコピーしました");
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>ResoniteLink 接続URL</DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          <p className="text-sm text-muted-foreground">
            ResoniteLink クライアントから接続するための一時 URL です。
            トークンには有効期限があります。
          </p>
          <div className="flex space-x-2">
            <Input
              value={wsUrl}
              readOnly
              placeholder={isPending ? "発行中..." : ""}
              className="font-mono text-xs"
            />
            <Button variant="outline" onClick={handleCopy} disabled={!wsUrl}>
              コピー
            </Button>
          </div>
          {data?.expiresAt && (
            <p className="text-xs text-muted-foreground">
              有効期限: {formatTimestamp(data.expiresAt)}
            </p>
          )}
        </div>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => mutate({ sessionId })}
            disabled={isPending}
          >
            再発行
          </Button>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            閉じる
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
