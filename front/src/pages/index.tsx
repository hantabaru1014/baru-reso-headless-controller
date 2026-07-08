import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { Controller, useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { TFunction } from "i18next";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";
import {
  createMessage,
  deleteMessage,
  listMessages,
  updateMessage,
} from "../../pbgen/hdlctrl/v1/message-MessageService_connectquery";
import { Message } from "../../pbgen/hdlctrl/v1/message_pb";
import { listGroups } from "../../pbgen/hdlctrl/v1/permission-GroupService_connectquery";
import { GroupType } from "../../pbgen/hdlctrl/v1/permission_pb";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  Badge,
  Button,
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../components/ui";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "../components/ui/resizable";
import {
  Loading,
  Markdown,
  RefetchButton,
  SelectField,
  TextField,
  TextareaField,
  UserCell,
} from "../components/base";
import { MessageList } from "../components/MessageList";
import { usePermissions } from "../hooks/usePermissions";
import { PERMISSION_KEYS } from "../libs/permissionUtils";
import { formatTimestamp } from "../libs/datetimeUtils";

// 全員向け (group_id 未設定) を表す SelectField 用のセンチネル.
// Radix Select は空文字の value を許容しないため専用の値を使う.
const ALL_USERS_VALUE = "__all__";

const makeMessageFormSchema = (t: TFunction) =>
  z.object({
    title: z
      .string()
      .min(1, t("dashboardPage.titleRequired"))
      .max(200, t("dashboardPage.titleMaxLength")),
    body: z.string().min(1, t("dashboardPage.bodyRequired")),
    groupId: z.string(),
  });
type MessageFormData = z.infer<ReturnType<typeof makeMessageFormSchema>>;

function MessageEditorDialog({
  message,
  open,
  onClose,
  onSaved,
}: {
  /** 指定時は編集, 未指定時は新規作成. */
  message: Message | undefined;
  open: boolean;
  onClose: () => void;
  onSaved: (saved: Message) => void;
}) {
  const { t } = useTranslation();
  const isEdit = !!message;
  const createMut = useMutation(createMessage);
  const updateMut = useMutation(updateMessage);
  const isPending = createMut.isPending || updateMut.isPending;

  const messageFormSchema = useMemo(() => makeMessageFormSchema(t), [t]);

  // 対象グループ候補. システムグループは掲示板の対象にしないため除外する.
  const { data: groupsData } = useQuery(listGroups, {}, { enabled: open });
  const groupOptions = useMemo(
    () => [
      { id: ALL_USERS_VALUE, label: t("dashboardPage.forEveryone") },
      ...(groupsData?.groups ?? [])
        .filter((g) => g.type !== GroupType.SYSTEM)
        .map((g) => ({ id: g.id, label: g.name })),
    ],
    [groupsData?.groups, t],
  );

  const {
    control,
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<MessageFormData>({
    resolver: zodResolver(messageFormSchema),
    defaultValues: { title: "", body: "", groupId: ALL_USERS_VALUE },
  });

  // ダイアログを開くたびに対象メッセージの内容で初期化する.
  useEffect(() => {
    if (open) {
      reset({
        title: message?.title ?? "",
        body: message?.body ?? "",
        groupId: message?.groupId ?? ALL_USERS_VALUE,
      });
    }
  }, [open, message, reset]);

  const onSubmit = async (data: MessageFormData) => {
    const groupId = data.groupId === ALL_USERS_VALUE ? undefined : data.groupId;
    try {
      if (isEdit) {
        const res = await updateMut.mutateAsync({
          messageId: message.id,
          title: data.title,
          body: data.body,
          groupId,
        });
        toast.success(t("dashboardPage.updateSuccess"));
        if (res.message) onSaved(res.message);
      } else {
        const res = await createMut.mutateAsync({
          title: data.title,
          body: data.body,
          groupId,
        });
        toast.success(t("dashboardPage.createSuccess"));
        if (res.message) onSaved(res.message);
      }
      onClose();
    } catch (e) {
      toast.error(
        e instanceof Error
          ? e.message
          : isEdit
            ? t("dashboardPage.updateError")
            : t("dashboardPage.createError"),
      );
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>
            {isEdit
              ? t("dashboardPage.editMessage")
              : t("dashboardPage.createMessage")}
          </DialogTitle>
        </DialogHeader>
        <form
          id="message-form"
          onSubmit={handleSubmit(onSubmit)}
          className="space-y-4"
        >
          <TextField
            label={t("dashboardPage.titleLabel")}
            {...register("title")}
            error={errors.title?.message}
          />
          <TextareaField
            label={t("dashboardPage.bodyLabel")}
            rows={10}
            {...register("body")}
            error={errors.body?.message}
          />
          <Controller
            name="groupId"
            control={control}
            render={({ field }) => (
              <SelectField
                label={t("dashboardPage.targetLabel")}
                helperText={t("dashboardPage.targetHelperText")}
                options={groupOptions}
                selectedId={field.value}
                onChange={(o) => field.onChange(o.id)}
                error={errors.groupId?.message}
              />
            )}
          />
        </form>
        <DialogFooter>
          <Button type="submit" form="message-form" disabled={isPending}>
            {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {isEdit ? t("common.update") : t("common.create")}
          </Button>
          <DialogClose asChild>
            <Button variant="outline" type="button">
              {t("common.cancel")}
            </Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function DeleteMessageDialog({
  message,
  open,
  onClose,
  onDeleted,
}: {
  message: Message | undefined;
  open: boolean;
  onClose: () => void;
  onDeleted: () => void;
}) {
  const { t } = useTranslation();
  const { mutateAsync, isPending } = useMutation(deleteMessage);

  const handleConfirm = async () => {
    if (!message) return;
    try {
      await mutateAsync({ messageId: message.id });
      toast.success(t("dashboardPage.deleteSuccess"));
      onDeleted();
      onClose();
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t("dashboardPage.deleteError"),
      );
    }
  };

  return (
    <AlertDialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("dashboardPage.deleteTitle")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("dashboardPage.deleteConfirm", { title: message?.title })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={isPending}>
            {t("common.cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            disabled={isPending}
            onClick={(e) => {
              e.preventDefault();
              handleConfirm();
            }}
            className="bg-destructive text-white hover:bg-destructive/90"
          >
            {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {t("common.delete")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function MessageDetail({
  message,
  canManage,
  onEdit,
  onDelete,
}: {
  message: Message;
  canManage: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const { t } = useTranslation();
  return (
    <article className="space-y-4">
      <div className="flex items-start justify-between gap-4">
        <h1 className="text-2xl font-bold">{message.title}</h1>
        {canManage && (
          <div className="flex shrink-0 gap-2">
            <Button variant="outline" size="sm" onClick={onEdit}>
              {t("common.edit")}
            </Button>
            <Button variant="ghost" size="sm" onClick={onDelete}>
              {t("common.delete")}
            </Button>
          </div>
        )}
      </div>
      <div className="text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
        <span className="flex items-center gap-1">
          {t("dashboardPage.target")}
          <Badge variant="secondary">
            {message.groupId
              ? (message.groupName ?? message.groupId)
              : t("dashboardPage.everyone")}
          </Badge>
        </span>
        <span>
          {t("dashboardPage.updatedAt")} {formatTimestamp(message.updatedAt)}
        </span>
        <span>
          {t("dashboardPage.createdAt")} {formatTimestamp(message.createdAt)}
        </span>
        {message.lastUpdatedBy && (
          <span className="flex items-center gap-1">
            {t("dashboardPage.lastUpdatedBy")}
            <UserCell userId={message.lastUpdatedBy} />
          </span>
        )}
      </div>
      <div className="border-t pt-4">
        <Markdown>{message.body}</Markdown>
      </div>
    </article>
  );
}

export default function HomePage() {
  const { t } = useTranslation();
  const { hasSystemPermission, isPending: isPermPending } = usePermissions();
  const canManage = hasSystemPermission(PERMISSION_KEYS.SYSTEM_MESSAGE_MANAGE);

  const { data, isPending, refetch } = useQuery(listMessages, {});
  const messages = useMemo(() => data?.messages ?? [], [data?.messages]);

  const [selectedId, setSelectedId] = useState<string | undefined>(undefined);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Message | undefined>(undefined);
  const [deleteTarget, setDeleteTarget] = useState<Message | undefined>(
    undefined,
  );

  const selectedMessage = messages.find((m) => m.id === selectedId);

  // データ読み込み後に有効な選択が無ければ先頭 (最新) を自動選択する.
  useEffect(() => {
    if (messages.length > 0 && !messages.some((m) => m.id === selectedId)) {
      setSelectedId(messages[0].id);
    }
  }, [messages, selectedId]);

  const openCreate = () => {
    setEditTarget(undefined);
    setEditorOpen(true);
  };

  const openEdit = (message: Message) => {
    setEditTarget(message);
    setEditorOpen(true);
  };

  return (
    <div className="flex h-[calc(100dvh-9rem)] min-h-[24rem] flex-col gap-4">
      <div className="flex items-center justify-between gap-2">
        <p className="text-muted-foreground text-sm">
          {t("dashboardPage.description")}
        </p>
        <div className="flex gap-2">
          <RefetchButton refetch={refetch} />
          {canManage && (
            <Button onClick={openCreate}>{t("dashboardPage.createNew")}</Button>
          )}
        </div>
      </div>

      {isPending || isPermPending ? (
        <Loading loading>
          <div className="h-64" />
        </Loading>
      ) : (
        <ResizablePanelGroup direction="horizontal" className="min-h-0 flex-1">
          {/* 左パネル: お知らせ一覧 */}
          <ResizablePanel defaultSize={30} minSize={20}>
            <div className="flex h-full flex-col rounded-l-md border">
              <div className="flex-1 overflow-auto">
                <MessageList
                  messages={messages}
                  selectedId={selectedId}
                  onSelect={(m) => setSelectedId(m.id)}
                />
              </div>
            </div>
          </ResizablePanel>

          <ResizableHandle withHandle />

          {/* 右パネル: 内容表示 */}
          <ResizablePanel defaultSize={70} minSize={40}>
            <div className="h-full overflow-auto rounded-r-md border p-4">
              {selectedMessage ? (
                <MessageDetail
                  message={selectedMessage}
                  canManage={canManage}
                  onEdit={() => openEdit(selectedMessage)}
                  onDelete={() => setDeleteTarget(selectedMessage)}
                />
              ) : (
                <div className="text-muted-foreground flex h-full items-center justify-center text-sm">
                  {t("dashboardPage.noMessages")}
                </div>
              )}
            </div>
          </ResizablePanel>
        </ResizablePanelGroup>
      )}

      <MessageEditorDialog
        message={editTarget}
        open={editorOpen}
        onClose={() => setEditorOpen(false)}
        onSaved={async (saved) => {
          // refetch 前に選択を切り替えると auto-select effect が旧一覧の先頭に
          // 戻してしまうため、一覧へ反映されてから選択する.
          await refetch();
          setSelectedId(saved.id);
        }}
      />
      <DeleteMessageDialog
        message={deleteTarget}
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(undefined)}
        onDeleted={() => {
          setSelectedId(undefined);
          refetch();
        }}
      />
    </div>
  );
}
