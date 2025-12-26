import { Button, Input } from "../ui";
import {
  callUnaryMethod,
  useMutation,
  useTransport,
} from "@connectrpc/connect-query";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import {
  getContactMessages,
  sendContactMessage,
} from "../../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Loader2, Send } from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/libs/cssUtils";
import { resolveUrl } from "@/libs/skyfrostUtils";
import { ContactChatMessageType } from "../../../pbgen/headless/v1/headless_pb";
import type {
  ChatMessagesPanelProps,
  MessagePageParam,
  SessionInviteData,
} from "./types";
import {
  formatMessageTime,
  formatSessionTime,
  parseSessionInvite,
} from "./utils";

export function ChatMessagesPanel({
  accountId,
  contact,
  enabled,
  onNavigateToSession,
  showPlaceholder = true,
  className,
}: ChatMessagesPanelProps) {
  const transport = useTransport();
  const queryClient = useQueryClient();

  const [messageInput, setMessageInput] = useState("");
  const [optimisticMessages, setOptimisticMessages] = useState<
    Array<{
      tempId: string;
      content: string;
      sendTime: { seconds: bigint };
      isOwnMessage: boolean;
    }>
  >([]);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  // Reset state when contact changes or disabled
  useEffect(() => {
    if (!enabled || !contact) {
      setMessageInput("");
      setOptimisticMessages([]);
    }
  }, [enabled, contact?.id]);

  // Reset optimistic messages when contact changes
  useEffect(() => {
    setOptimisticMessages([]);
  }, [contact?.id]);

  // Messages infinite query
  const {
    data: messagesData,
    fetchPreviousPage,
    hasPreviousPage,
    isFetchingPreviousPage,
    isPending: isLoadingMessages,
  } = useInfiniteQuery({
    queryKey: ["contactMessages", accountId, contact?.id],
    queryFn: async ({ pageParam }: { pageParam: MessagePageParam }) => {
      const response = await callUnaryMethod(transport, getContactMessages, {
        headlessAccountId: accountId,
        contactUserId: contact!.id,
        limit: 50,
        beforeId:
          pageParam.direction === "before" ? pageParam.cursorId : undefined,
        afterId:
          pageParam.direction === "after" ? pageParam.cursorId : undefined,
      });
      return {
        messages: response.messages,
        hasMoreBefore: response.hasMoreBefore,
        hasMoreAfter: response.hasMoreAfter,
        direction: pageParam.direction,
      };
    },
    initialPageParam: { direction: "init" } as MessagePageParam,
    getNextPageParam: () => undefined,
    getPreviousPageParam: (firstPage) => {
      if (!firstPage.hasMoreBefore) return undefined;
      const firstMessage = firstPage.messages[0];
      if (!firstMessage) return undefined;
      return {
        direction: "before",
        cursorId: firstMessage.id,
      } as MessagePageParam;
    },
    enabled: enabled && !!contact,
  });

  const messages = useMemo(() => {
    const allMessages =
      messagesData?.pages.flatMap((page) => page.messages) ?? [];
    const uniqueMessages = Array.from(
      new Map(allMessages.map((msg) => [msg.id, msg])).values(),
    );

    const combined = [
      ...uniqueMessages.map((msg) => ({
        id: msg.id,
        type: msg.type,
        content: msg.content,
        sendTime: msg.sendTime,
        isOwnMessage: msg.isOwnMessage,
        isOptimistic: false,
      })),
      ...optimisticMessages.map((msg) => ({
        id: msg.tempId,
        type: ContactChatMessageType.TEXT,
        content: msg.content,
        sendTime: msg.sendTime,
        isOwnMessage: msg.isOwnMessage,
        isOptimistic: true,
      })),
    ];

    return combined.sort((a, b) => {
      const timeA = a.sendTime ? Number(a.sendTime.seconds) : 0;
      const timeB = b.sendTime ? Number(b.sendTime.seconds) : 0;
      return timeA - timeB;
    });
  }, [messagesData?.pages, optimisticMessages]);

  // Virtual scrolling for messages
  const virtualizer = useVirtualizer({
    count: messages.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => 80,
    overscan: 5,
    getItemKey: (index) => messages[index]?.id ?? index,
  });

  const shouldScrollToBottomRef = useRef(true);

  // Handle scroll for loading older messages
  useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container) return;

    const handleScroll = () => {
      if (
        container.scrollTop < 100 &&
        hasPreviousPage &&
        !isFetchingPreviousPage
      ) {
        fetchPreviousPage();
      }
    };

    container.addEventListener("scroll", handleScroll);
    return () => container.removeEventListener("scroll", handleScroll);
  }, [fetchPreviousPage, hasPreviousPage, isFetchingPreviousPage]);

  const isNearBottom = useCallback(() => {
    const container = scrollContainerRef.current;
    if (!container) return false;
    const threshold = 150;
    return (
      container.scrollHeight - container.scrollTop - container.clientHeight <
      threshold
    );
  }, []);

  const scrollToBottom = useCallback(() => {
    const container = scrollContainerRef.current;
    if (!container || messages.length === 0) return;

    requestAnimationFrame(() => {
      setTimeout(() => {
        virtualizer.scrollToIndex(messages.length - 1, { align: "end" });
        requestAnimationFrame(() => {
          container.scrollTop = container.scrollHeight;
        });
      }, 50);
    });
  }, [messages.length, virtualizer]);

  // Polling for new messages
  useEffect(() => {
    if (!enabled || !contact) return;

    const timer = setInterval(async () => {
      const allMessages = messagesData?.pages.flatMap((p) => p.messages) ?? [];
      if (allMessages.length === 0) return;

      const newestMessage = allMessages.reduce((newest, msg) => {
        const newestTime = newest.sendTime
          ? Number(newest.sendTime.seconds)
          : 0;
        const msgTime = msg.sendTime ? Number(msg.sendTime.seconds) : 0;
        return msgTime > newestTime ? msg : newest;
      });

      try {
        const response = await callUnaryMethod(transport, getContactMessages, {
          headlessAccountId: accountId,
          contactUserId: contact.id,
          limit: 50,
          afterId: newestMessage.id,
        });

        if (response.messages.length > 0) {
          const wasNearBottom = isNearBottom();

          queryClient.setQueryData(
            ["contactMessages", accountId, contact.id],
            (oldData: typeof messagesData) => {
              if (!oldData) return oldData;
              const newPages = [...oldData.pages];
              const lastPageIndex = newPages.length - 1;
              newPages[lastPageIndex] = {
                ...newPages[lastPageIndex],
                messages: [
                  ...newPages[lastPageIndex].messages,
                  ...response.messages,
                ],
                hasMoreAfter: response.hasMoreAfter,
              };
              return { ...oldData, pages: newPages };
            },
          );

          setOptimisticMessages((prev) =>
            prev.filter(
              (opt) =>
                !response.messages.some((msg) => msg.content === opt.content),
            ),
          );

          if (wasNearBottom) {
            scrollToBottom();
          }
        }
      } catch {
        // Ignore polling errors
      }
    }, 10000);

    return () => clearInterval(timer);
  }, [
    enabled,
    contact,
    accountId,
    messagesData?.pages,
    transport,
    queryClient,
    isNearBottom,
    scrollToBottom,
  ]);

  // Scroll to bottom when messages first load
  useEffect(() => {
    if (shouldScrollToBottomRef.current && messages.length > 0) {
      scrollToBottom();
      shouldScrollToBottomRef.current = false;
    }
  }, [messages.length, scrollToBottom]);

  // Reset scroll flag when contact changes
  useEffect(() => {
    shouldScrollToBottomRef.current = true;
  }, [contact?.id]);

  // Send message mutation
  const { mutateAsync: mutateSendMessage, isPending: isSending } =
    useMutation(sendContactMessage);

  const handleSend = useCallback(async () => {
    if (!messageInput.trim() || !contact) return;

    const messageContent = messageInput.trim();
    const tempId = `temp-${Date.now()}-${Math.random().toString(36).slice(2)}`;

    setOptimisticMessages((prev) => [
      ...prev,
      {
        tempId,
        content: messageContent,
        sendTime: { seconds: BigInt(Math.floor(Date.now() / 1000)) },
        isOwnMessage: true,
      },
    ]);
    setMessageInput("");

    setTimeout(() => scrollToBottom(), 100);

    try {
      await mutateSendMessage({
        headlessAccountId: accountId,
        contactUserId: contact.id,
        message: messageContent,
      });

      const allMessages = messagesData?.pages.flatMap((p) => p.messages) ?? [];
      const newestMessage = allMessages.reduce(
        (newest, msg) => {
          const newestTime = newest?.sendTime
            ? Number(newest.sendTime.seconds)
            : 0;
          const msgTime = msg.sendTime ? Number(msg.sendTime.seconds) : 0;
          return msgTime > newestTime ? msg : newest;
        },
        null as (typeof allMessages)[0] | null,
      );

      if (newestMessage) {
        const response = await callUnaryMethod(transport, getContactMessages, {
          headlessAccountId: accountId,
          contactUserId: contact.id,
          limit: 50,
          afterId: newestMessage.id,
        });

        if (response.messages.length > 0) {
          queryClient.setQueryData(
            ["contactMessages", accountId, contact.id],
            (oldData: typeof messagesData) => {
              if (!oldData) return oldData;
              const newPages = [...oldData.pages];
              const lastPageIndex = newPages.length - 1;
              newPages[lastPageIndex] = {
                ...newPages[lastPageIndex],
                messages: [
                  ...newPages[lastPageIndex].messages,
                  ...response.messages,
                ],
                hasMoreAfter: response.hasMoreAfter,
              };
              return { ...oldData, pages: newPages };
            },
          );

          setOptimisticMessages((prev) =>
            prev.filter(
              (opt) =>
                !response.messages.some(
                  (msg) => msg.content === opt.content && msg.isOwnMessage,
                ),
            ),
          );
        }
      }
    } catch (e) {
      setOptimisticMessages((prev) => prev.filter((m) => m.tempId !== tempId));
      toast.error(
        e instanceof Error ? e.message : "メッセージの送信に失敗しました",
      );
    }
  }, [
    messageInput,
    contact,
    accountId,
    mutateSendMessage,
    messagesData?.pages,
    transport,
    queryClient,
    scrollToBottom,
  ]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend],
  );

  const renderSessionInvite = (
    invite: SessionInviteData,
    isOwnMessage: boolean,
  ) => {
    const handleClick = () => {
      if (isOwnMessage && invite.sessionId && onNavigateToSession) {
        onNavigateToSession(invite.sessionId);
      }
    };

    return (
      <div
        onClick={handleClick}
        className={cn(
          "rounded-lg overflow-hidden bg-slate-800 text-white min-w-[280px] max-w-[350px]",
          isOwnMessage &&
            onNavigateToSession &&
            "cursor-pointer hover:bg-slate-700 transition-colors",
        )}
      >
        <div className="text-center text-xs text-slate-300 py-1 border-b border-slate-700">
          {formatSessionTime(invite.lastUpdate)}
        </div>
        <div className="flex items-center gap-3 p-3">
          <div className="flex-shrink-0 w-14 h-14 rounded-md overflow-hidden bg-[url('data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxNiIgaGVpZ2h0PSIxNiI+PHJlY3Qgd2lkdGg9IjgiIGhlaWdodD0iOCIgZmlsbD0iIzk5OSIvPjxyZWN0IHg9IjgiIHk9IjgiIHdpZHRoPSI4IiBoZWlnaHQ9IjgiIGZpbGw9IiM5OTkiLz48L3N2Zz4=')]">
            {invite.thumbnailUrl ? (
              <img
                src={resolveUrl(invite.thumbnailUrl)}
                alt={invite.name}
                className="w-full h-full object-cover"
              />
            ) : (
              <div className="w-full h-full" />
            )}
          </div>
          <div className="flex-1 min-w-0">
            <div className="font-bold text-sm truncate">{invite.name}</div>
            <div className="text-xs text-slate-300">
              Host: {invite.hostUsername}
            </div>
            <div className="flex items-center gap-1 text-xs text-slate-300">
              <span className="w-2 h-2 rounded-full bg-green-500" />
              Users: {invite.joinedUsers} ({invite.activeUsers})
            </div>
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className={cn("h-full flex flex-col", className)}>
      {/* Messages area */}
      <div ref={scrollContainerRef} className="flex-1 overflow-auto p-4">
        {!contact && showPlaceholder ? (
          <div className="h-full flex items-center justify-center text-muted-foreground">
            コンタクトを選択してください
          </div>
        ) : isLoadingMessages ? (
          <div className="flex justify-center p-4">
            <Loader2 className="animate-spin" />
          </div>
        ) : messages.length === 0 ? (
          <div className="h-full flex items-center justify-center text-muted-foreground">
            メッセージがありません
          </div>
        ) : (
          <div
            style={{
              height: virtualizer.getTotalSize(),
              position: "relative",
            }}
          >
            {isFetchingPreviousPage && (
              <div className="flex justify-center p-2">
                <Loader2 className="h-4 w-4 animate-spin" />
              </div>
            )}
            {virtualizer.getVirtualItems().map((virtualItem) => {
              const message = messages[virtualItem.index];
              const isSessionInvite =
                message.type === ContactChatMessageType.SESSION_INVITE;
              const sessionInvite = isSessionInvite
                ? parseSessionInvite(message.content)
                : null;

              return (
                <div
                  key={message.id}
                  data-index={virtualItem.index}
                  ref={virtualizer.measureElement}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    transform: `translateY(${virtualItem.start}px)`,
                  }}
                  className={cn(
                    "flex pb-2 px-2",
                    message.isOwnMessage ? "justify-end" : "justify-start",
                  )}
                >
                  {isSessionInvite && sessionInvite ? (
                    renderSessionInvite(sessionInvite, message.isOwnMessage)
                  ) : (
                    <div
                      className={cn(
                        "max-w-[70%] rounded-lg px-3 py-2",
                        message.isOwnMessage
                          ? "bg-primary text-primary-foreground"
                          : "bg-muted",
                      )}
                    >
                      <div className="text-sm break-words whitespace-pre-wrap">
                        {message.content}
                      </div>
                      <div
                        className={cn(
                          "text-xs mt-1",
                          message.isOwnMessage
                            ? "text-primary-foreground/70"
                            : "text-muted-foreground",
                        )}
                      >
                        {formatMessageTime(message.sendTime)}
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Input area */}
      {contact && (
        <div className="p-2 border-t flex gap-2">
          <Input
            value={messageInput}
            onChange={(e) => setMessageInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="メッセージを入力..."
            disabled={isSending}
            className="flex-1"
          />
          <Button
            onClick={handleSend}
            disabled={!messageInput.trim() || isSending}
            size="icon"
          >
            {isSending ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Send className="h-4 w-4" />
            )}
          </Button>
        </div>
      )}
    </div>
  );
}
