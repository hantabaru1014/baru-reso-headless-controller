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
import { useTranslation } from "react-i18next";

function InstanceLogDialog({
  hostId,
  instanceId,
}: {
  hostId: string;
  instanceId: number;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button variant="outline" size="sm" onClick={() => setOpen(true)}>
        {t("pastInstancesDialog.viewLog")}
      </Button>
      <DialogContent className="sm:max-w-[900px]">
        <DialogHeader>
          <DialogTitle>
            {t("pastInstancesDialog.instanceLogTitle", { id: instanceId })}
          </DialogTitle>
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
            <Button variant="outline">{t("pastInstancesDialog.close")}</Button>
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
  const { t } = useTranslation();
  const { data, isPending } = useQuery(
    listHeadlessHostInstances,
    { hostId },
    { enabled: open },
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>{t("pastInstancesDialog.title")}</DialogTitle>
        </DialogHeader>
        <ScrollBase height="60vh">
          {isPending ? (
            <div className="py-4 text-center text-muted-foreground">
              {t("pastInstancesDialog.loading")}
            </div>
          ) : data?.instances.length === 0 ? (
            <div className="py-4 text-center text-muted-foreground">
              {t("pastInstancesDialog.noInstances")}
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
                      {t("pastInstancesDialog.instanceLabel", {
                        id: inst.instanceId,
                      })}
                      {inst.isCurrent && (
                        <span className="ml-2 text-xs bg-primary text-primary-foreground px-2 py-0.5 rounded">
                          {t("pastInstancesDialog.current")}
                        </span>
                      )}
                    </div>
                    <div className="text-sm text-muted-foreground">
                      {t("pastInstancesDialog.startedAt")}{" "}
                      {inst.firstLogAt
                        ? new Date(
                            Number(inst.firstLogAt.seconds) * 1000,
                          ).toLocaleString("ja-JP")
                        : "-"}
                    </div>
                    <div className="text-sm text-muted-foreground">
                      {t("pastInstancesDialog.endedAt")}{" "}
                      {inst.lastLogAt
                        ? new Date(
                            Number(inst.lastLogAt.seconds) * 1000,
                          ).toLocaleString("ja-JP")
                        : "-"}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {t("pastInstancesDialog.logCount", {
                        n: inst.logCount.toString(),
                      })}
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
            <Button variant="outline">{t("pastInstancesDialog.close")}</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
