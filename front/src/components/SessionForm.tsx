import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  deleteEndedSession,
  getSessionDetails,
  listHeadlessHost,
  startWorld,
  updateSessionExtraSettings,
  updateSessionParameters,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { Button } from "./ui/button";
import { Card, CardContent } from "./ui/card";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "./ui/dialog";
import { EditableTextField } from "./base/EditableTextField";
import { EditableSelectField } from "./base/EditableSelectField";
import { AccessLevels } from "../constants";
import SessionControlButtons from "./SessionControlButtons";
import { ImageOff } from "lucide-react";
import { RefetchButton } from "./base/RefetchButton";
import {
  HeadlessHostStatus,
  SessionStatus,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import { useState } from "react";
import { SelectField } from "./base/SelectField";
import { useNavigate } from "react-router";
import { formatTimestamp } from "../libs/datetimeUtils";
import { toast } from "sonner";
import { EditableTextArea, SplitButton } from "./base";
import { AspectRatio, DropdownMenuItem } from "./ui";
import HostTip from "./HostTip";

const BOOL_SELECT_OPTIONS = [
  { id: "true", label: "はい", value: true },
  { id: "false", label: "いいえ", value: false },
];

function SelectHostDialog({
  isOpen,
  onClose,
  title,
  primaryButtonLabel,
}: {
  isOpen: boolean;
  onClose: (hostId: string | null) => void;
  title: string;
  primaryButtonLabel: string;
}) {
  const { data: hostList } = useQuery(listHeadlessHost);
  const [selectedHostId, setSelectedHostId] = useState("");

  return (
    <Dialog open={isOpen} onOpenChange={() => onClose(null)}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <SelectField
          label="Host"
          options={
            hostList?.hosts
              .filter((host) => host.status === HeadlessHostStatus.RUNNING)
              .map((host) => ({
                id: host.id,
                label: `${host.name} (${host.id.slice(0, 6)})`,
                value: host,
              })) ?? []
          }
          selectedId={selectedHostId || ""}
          onChange={(option) => setSelectedHostId(option.value?.id ?? "")}
          minWidth="7rem"
        />
        <DialogFooter>
          <Button onClick={() => onClose(selectedHostId)}>
            {primaryButtonLabel}
          </Button>
          <Button variant="outline" onClick={() => onClose(null)}>
            キャンセル
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default function SessionForm({ sessionId }: { sessionId: string }) {
  const { data, refetch, isPending } = useQuery(getSessionDetails, {
    sessionId,
  });
  const { mutateAsync: mutateSave } = useMutation(updateSessionParameters);
  const { mutateAsync: mutateSaveExtra } = useMutation(
    updateSessionExtraSettings,
  );
  const { mutateAsync: mutateStartWorld, isPending: isPendingStartWorld } =
    useMutation(startWorld);
  const { mutateAsync: mutateDelete } = useMutation(deleteEndedSession);
  const navigate = useNavigate();
  const [isOpenSelectHostDialog, setIsOpenSelectHostDialog] = useState(false);

  const hostId = data?.session?.hostId;
  const isRunning = data?.session?.status === SessionStatus.RUNNING;
  const sessionState = data?.session?.currentState;
  const startupParams = data?.session?.startupParameters;

  const handleSave = async <V,>(fieldName: string, value: V) => {
    try {
      await mutateSave({
        hostId,
        parameters: {
          sessionId,
          [fieldName]: value,
        },
      });
      // すぐには反映されない項目もあるので、ちょっと待ってから再取得する
      setTimeout(() => refetch(), 500);
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  const handleSaveTags = async (tags: string) => {
    const tagList = tags
      .split(",")
      .map((t) => t.trim())
      .filter((t) => t);
    try {
      await mutateSave({
        hostId,
        parameters: {
          sessionId,
          updateTags: true,
          tags: tagList,
        },
      });
      refetch();
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  const handleSaveExtra = async <V,>(fieldName: string, value: V) => {
    try {
      await mutateSaveExtra({
        sessionId,
        [fieldName]: value,
      });
      refetch();
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  const handleCopyUrl = () => {
    const url = sessionState?.sessionUrl;
    if (!url) {
      return;
    }
    navigator.clipboard.writeText(url);
    toast.success("セッションURLをコピーしました");
  };

  const handleCopyWorldUrl = () => {
    const url = sessionState?.worldUrl;
    if (!url) {
      return;
    }
    navigator.clipboard.writeText(url);
    toast.success("ワールドURLをコピーしました");
  };

  const handleRestartSession = async (selectedHostId: string) => {
    if (!selectedHostId) {
      return;
    }
    try {
      const result = await mutateStartWorld({
        hostId: selectedHostId,
        parameters: startupParams,
        memo: data?.session?.memo,
      });
      if (result.openedSession) {
        toast.success("セッションを開始しました");
        navigate(`/sessions/${result.openedSession.id}`);
      }
    } catch (e) {
      toast.error(`セッションの開始に失敗しました: ${e}`);
    }
  };

  const handleDeleteSession = async () => {
    try {
      await mutateDelete({ sessionId });
      toast.success("セッションを削除しました");
      navigate("/sessions");
    } catch (e) {
      toast.error(`セッションの削除に失敗しました: ${e}`);
    }
  };

  return (
    <>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 col-span-12">
        <EditableTextField
          label="セッション名"
          value={sessionState?.name || startupParams?.name || ""}
          onSave={(v) => handleSave("name", v)}
          readonly={!isRunning}
          isLoading={isPending}
        />
        <div className="flex justify-end space-x-2">
          {isRunning ? (
            <SessionControlButtons
              sessionId={sessionId}
              canSaveOverride={sessionState?.canSave}
              canSaveAs={sessionState?.canSaveAs}
              additionalButtons={
                <>
                  <SplitButton
                    variant="outline"
                    onClick={handleCopyUrl}
                    dropdownContent={
                      <DropdownMenuItem
                        onClick={handleCopyWorldUrl}
                        disabled={!sessionState?.worldUrl}
                      >
                        ワールドURLをコピー
                      </DropdownMenuItem>
                    }
                  >
                    URLをコピー
                  </SplitButton>
                  <RefetchButton refetch={refetch} />
                </>
              }
            />
          ) : (
            <div className="flex space-x-2">
              <Button
                disabled={isPendingStartWorld}
                onClick={() => setIsOpenSelectHostDialog(true)}
              >
                同設定で開始
              </Button>
              <Button variant="destructive" onClick={handleDeleteSession}>
                削除
              </Button>
            </div>
          )}
        </div>
        <Card className="h-full">
          <CardContent className="h-full">
            <AspectRatio ratio={2 / 1}>
              {sessionState?.thumbnailUrl ? (
                <img
                  src={sessionState?.thumbnailUrl}
                  alt="セッションサムネイル"
                  className="w-full h-auto"
                />
              ) : (
                <div className="h-full flex items-center justify-center">
                  <ImageOff className="w-8 h-8 text-gray-400" />
                </div>
              )}
            </AspectRatio>
          </CardContent>
        </Card>
        <div className="flex flex-col space-y-1">
          <span>
            ホスト: <HostTip hostId={data?.session?.hostId} />
          </span>
          <span>開始: {formatTimestamp(data?.session?.startedAt)}</span>
          {data?.session?.ownerId && (
            <span>オーナー: {data?.session?.ownerId}</span>
          )}
          {data?.session?.endedAt && (
            <span>終了: {formatTimestamp(data?.session?.endedAt)}</span>
          )}
          {sessionState?.lastSavedAt && sessionState.canSave && (
            <span>最終保存: {formatTimestamp(sessionState.lastSavedAt)}</span>
          )}
          <EditableTextArea
            label="管理者メモ"
            value={data?.session?.memo || ""}
            onSave={(v) => handleSaveExtra("memo", v)}
            isLoading={isPending}
          />
          <EditableTextArea
            label="説明"
            value={
              sessionState?.description || startupParams?.description || ""
            }
            onSave={(v) => handleSave("description", v)}
            readonly={!isRunning}
            isLoading={isPending}
          />
        </div>
        <EditableTextField
          label="タグ"
          value={
            sessionState?.tags?.join(", ") ||
            startupParams?.tags?.join(", ") ||
            ""
          }
          onSave={handleSaveTags}
          readonly={!isRunning}
          isLoading={isPending}
          helperText="カンマ区切りで入力してください"
        />
        <EditableTextField
          label="最大ユーザー数"
          type="number"
          value={
            sessionState?.maxUsers?.toString() ||
            startupParams?.maxUsers?.toString() ||
            "0"
          }
          onSave={(v) => handleSave("maxUsers", parseInt(v))}
          readonly={!isRunning}
          isLoading={isPending}
        />
        <EditableSelectField
          label="アクセスレベル"
          options={AccessLevels.map((l) => l)}
          selectedId={
            `${sessionState?.accessLevel || startupParams?.accessLevel}` || "1"
          }
          onSave={(v) => handleSave("accessLevel", v)}
          readonly={!isRunning}
          isLoading={isPending}
        />
        {/* <EditableCheckBox
          label="自動アップデート"
          checked={data?.session?.autoUpgrade || false}
          onSave={(v) => handleSaveExtra("autoUpgrade", v)}
          isLoading={isPending}
          helperText="新しいバージョンが出た場合にユーザがいなければ自動で新しいバージョンのホストに移行します"
        /> */}
        <EditableTextField
          label="AFKキック時間(分)"
          type="number"
          value={
            sessionState?.awayKickMinutes ||
            startupParams?.awayKickMinutes ||
            -1
          }
          onSave={(v) => handleSave("awayKickMinutes", parseFloat(v))}
          helperText="-1で無効"
          readonly={!isRunning}
          isLoading={isPending}
        />
        <EditableSelectField
          label="セッションリストから隠す"
          options={BOOL_SELECT_OPTIONS}
          selectedId={
            `${sessionState?.hideFromPublicListing}` ||
            `${startupParams?.hideFromPublicListing}` ||
            "false"
          }
          onSave={(v) => handleSave("hideFromPublicListing", v)}
          readonly={!isRunning}
          isLoading={isPending}
        />
        <EditableSelectField
          label="セッション終了時に保存"
          options={BOOL_SELECT_OPTIONS}
          selectedId={
            `${sessionState?.saveOnExit}` ||
            `${startupParams?.saveOnExit}` ||
            "false"
          }
          onSave={(v) => handleSave("saveOnExit", v)}
          readonly={!isRunning}
          isLoading={isPending}
        />
        <EditableTextField
          label="自動保存間隔(秒)"
          type="number"
          value={
            sessionState?.autoSaveIntervalSeconds ||
            startupParams?.autoSaveIntervalSeconds ||
            -1
          }
          onSave={(v) => handleSave("autoSaveIntervalSeconds", parseInt(v))}
          helperText="-1で無効"
          readonly={!isRunning}
          isLoading={isPending}
        />
        {/* FIXME: 反応しないのでヘッドレス側を修正するまで一旦コメントアウト */}
        {/* <EditableCheckBox
          label="オートスリープ"
          checked={sessionState?.autoSleep || startupParams?.autoSleep || false}
          onSave={(v) => handleSave("autoSleep", v)}
          readonly={!isRunning}
          isLoading={isPending}
        /> */}
        <EditableTextField
          label="アイドル時の自動再起動間隔(秒)"
          type="number"
          value={
            sessionState?.idleRestartIntervalSeconds ||
            startupParams?.idleRestartIntervalSeconds ||
            -1
          }
          onSave={(v) => handleSave("idleRestartIntervalSeconds", parseInt(v))}
          helperText="-1で無効"
          readonly={!isRunning}
          isLoading={isPending}
        />
      </div>
      <SelectHostDialog
        isOpen={isOpenSelectHostDialog}
        title="セッションを開始するホストを選択"
        primaryButtonLabel="開始"
        onClose={(hostId) => {
          setIsOpenSelectHostDialog(false);
          if (hostId) {
            handleRestartSession(hostId);
          }
        }}
      />
    </>
  );
}
