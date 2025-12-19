import { callUnaryMethod, useTransport } from "@connectrpc/connect-query";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { getHeadlessHostLogs } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { Card, CardContent, CardHeader } from "./ui/card";
import { Button } from "./ui/button";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Download, Loader2 } from "lucide-react";
import { toast } from "sonner";

type PageParam = {
  direction: "before" | "after" | "init";
  cursorId?: bigint;
};

export default function HostLogViewer({
  hostId,
  instanceId,
  tailing,
  height = "30rem",
}: {
  hostId: string;
  instanceId: number;
  tailing: boolean;
  height?: string;
}) {
  const transport = useTransport();
  const queryClient = useQueryClient();

  const scrollContainerRef = useRef<HTMLDivElement>(null);
  // スクロール位置調整用
  const prevLogsLengthRef = useRef(0);
  const prevFirstLogIdRef = useRef<bigint | null>(null);
  const scrollAdjustmentRef = useRef<{
    type: "init" | "before" | "after";
    prevFirstVisibleIndex?: number;
    addedCount?: number;
  } | null>(null);

  const {
    data,
    fetchNextPage,
    fetchPreviousPage,
    hasNextPage,
    hasPreviousPage,
    isFetchingNextPage,
    isFetchingPreviousPage,
    isPending,
  } = useInfiniteQuery({
    queryKey: ["hostLogs", hostId, instanceId],
    queryFn: async ({ pageParam }: { pageParam: PageParam }) => {
      let cursor:
        | { case: "beforeId"; value: bigint }
        | { case: "afterId"; value: bigint }
        | undefined = undefined;

      if (pageParam.direction === "before" && pageParam.cursorId) {
        cursor = { case: "beforeId" as const, value: pageParam.cursorId };
      } else if (pageParam.direction === "after" && pageParam.cursorId) {
        cursor = { case: "afterId" as const, value: pageParam.cursorId };
      }

      const response = await callUnaryMethod(transport, getHeadlessHostLogs, {
        hostId,
        instanceId,
        limit: 100,
        cursor,
      });

      return {
        logs: response.logs,
        hasMoreBefore: response.hasMoreBefore,
        hasMoreAfter: response.hasMoreAfter,
        direction: pageParam.direction,
      };
    },
    initialPageParam: { direction: "init" } as PageParam,
    getNextPageParam: (lastPage) => {
      if (!lastPage.hasMoreAfter) return undefined;
      const lastLog = lastPage.logs[lastPage.logs.length - 1];
      if (!lastLog) return undefined;
      return { direction: "after", cursorId: lastLog.id } as PageParam;
    },
    getPreviousPageParam: (firstPage) => {
      if (!firstPage.hasMoreBefore) return undefined;
      const firstLog = firstPage.logs[0];
      if (!firstLog) return undefined;
      return { direction: "before", cursorId: firstLog.id } as PageParam;
    },
  });

  const logs = useMemo(
    () => data?.pages.flatMap((page) => page.logs) ?? [],
    [data?.pages],
  );

  // 仮想スクロール設定
  const virtualizer = useVirtualizer({
    count: logs.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => 20,
    overscan: 10,
  });

  // ログ変化を検知してスクロール調整
  useEffect(() => {
    const currentLength = logs.length;
    const prevLength = prevLogsLengthRef.current;
    const currentFirstLogId = logs[0]?.id ?? null;
    const prevFirstLogId = prevFirstLogIdRef.current;

    if (currentLength > 0 && prevLength === 0) {
      // 初期ロード - 最下部にスクロール
      scrollAdjustmentRef.current = { type: "init" };
    } else if (currentLength > prevLength && prevLength > 0) {
      // ログが追加された
      const addedCount = currentLength - prevLength;

      // 先頭のログIDが変わった = 先頭にログ追加 (before)
      // 先頭のログIDが同じ = 末尾にログ追加 (after)
      if (currentFirstLogId !== prevFirstLogId) {
        // 古いログが先頭に追加された - 現在位置を維持
        const firstVisibleIndex = virtualizer.range?.startIndex ?? 0;
        scrollAdjustmentRef.current = {
          type: "before",
          prevFirstVisibleIndex: firstVisibleIndex,
          addedCount,
        };
      } else {
        // 新しいログが末尾に追加された - 最下部にスクロール
        scrollAdjustmentRef.current = { type: "after" };
      }
    }

    prevLogsLengthRef.current = currentLength;
    prevFirstLogIdRef.current = currentFirstLogId;
  }, [logs, virtualizer]);

  // スクロール位置調整の実行
  useEffect(() => {
    const adjustment = scrollAdjustmentRef.current;
    if (!adjustment || logs.length === 0) return;

    // requestAnimationFrame で次のフレームで実行（virtualizer更新後）
    requestAnimationFrame(() => {
      if (adjustment.type === "init" || adjustment.type === "after") {
        // 最下部にスクロール
        virtualizer.scrollToIndex(logs.length - 1, { align: "end" });
      } else if (adjustment.type === "before" && adjustment.addedCount) {
        // 古いログ追加時: 追加分だけインデックスをずらして同じ位置を維持
        const newIndex =
          (adjustment.prevFirstVisibleIndex ?? 0) + adjustment.addedCount;
        virtualizer.scrollToIndex(newIndex, { align: "start" });
      }
      scrollAdjustmentRef.current = null;
    });
  }, [logs.length, virtualizer]);

  // スクロールイベント監視
  useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container) return;

    const handleScroll = () => {
      const isFetching = isFetchingNextPage || isFetchingPreviousPage;

      // 上端到達 → 古いログ取得
      if (container.scrollTop < 100 && hasPreviousPage && !isFetching) {
        fetchPreviousPage();
      }
      // 下端到達 → 新しいログ取得 (tailing無効時)
      if (
        container.scrollHeight - container.scrollTop - container.clientHeight <
          100 &&
        hasNextPage &&
        !tailing &&
        !isFetching
      ) {
        fetchNextPage();
      }
    };

    container.addEventListener("scroll", handleScroll);
    return () => container.removeEventListener("scroll", handleScroll);
  }, [
    fetchNextPage,
    fetchPreviousPage,
    hasNextPage,
    hasPreviousPage,
    isFetchingNextPage,
    isFetchingPreviousPage,
    tailing,
  ]);

  // Tailing - 直接APIを呼び出して新しいログをマージ
  const logsRef = useRef(logs);
  logsRef.current = logs;
  const isTailingFetchingRef = useRef(false);

  useEffect(() => {
    if (!tailing) return;

    const fetchNewLogs = async () => {
      if (isTailingFetchingRef.current) return;
      isTailingFetchingRef.current = true;

      const currentLogs = logsRef.current;
      const lastLog = currentLogs[currentLogs.length - 1];
      if (!lastLog) {
        isTailingFetchingRef.current = false;
        return;
      }

      try {
        const response = await callUnaryMethod(transport, getHeadlessHostLogs, {
          hostId,
          instanceId,
          limit: 100,
          cursor: { case: "afterId" as const, value: lastLog.id },
        });

        if (response.logs.length > 0) {
          queryClient.setQueryData<typeof data>(
            ["hostLogs", hostId, instanceId],
            (old) => {
              if (!old) return old;
              return {
                ...old,
                pages: [
                  ...old.pages,
                  {
                    logs: response.logs,
                    hasMoreBefore: true,
                    hasMoreAfter: response.hasMoreAfter,
                    direction: "after" as const,
                  },
                ],
                pageParams: [
                  ...old.pageParams,
                  { direction: "after", cursorId: lastLog.id } as PageParam,
                ],
              };
            },
          );
        }
      } finally {
        isTailingFetchingRef.current = false;
      }
    };

    const timer = setInterval(fetchNewLogs, 2000);
    return () => clearInterval(timer);
  }, [tailing, transport, hostId, instanceId, queryClient]);

  const isLoading = isPending || isFetchingNextPage || isFetchingPreviousPage;

  const virtualItems = virtualizer.getVirtualItems();

  const [isDownloading, setIsDownloading] = useState(false);

  const handleDownload = useCallback(async () => {
    setIsDownloading(true);

    try {
      // 全ログを取得
      type LogEntry = (typeof logs)[number];
      const allLogs: LogEntry[] = [];

      // 最初のページを取得
      let response = await callUnaryMethod(transport, getHeadlessHostLogs, {
        hostId,
        instanceId,
        limit: 100,
      });
      allLogs.push(...response.logs);

      // 古いログを全て取得
      while (response.hasMoreBefore && response.logs.length > 0) {
        const firstLog = response.logs[0];
        response = await callUnaryMethod(transport, getHeadlessHostLogs, {
          hostId,
          instanceId,
          limit: 100,
          cursor: { case: "beforeId" as const, value: firstLog.id },
        });
        allLogs.unshift(...response.logs);
      }

      if (allLogs.length === 0) return;

      const content = allLogs
        .map((log) => {
          const timestamp = log.timestamp
            ? new Date(Number(log.timestamp.seconds) * 1000).toISOString()
            : "";
          return `${timestamp} ${log.body}`;
        })
        .join("\n");

      const blob = new Blob([content], { type: "text/plain" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `host-${hostId}-instance-${instanceId}.log`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : "ログのダウンロードに失敗しました",
      );
    } finally {
      setIsDownloading(false);
    }
  }, [transport, hostId, instanceId]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <h3 className="text-lg font-semibold">Logs</h3>
        <Button
          variant="outline"
          size="sm"
          onClick={handleDownload}
          disabled={logs.length === 0 || isDownloading}
        >
          {isDownloading ? (
            <Loader2 className="h-4 w-4 mr-1 animate-spin" />
          ) : (
            <Download className="h-4 w-4 mr-1" />
          )}
          {isDownloading ? "Downloading..." : "Download"}
        </Button>
      </CardHeader>
      <CardContent className="relative" style={{ height }}>
        <div
          ref={scrollContainerRef}
          className="absolute inset-0 overflow-auto"
        >
          {isLoading && logs.length === 0 && (
            <div className="flex justify-center py-4 text-muted-foreground">
              読み込み中...
            </div>
          )}
          {!isLoading && logs.length === 0 && (
            <div className="flex justify-center py-4 text-muted-foreground">
              ログがありません
            </div>
          )}
          {logs.length > 0 && (
            <>
              {isFetchingPreviousPage && hasPreviousPage && (
                <div className="flex justify-center py-2 text-muted-foreground text-sm">
                  読み込み中...
                </div>
              )}
              <div
                style={{
                  height: `${virtualizer.getTotalSize()}px`,
                  minWidth: "100%",
                  width: "max-content",
                  position: "relative",
                }}
              >
                {virtualItems.map((virtualItem) => {
                  const log = logs[virtualItem.index];
                  return (
                    <div
                      key={virtualItem.key}
                      style={{
                        position: "absolute",
                        top: 0,
                        left: 0,
                        height: "20px",
                        transform: `translateY(${virtualItem.start}px)`,
                      }}
                      className="font-mono text-sm whitespace-nowrap pr-4"
                    >
                      <span
                        className={
                          log.isError ? "text-destructive" : "text-foreground"
                        }
                      >
                        {log.timestamp
                          ? new Date(
                              Number(log.timestamp.seconds) * 1000,
                            ).toLocaleTimeString("ja-JP") + " "
                          : ""}
                        {log.body}
                      </span>
                    </div>
                  );
                })}
              </div>
            </>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
