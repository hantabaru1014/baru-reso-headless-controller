import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  deleteEndedSession,
  getSessionDetails,
  updateSessionExtraSettings,
  updateSessionParameters,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { Button } from "./ui/button";
import { Card, CardContent } from "./ui/card";
import { EditableTextField } from "./base/EditableTextField";
import { EditableSelectField } from "./base/EditableSelectField";
import { AccessLevels } from "../constants";
import SessionControlButtons from "./SessionControlButtons";
import { ImageOff } from "lucide-react";
import { RefetchButton } from "./base/RefetchButton";
import { SessionStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { useNavigate } from "react-router";
import { startupParamsToSearchParams } from "../libs/sessionFormUtils";
import { formatTimestamp } from "../libs/datetimeUtils";
import { toast } from "sonner";
import { EditableTextArea, SplitButton } from "./base";
import { AspectRatio, DropdownMenuItem } from "./ui";
import HostTip from "./HostTip";
import { SessionOpsMenu } from "./SessionOpsMenu";
import { useTranslation } from "react-i18next";

export default function SessionForm({ sessionId }: { sessionId: string }) {
  const { t } = useTranslation();
  const BOOL_SELECT_OPTIONS = [
    { id: "true", label: t("common.yes"), value: true },
    { id: "false", label: t("common.no"), value: false },
  ];
  const { data, refetch, isPending } = useQuery(getSessionDetails, {
    sessionId,
  });
  const { mutateAsync: mutateSave } = useMutation(updateSessionParameters);
  const { mutateAsync: mutateSaveExtra } = useMutation(
    updateSessionExtraSettings,
  );
  const { mutateAsync: mutateDelete, isPending: isPendingDelete } =
    useMutation(deleteEndedSession);
  const navigate = useNavigate();

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
    toast.success(t("sessionForm.sessionUrlCopied"));
  };

  const handleCopyWorldUrl = () => {
    const url = sessionState?.worldUrl;
    if (!url) {
      return;
    }
    navigator.clipboard.writeText(url);
    toast.success(t("sessionForm.worldUrlCopied"));
  };

  const handleOpenWithSameSettings = () => {
    const searchParams = startupParamsToSearchParams(startupParams);
    navigate(`/sessions/new?${searchParams.toString()}`);
  };

  const handleDeleteSession = async () => {
    try {
      await mutateDelete({ sessionId });
      toast.success(t("sessionForm.sessionDeleted"));
      navigate("/sessions");
    } catch (e) {
      toast.error(t("sessionForm.sessionDeleteError", { error: `${e}` }));
    }
  };

  return (
    <>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 col-span-12">
        <EditableTextField
          label={t("sessionForm.sessionName")}
          value={sessionState?.name || startupParams?.name || ""}
          onSave={(v) => handleSave("name", v)}
          readonly={!isRunning}
          isLoading={isPending}
        />
        <div className="flex justify-end space-x-2">
          {isRunning ? (
            <>
              <SessionControlButtons
                sessionId={sessionId}
                canSaveOverride={sessionState?.canSave}
                canSaveAs={sessionState?.canSaveAs}
                leadingButtons={<RefetchButton refetch={refetch} />}
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
                          {t("sessionForm.copyWorldUrl")}
                        </DropdownMenuItem>
                      }
                    >
                      {t("sessionForm.copyUrl")}
                    </SplitButton>
                    {hostId && (
                      <SessionOpsMenu hostId={hostId} sessionId={sessionId} />
                    )}
                  </>
                }
              />
            </>
          ) : (
            <div className="flex space-x-2">
              {startupParams?.loadWorld?.case === "loadWorldUrl" &&
                startupParams.loadWorld.value && (
                  <Button
                    variant="outline"
                    onClick={() => {
                      if (
                        startupParams?.loadWorld?.case === "loadWorldUrl" &&
                        startupParams.loadWorld.value
                      ) {
                        navigator.clipboard.writeText(
                          startupParams.loadWorld.value,
                        );
                        toast.success(t("sessionForm.worldUrlCopied"));
                      }
                    }}
                  >
                    {t("sessionForm.copyWorldUrl")}
                  </Button>
                )}
              <Button onClick={handleOpenWithSameSettings}>
                {t("sessionForm.startWithSameSettings")}
              </Button>
              <Button
                variant="destructive"
                onClick={handleDeleteSession}
                disabled={isPendingDelete}
              >
                {t("common.delete")}
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
                  alt={t("sessionForm.thumbnailAlt")}
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
            {t("sessionForm.hostLabel")}:{" "}
            <HostTip hostId={data?.session?.hostId} />
          </span>
          <span>
            {t("sessionForm.startedAt")}:{" "}
            {formatTimestamp(data?.session?.startedAt)}
          </span>
          {data?.session?.groupId && (
            <span>
              {t("sessionForm.groupLabel")}: {data?.session?.groupId}
            </span>
          )}
          {data?.session?.createdBy && (
            <span>
              {t("sessionForm.createdBy")}: {data?.session?.createdBy}
            </span>
          )}
          {data?.session?.endedAt && (
            <span>
              {t("sessionForm.endedAt")}:{" "}
              {formatTimestamp(data?.session?.endedAt)}
            </span>
          )}
          {sessionState?.lastSavedAt && sessionState.canSave && (
            <span>
              {t("sessionForm.lastSavedAt")}:{" "}
              {formatTimestamp(sessionState.lastSavedAt)}
            </span>
          )}
          {isRunning && (
            <span>
              {t("sessionForm.resoniteLinkConnected")}:{" "}
              {sessionState?.resoniteLinkClientsCount ?? 0}
            </span>
          )}
          <EditableTextArea
            label={t("sessionForm.adminMemo")}
            value={data?.session?.memo || ""}
            onSave={(v) => handleSaveExtra("memo", v)}
            isLoading={isPending}
          />
          <EditableTextArea
            label={t("common.description")}
            value={
              sessionState?.description || startupParams?.description || ""
            }
            onSave={(v) => handleSave("description", v)}
            readonly={!isRunning}
            isLoading={isPending}
          />
        </div>
        <EditableTextField
          label={t("sessionForm.tags")}
          value={
            sessionState?.tags?.join(", ") ||
            startupParams?.tags?.join(", ") ||
            ""
          }
          onSave={handleSaveTags}
          readonly={!isRunning}
          isLoading={isPending}
          helperText={t("sessionForm.commaSeparated")}
        />
        <EditableTextField
          label={t("sessionForm.maxUsers")}
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
          label={t("sessionForm.accessLevel")}
          options={AccessLevels.map((l) => ({
            id: l.id,
            value: l.value,
            label: t(l.labelKey),
          }))}
          selectedId={
            `${sessionState?.accessLevel || startupParams?.accessLevel}` || "1"
          }
          onSave={(v) => handleSave("accessLevel", v)}
          readonly={!isRunning}
          isLoading={isPending}
        />
        <EditableTextField
          label={t("sessionForm.awayKickMinutes")}
          type="number"
          value={
            sessionState?.awayKickMinutes ||
            startupParams?.awayKickMinutes ||
            -1
          }
          onSave={(v) => handleSave("awayKickMinutes", parseFloat(v))}
          helperText={t("sessionForm.disableWithMinusOne")}
          readonly={!isRunning}
          isLoading={isPending}
        />
        <EditableSelectField
          label={t("sessionForm.hideFromPublicListing")}
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
          label={t("sessionForm.saveOnExit")}
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
          label={t("sessionForm.autoSaveIntervalSeconds")}
          type="number"
          value={
            sessionState?.autoSaveIntervalSeconds ||
            startupParams?.autoSaveIntervalSeconds ||
            -1
          }
          onSave={(v) => handleSave("autoSaveIntervalSeconds", parseInt(v))}
          helperText={t("sessionForm.disableWithMinusOne")}
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
          label={t("sessionForm.idleRestartIntervalSeconds")}
          type="number"
          value={
            sessionState?.idleRestartIntervalSeconds ||
            startupParams?.idleRestartIntervalSeconds ||
            -1
          }
          onSave={(v) => handleSave("idleRestartIntervalSeconds", parseInt(v))}
          helperText={t("sessionForm.disableWithMinusOne")}
          readonly={!isRunning}
          isLoading={isPending}
        />
      </div>
    </>
  );
}
