import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  acceptFriendRequests,
  allowHostAccess,
  denyHostAccess,
  getFriendRequests,
  getHeadlessHost,
  restartHeadlessHost,
  shutdownHeadlessHost,
  updateHeadlessHostSettings,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  Button,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogTrigger,
  DialogClose,
  Badge,
  Avatar,
  AvatarImage,
  AvatarFallback,
} from "./ui";
import { EditableTextField } from "./base/EditableTextField";
import { ReadOnlyField } from "./base/ReadOnlyField";
import { HeadlessHostStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { hostStatusToLabel } from "../libs/hostUtils";
import { useNavigate } from "react-router";
import {
  AllowedAccessEntry_AccessType,
  UserInfo,
} from "../../pbgen/headless/v1/headless_pb";
import { useState } from "react";
import { ScrollBase } from "./base/ScrollBase";
import { SelectField } from "./base/SelectField";
import { toast } from "sonner";
import { DataTable, RefetchButton, TextField } from "./base";
import { ColumnDef } from "@tanstack/react-table";
import { resolveUrl } from "@/libs/skyfrostUtils";

type AllowedAccessEntryType = {
  host: string;
  port: number;
  accessType: AllowedAccessEntry_AccessType;
};

function AllowedUrlHostsDialog({
  hostId,
  hosts: initHosts,
  onClose,
}: {
  hostId: string;
  hosts: AllowedAccessEntryType[];
  onClose?: () => void;
}) {
  const { mutateAsync: allow } = useMutation(allowHostAccess);
  const { mutateAsync: deny } = useMutation(denyHostAccess);

  const [hosts, setHosts] = useState<AllowedAccessEntryType[]>(initHosts);
  const [newUrl, setNewUrl] = useState("");
  const [newPort, setNewPort] = useState("80");
  const [newAccessType, setNewAccessType] = useState(
    AllowedAccessEntry_AccessType.HTTP,
  );

  return (
    <Dialog onOpenChange={(open) => !open && onClose?.()}>
      <DialogTrigger asChild>
        <Button variant="outline">Allowed Url Hosts</Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>Allowed Url Hosts</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="flex gap-2">
            <TextField
              label="Host"
              value={newUrl}
              onChange={(e) => setNewUrl(e.target.value)}
            />
            <TextField
              label="Port"
              type="number"
              value={newPort}
              onChange={(e) => setNewPort(e.target.value)}
            />
            <div className="w-40">
              <SelectField
                label="Access Type"
                options={[
                  {
                    id: AllowedAccessEntry_AccessType[
                      AllowedAccessEntry_AccessType.HTTP
                    ],
                    label: "HTTP",
                    value: AllowedAccessEntry_AccessType.HTTP,
                  },
                  {
                    id: AllowedAccessEntry_AccessType[
                      AllowedAccessEntry_AccessType.WEBSOCKET
                    ],
                    label: "WEBSOCKET",
                    value: AllowedAccessEntry_AccessType.WEBSOCKET,
                  },
                  {
                    id: AllowedAccessEntry_AccessType[
                      AllowedAccessEntry_AccessType.OSC_RECEIVING
                    ],
                    label: "OSC_RECEIVING",
                    value: AllowedAccessEntry_AccessType.OSC_RECEIVING,
                  },
                  {
                    id: AllowedAccessEntry_AccessType[
                      AllowedAccessEntry_AccessType.OSC_SENDING
                    ],
                    label: "OSC_SENDING",
                    value: AllowedAccessEntry_AccessType.OSC_SENDING,
                  },
                ]}
                selectedId={AllowedAccessEntry_AccessType[newAccessType]}
                onChange={(v) =>
                  setNewAccessType(
                    v.value ?? AllowedAccessEntry_AccessType.HTTP,
                  )
                }
              />
            </div>
            <div className="self-end">
              <Button
                onClick={async () => {
                  try {
                    await allow({
                      hostId,
                      request: {
                        host: newUrl,
                        port: parseInt(newPort),
                        accessType: newAccessType,
                      },
                    });
                    setNewUrl("");
                    setNewPort("80");
                    setHosts((prev) => [
                      ...prev,
                      {
                        host: newUrl,
                        port: parseInt(newPort),
                        accessType: newAccessType,
                      },
                    ]);
                  } catch (e) {
                    toast.error(
                      e instanceof Error
                        ? e.message
                        : "ホストの追加に失敗しました",
                    );
                  }
                }}
                disabled={!newUrl}
              >
                Add
              </Button>
            </div>
          </div>
          <ScrollBase height="60vh">
            <div className="space-y-2">
              {hosts.map((host) => (
                <div
                  key={host.host}
                  className="flex items-center justify-between p-2 border rounded"
                >
                  <span>
                    {`${host.host}:${host.port} (${AllowedAccessEntry_AccessType[host.accessType]})`}
                  </span>
                  <Button
                    variant="destructive"
                    onClick={async () => {
                      try {
                        await deny({ hostId, request: host });
                        setHosts((prev) =>
                          prev.filter(
                            (h) =>
                              h.host !== host.host ||
                              h.port !== host.port ||
                              h.accessType !== host.accessType,
                          ),
                        );
                      } catch (e) {
                        toast.error(
                          e instanceof Error
                            ? e.message
                            : "ホストの削除に失敗しました",
                        );
                      }
                    }}
                  >
                    Remove
                  </Button>
                </div>
              ))}
            </div>
          </ScrollBase>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" onClick={() => onClose?.()}>
              閉じる
            </Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function AutoSpawnItemsDialog({
  hostId,
  items: initItems,
  onClose,
}: {
  hostId: string;
  items: string[];
  onClose?: () => void;
}) {
  const { mutateAsync: updateHost } = useMutation(updateHeadlessHostSettings);

  const [items, setItems] = useState<string[]>(initItems);
  const [newItemUri, setNewItemUri] = useState("");

  return (
    <Dialog onOpenChange={(open) => !open && onClose?.()}>
      <DialogTrigger asChild>
        <Button variant="outline">Auto Spawn Items</Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>Auto Spawn Items</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="flex gap-2 items-end">
            <TextField
              label="Item URI"
              value={newItemUri}
              onChange={(e) => setNewItemUri(e.target.value)}
              className="flex-1"
            />
            <div className="self-end">
              <Button
                onClick={async () => {
                  try {
                    await updateHost({
                      hostId,
                      updateAutoSpawnItems: true,
                      autoSpawnItems: [...items, newItemUri],
                    });
                    setNewItemUri("");
                    setItems((prev) => [...prev, newItemUri]);
                  } catch (e) {
                    toast.error(
                      e instanceof Error
                        ? e.message
                        : "アイテムの追加に失敗しました",
                    );
                  }
                }}
                disabled={!newItemUri}
              >
                Add
              </Button>
            </div>
          </div>
          <ScrollBase height="60vh">
            <div className="space-y-2">
              {items.map((item) => (
                <div
                  key={item}
                  className="flex items-center justify-between p-2 border rounded"
                >
                  <span className="text-sm">{item}</span>
                  <Button
                    variant="destructive"
                    onClick={async () => {
                      try {
                        await updateHost({
                          hostId,
                          updateAutoSpawnItems: true,
                          autoSpawnItems: items.filter((i) => i !== item),
                        });
                        setItems((prev) => prev.filter((i) => i !== item));
                      } catch (e) {
                        toast.error(
                          e instanceof Error
                            ? e.message
                            : "アイテムの削除に失敗しました",
                        );
                      }
                    }}
                  >
                    Remove
                  </Button>
                </div>
              ))}
            </div>
          </ScrollBase>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">閉じる</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function FriendRequestsDialog({
  onClose,
  hostId,
}: {
  onClose?: () => void;
  hostId: string;
}) {
  const { data, isPending, refetch } = useQuery(getFriendRequests, { hostId });
  const { mutateAsync: mutateAcceptFriendRequest } =
    useMutation(acceptFriendRequests);

  const columns: ColumnDef<UserInfo>[] = [
    {
      accessorKey: "iconUrl",
      header: "アイコン",
      cell: ({ row }) => (
        <Avatar>
          <AvatarImage
            src={resolveUrl(row.original.iconUrl)}
            alt={`${row.original.name}のアイコン`}
          />
          <AvatarFallback>{row.original.name.charAt(0)}</AvatarFallback>
        </Avatar>
      ),
    },
    {
      accessorKey: "name",
      header: "名前",
    },
    {
      id: "actions",
      header: "アクション",
      cell: ({ row }) => (
        <Button
          onClick={async () => {
            await mutateAcceptFriendRequest({
              hostId,
              userIds: [row.original.id],
            });
            refetch();
          }}
          className="w-full"
        >
          承認
        </Button>
      ),
    },
  ];

  return (
    <Dialog onOpenChange={(open) => !open && onClose?.()}>
      <DialogTrigger asChild>
        <Button variant="outline">
          フレンドリクエスト
          {(data?.users.length ?? 0) > 0 && (
            <Badge variant="default">{data?.users.length ?? 0}</Badge>
          )}
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader className="flex justify-between">
          <DialogTitle>フレンドリクエスト</DialogTitle>
        </DialogHeader>
        <div>
          <div className="flex justify-end mb-2">
            <RefetchButton refetch={refetch} />
          </div>
          <DataTable
            columns={columns}
            data={data?.users ?? []}
            isLoading={isPending}
          />
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">閉じる</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default function HostDetailPanel({ hostId }: { hostId: string }) {
  const navigate = useNavigate();
  const { data, isPending, refetch } = useQuery(getHeadlessHost, { hostId });

  const { mutateAsync: shutdownHost, isPending: isPendingShutdown } =
    useMutation(shutdownHeadlessHost);
  const { mutateAsync: updateHost } = useMutation(updateHeadlessHostSettings);
  const { mutateAsync: restartHost, isPending: isPendingRestart } =
    useMutation(restartHeadlessHost);

  const settings = data?.settings;

  const handleRestart = async () => {
    try {
      const result = await restartHost({
        hostId,
        withUpdate: true,
        withWorldRestart: true,
      });
      setTimeout(() => {
        refetch();
      }, 1000);
      if (result.newHostId) {
        navigate(`/hosts/${result.newHostId}`);
      }
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : "ホストの再起動に失敗しました",
      );
    }
  };

  const handleShutdown = async () => {
    try {
      await shutdownHost({ hostId });
      setTimeout(() => {
        refetch();
      }, 1000);
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : "ホストのシャットダウンに失敗しました",
      );
    }
  };

  const handleSave = async <V,>(fieldName: string, value: V) => {
    try {
      await updateHost({ hostId, [fieldName]: value });
      // すぐには反映されない項目もあるので、ちょっと待ってから再取得する
      setTimeout(() => refetch(), 500);
      return { ok: true };
    } catch (e) {
      return {
        ok: false,
        error: e instanceof Error ? e.message : `${e}`,
      };
    }
  };

  return (
    <div className="space-y-4">
      <div className="col-span-12">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <EditableTextField
            label="Name"
            value={data?.host?.name}
            onSave={(v) => handleSave("name", v)}
            isLoading={isPending}
          />
          <div className="flex items-center gap-2 justify-end">
            <span className="text-sm">
              ステータス:{" "}
              {data?.host?.status
                ? hostStatusToLabel(data?.host?.status)
                : "不明"}
            </span>
            <Button
              variant="outline"
              onClick={handleShutdown}
              disabled={
                isPending ||
                isPendingShutdown ||
                data?.host?.status !== HeadlessHostStatus.RUNNING
              }
            >
              シャットダウン
            </Button>
            <Button
              variant="outline"
              onClick={handleRestart}
              disabled={isPending || isPendingRestart}
            >
              再起動
            </Button>
          </div>
        </div>
      </div>
      {data?.host?.status === HeadlessHostStatus.RUNNING && (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <ReadOnlyField
              label="アカウント"
              value={`${data?.host?.accountName} (${data?.host?.accountId})`}
              isLoading={isPending}
            />
            <ReadOnlyField
              label="Resoniteバージョン"
              value={data?.host?.resoniteVersion}
              isLoading={isPending}
            />
            <ReadOnlyField
              label="FPS (Current)"
              value={data?.host?.fps?.toString()}
              isLoading={isPending}
            />
            <FriendRequestsDialog hostId={hostId} />
          </div>
          {settings && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <ReadOnlyField
                label="Universe ID"
                value={settings.universeId}
                isLoading={isPending}
              />
              <EditableTextField
                label="Username override"
                value={settings.usernameOverride}
                onSave={(v) => handleSave("usernameOverride", v)}
                isLoading={isPending}
              />
              <EditableTextField
                label="Tick Rate (Target FPS)"
                type="number"
                value={settings.tickRate}
                onSave={(v) => handleSave("tickRate", parseFloat(v))}
                isLoading={isPending}
              />
              <EditableTextField
                label="Max concurrent asset transfers"
                type="number"
                value={settings.maxConcurrentAssetTransfers}
                onSave={(v) =>
                  handleSave("maxConcurrentAssetTransfers", parseInt(v))
                }
                isLoading={isPending}
              />
              <AllowedUrlHostsDialog
                onClose={() => refetch()}
                hostId={hostId}
                hosts={settings.allowedUrlHosts
                  .flatMap((e) =>
                    e.ports.map((p) => ({
                      host: e.host,
                      port: p,
                      accessTypes: e.accessTypes,
                    })),
                  )
                  .flatMap((e) =>
                    e.accessTypes
                      .filter(
                        (a) => a !== AllowedAccessEntry_AccessType.UNSPECIFIED, // TODO
                      )
                      .map((a) => ({
                        host: e.host,
                        port: e.port,
                        accessType: a,
                      })),
                  )}
              />
              <AutoSpawnItemsDialog
                hostId={hostId}
                items={settings.autoSpawnItems}
                onClose={() => refetch()}
              />
            </div>
          )}
        </>
      )}
    </div>
  );
}
