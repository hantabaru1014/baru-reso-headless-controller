import { useQuery } from "@connectrpc/connect-query";
import { useState } from "react";
import {
  Button,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogClose,
} from "./ui";
import { listHeadlessHostInstances } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { ScrollBase } from "./base/ScrollBase";
import HostLogViewer from "./HostLogViewer";

function InstanceLogDialog({
  hostId,
  instanceId,
}: {
  hostId: string;
  instanceId: number;
}) {
  const [open, setOpen] = useState(false);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button variant="outline" size="sm" onClick={() => setOpen(true)}>
        ログを見る
      </Button>
      <DialogContent className="sm:max-w-[900px]">
        <DialogHeader>
          <DialogTitle>インスタンス #{instanceId} のログ</DialogTitle>
        </DialogHeader>
        {open && (
          <HostLogViewer
            hostId={hostId}
            instanceId={instanceId}
            tailing={false}
            height="60vh"
          />
        )}
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">閉じる</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function PastInstancesDialog({
  hostId,
  open,
  onOpenChange,
}: {
  hostId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { data, isPending } = useQuery(
    listHeadlessHostInstances,
    { hostId },
    { enabled: open },
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>過去のインスタンス一覧</DialogTitle>
        </DialogHeader>
        <ScrollBase height="60vh">
          {isPending ? (
            <div className="py-4 text-center text-muted-foreground">
              読み込み中...
            </div>
          ) : data?.instances.length === 0 ? (
            <div className="py-4 text-center text-muted-foreground">
              インスタンスがありません
            </div>
          ) : (
            <div className="space-y-2">
              {data?.instances.map((inst) => (
                <div
                  key={inst.instanceId}
                  className="flex items-center justify-between p-3 border rounded"
                >
                  <div className="space-y-1">
                    <div className="font-medium">
                      インスタンス #{inst.instanceId}
                      {inst.isCurrent && (
                        <span className="ml-2 text-xs bg-primary text-primary-foreground px-2 py-0.5 rounded">
                          現在
                        </span>
                      )}
                    </div>
                    <div className="text-sm text-muted-foreground">
                      開始:{" "}
                      {inst.firstLogAt
                        ? new Date(
                            Number(inst.firstLogAt.seconds) * 1000,
                          ).toLocaleString("ja-JP")
                        : "-"}
                    </div>
                    <div className="text-sm text-muted-foreground">
                      終了:{" "}
                      {inst.lastLogAt
                        ? new Date(
                            Number(inst.lastLogAt.seconds) * 1000,
                          ).toLocaleString("ja-JP")
                        : "-"}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      ログ: {inst.logCount.toString()}件
                    </div>
                  </div>
                  <InstanceLogDialog
                    hostId={hostId}
                    instanceId={inst.instanceId}
                  />
                </div>
              ))}
            </div>
          )}
        </ScrollBase>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">閉じる</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
