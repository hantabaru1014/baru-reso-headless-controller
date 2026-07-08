import { useMutation } from "@connectrpc/connect-query";
import { MoreHorizontalIcon } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import {
  sendDynamicImpulse,
  spawnItem,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { TextField } from "./base/TextField";
import { ResoniteLinkConnectionDialog } from "./ResoniteLinkConnectionDialog";
import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "./ui";

function SpawnItemDialog({
  hostId,
  sessionId,
  open,
  onClose,
}: {
  hostId: string;
  sessionId: string;
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [url, setUrl] = useState("");
  const [posX, setPosX] = useState("");
  const [posY, setPosY] = useState("");
  const [posZ, setPosZ] = useState("");
  const { mutateAsync: mutateSpawn, isPending } = useMutation(spawnItem);

  const anyPosSet =
    posX.trim() !== "" || posY.trim() !== "" || posZ.trim() !== "";
  const positionOrUndefined = anyPosSet
    ? {
        x: Number(posX) || 0,
        y: Number(posY) || 0,
        z: Number(posZ) || 0,
      }
    : undefined;

  const handleSpawn = async () => {
    try {
      await mutateSpawn({
        hostId,
        parameters: {
          sessionId,
          url: url.trim(),
          position: positionOrUndefined,
        },
      });
      toast.success(t("sessionOpsMenu.spawnSuccess"));
      setUrl("");
      setPosX("");
      setPosY("");
      setPosZ("");
      onClose();
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t("sessionOpsMenu.spawnFailed"),
      );
    }
  };

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("sessionOpsMenu.spawnItemTitle")}</DialogTitle>
        </DialogHeader>
        <div className="grid gap-3 py-2">
          <TextField
            label={t("sessionOpsMenu.recordUrlLabel")}
            placeholder="resrec:///U-Resonite/R-Public-Cube"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
          />
          <div>
            <p className="text-sm font-medium mb-1">
              {t("sessionOpsMenu.spawnPositionHint")}
            </p>
            <div className="grid grid-cols-3 gap-2">
              <TextField
                label="X"
                type="number"
                value={posX}
                onChange={(e) => setPosX(e.target.value)}
              />
              <TextField
                label="Y"
                type="number"
                value={posY}
                onChange={(e) => setPosY(e.target.value)}
              />
              <TextField
                label="Z"
                type="number"
                value={posZ}
                onChange={(e) => setPosZ(e.target.value)}
              />
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onClose()}>
            {t("sessionOpsMenu.cancel")}
          </Button>
          <Button onClick={handleSpawn} disabled={isPending || !url.trim()}>
            {isPending
              ? t("sessionOpsMenu.spawning")
              : t("sessionOpsMenu.spawn")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

type ImpulseValueType = "none" | "string" | "int" | "float";

function SendDynamicImpulseDialog({
  hostId,
  sessionId,
  open,
  onClose,
}: {
  hostId: string;
  sessionId: string;
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [tag, setTag] = useState("");
  const [valueType, setValueType] = useState<ImpulseValueType>("none");
  const [valueStr, setValueStr] = useState("");
  const { mutateAsync: mutateSend, isPending } =
    useMutation(sendDynamicImpulse);

  const handleSend = async () => {
    let value:
      | { case: "stringValue"; value: string }
      | { case: "intValue"; value: number }
      | { case: "floatValue"; value: number }
      | undefined;
    switch (valueType) {
      case "string":
        value = { case: "stringValue", value: valueStr };
        break;
      case "int":
        value = { case: "intValue", value: parseInt(valueStr, 10) || 0 };
        break;
      case "float":
        value = { case: "floatValue", value: parseFloat(valueStr) || 0 };
        break;
      case "none":
        value = undefined;
        break;
    }
    try {
      const res = await mutateSend({
        hostId,
        parameters: {
          sessionId,
          tag: tag.trim(),
          value,
        },
      });
      toast.success(
        t("sessionOpsMenu.impulseSuccess", {
          n: res.triggeredReceivers,
        }),
      );
      setTag("");
      setValueStr("");
      onClose();
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t("sessionOpsMenu.impulseFailed"),
      );
    }
  };

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("sessionOpsMenu.impulseTitle")}</DialogTitle>
        </DialogHeader>
        <div className="grid gap-3 py-2">
          <TextField
            label="Tag"
            placeholder={t("sessionOpsMenu.tagPlaceholder")}
            value={tag}
            onChange={(e) => setTag(e.target.value)}
          />
          <div>
            <p className="text-sm font-medium mb-1">
              {t("sessionOpsMenu.valueTypeLabel")}
            </p>
            <div className="flex gap-2 flex-wrap">
              {(["none", "string", "int", "float"] as const).map((t) => (
                <Button
                  key={t}
                  size="sm"
                  variant={valueType === t ? "default" : "outline"}
                  onClick={() => setValueType(t)}
                >
                  {t}
                </Button>
              ))}
            </div>
          </div>
          {valueType !== "none" && (
            <TextField
              label={t("sessionOpsMenu.valueLabel", { type: valueType })}
              type={
                valueType === "int" || valueType === "float" ? "number" : "text"
              }
              value={valueStr}
              onChange={(e) => setValueStr(e.target.value)}
            />
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onClose()}>
            {t("sessionOpsMenu.cancel")}
          </Button>
          <Button onClick={handleSend} disabled={isPending || !tag.trim()}>
            {isPending ? t("sessionOpsMenu.sending") : t("sessionOpsMenu.send")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function SessionOpsMenu({
  hostId,
  sessionId,
}: {
  hostId: string;
  sessionId: string;
}) {
  const { t } = useTranslation();
  const [openSpawn, setOpenSpawn] = useState(false);
  const [openImpulse, setOpenImpulse] = useState(false);
  const [openResoLink, setOpenResoLink] = useState(false);

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="outline"
            size="icon"
            title={t("sessionOpsMenu.menuTitle")}
          >
            <MoreHorizontalIcon className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={() => setOpenResoLink(true)}>
            {t("sessionOpsMenu.resoniteLinkUrl")}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setOpenSpawn(true)}>
            {t("sessionOpsMenu.spawnItem")}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setOpenImpulse(true)}>
            {t("sessionOpsMenu.sendImpulse")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <SpawnItemDialog
        hostId={hostId}
        sessionId={sessionId}
        open={openSpawn}
        onClose={() => setOpenSpawn(false)}
      />
      <SendDynamicImpulseDialog
        hostId={hostId}
        sessionId={sessionId}
        open={openImpulse}
        onClose={() => setOpenImpulse(false)}
      />
      <ResoniteLinkConnectionDialog
        sessionId={sessionId}
        open={openResoLink}
        onOpenChange={setOpenResoLink}
      />
    </>
  );
}
