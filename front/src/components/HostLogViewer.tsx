import { callUnaryMethod, useTransport } from "@connectrpc/connect-query";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import {
  getHeadlessHostLogs,
  searchHeadlessHostLogs,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import type { GetHeadlessHostLogsRequest } from "../../pbgen/hdlctrl/v1/controller_pb";
import { Card, CardContent, CardHeader } from "./ui/card";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import {
  ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  ArrowDownToLine,
  ChevronDown,
  ChevronUp,
  Download,
  Loader2,
} from "lucide-react";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { cn } from "@/libs/cssUtils";

// ページの取得位置。case: undefined は最新のログから
type LogCursor = GetHeadlessHostLogsRequest["cursor"];

type SearchDirection = "older" | "newer";

type HostLogViewerProps = {
  hostId: string;
  instanceId: number;
  tailing: boolean;
  height?: string;
};

const PAGE_SIZE = 100;

// tailing で 1 回に取得する件数 (サーバー側の上限)。溜まった新着に少ない往復で追いつくため大きくする
const TAIL_FETCH_LIMIT = 1000;

// ホスト停止後も新着の取得を続ける時間 (ms)。
// 停止直前の出力はログ収集を経由して遅れて DB に届くので、停止と同時に止めると最後の数行を取りこぼす
const TAIL_LINGER_MS = 60_000;

// ログ 1 行の高さ (px)。折り返さないので固定
const ROW_HEIGHT_PX = 20;

// この距離 (px) 以内なら最下部にいるとみなす
const BOTTOM_THRESHOLD_PX = 30;

// 指定行が中央に来るようにスクロールする。行の高さは固定なので位置は直接計算できる
function scrollRowToCenter(container: HTMLElement, index: number) {
  container.scrollTop =
    index * ROW_HEIGHT_PX - (container.clientHeight - ROW_HEIGHT_PX) / 2;
}

// value が true から false になった後も、lingerMs の間は true を返す
function useLinger(value: boolean, lingerMs: number) {
  const [prevValue, setPrevValue] = useState(value);
  const [isLingering, setIsLingering] = useState(false);
  if (prevValue !== value) {
    setPrevValue(value);
    setIsLingering(!value);
  }

  useEffect(() => {
    if (!isLingering) return;
    const timer = setTimeout(() => setIsLingering(false), lingerMs);
    return () => clearTimeout(timer);
  }, [isLingering, lingerMs]);

  return value || isLingering;
}

function escapeRegExp(text: string) {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

// regex はキャプチャグループ 1 つで検索文字列全体を囲んだもの (split の奇数番目が一致箇所になる)
function highlightMatches(
  body: string,
  regex: RegExp,
  isCurrentMatch: boolean,
): ReactNode {
  const parts = body.split(regex);
  if (parts.length === 1) return body;

  return parts.map((part, i) =>
    i % 2 === 1 ? (
      <mark
        key={i}
        className={cn(
          "text-black",
          isCurrentMatch ? "bg-orange-400" : "bg-yellow-300",
        )}
      >
        {part}
      </mark>
    ) : (
      part
    ),
  );
}

export default function HostLogViewer(props: HostLogViewerProps) {
  // ホスト/インスタンスが変わったら検索状態やスクロール位置をすべて破棄する
  return (
    <HostLogViewerInner
      key={`${props.hostId}:${props.instanceId}`}
      {...props}
    />
  );
}

function HostLogViewerInner({
  hostId,
  instanceId,
  tailing,
  height = "30rem",
}: HostLogViewerProps) {
  const { t } = useTranslation();
  const transport = useTransport();
  const queryClient = useQueryClient();

  const scrollContainerRef = useRef<HTMLDivElement>(null);
  // スクロール位置調整用
  const prevLogsLengthRef = useRef(0);
  const prevFirstLogIdRef = useRef<bigint | null>(null);
  const prevHasNextPageRef = useRef(false);
  // 初期位置へのスクロールを済ませたビュー (queryKey)。ビューが切り替わるたびに 1 回だけ位置決めする
  const positionedViewRef = useRef<unknown>(null);

  // 検索結果へジャンプした場合の中心ログID。null なら最新のログから表示する
  const [anchorId, setAnchorId] = useState<bigint | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [currentMatchId, setCurrentMatchId] = useState<bigint | null>(null);
  const [searchState, setSearchState] = useState<
    "idle" | "searching" | "noMatch"
  >("idle");
  // 実行中の検索。検索文字列や表示位置が変わったら打ち切る
  const searchAbortRef = useRef<AbortController | null>(null);

  const queryKey = useMemo(
    // bigint は queryKey のハッシュ化 (JSON.stringify) で扱えないので文字列にする
    () => ["hostLogs", hostId, instanceId, anchorId?.toString() ?? null],
    [hostId, instanceId, anchorId],
  );

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
    queryKey,
    queryFn: async ({ pageParam }: { pageParam: LogCursor }) => {
      const response = await callUnaryMethod(transport, getHeadlessHostLogs, {
        hostId,
        instanceId,
        limit: PAGE_SIZE,
        cursor: pageParam,
      });

      return {
        logs: response.logs,
        hasMoreBefore: response.hasMoreBefore,
        hasMoreAfter: response.hasMoreAfter,
      };
    },
    initialPageParam: (anchorId !== null
      ? { case: "aroundId", value: anchorId }
      : { case: undefined }) as LogCursor,
    getNextPageParam: (lastPage): LogCursor | undefined => {
      if (!lastPage.hasMoreAfter) return undefined;
      const lastLog = lastPage.logs[lastPage.logs.length - 1];
      if (!lastLog) return undefined;
      return { case: "afterId", value: lastLog.id };
    },
    getPreviousPageParam: (firstPage): LogCursor | undefined => {
      if (!firstPage.hasMoreBefore) return undefined;
      const firstLog = firstPage.logs[0];
      if (!firstLog) return undefined;
      return { case: "beforeId", value: firstLog.id };
    },
    // ページはスクロールと tailing で継ぎ足していくので、自動 refetch で作り直させない
    // (Infinity だとページ取得に失敗した後のフォーカス復帰などで refetch されてしまう)。
    // 表示位置 (anchorId) を切り替えたら前のキャッシュは捨て、戻ってきたときは取得し直す
    staleTime: "static",
    gcTime: 0,
  });

  const logs = useMemo(
    () => data?.pages.flatMap((page) => page.logs) ?? [],
    [data?.pages],
  );
  const isLoaded = data !== undefined;

  // 仮想スクロール設定
  const virtualizer = useVirtualizer({
    count: logs.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => ROW_HEIGHT_PX,
    overscan: 10,
  });

  // ログ変化を検知してスクロール位置を調整。
  // 描画前に同期的に合わせる (次のフレームまで遅らせると、合わせる前の位置で発火した
  // スクロールイベントが余計なページ取得を起こし、その結果とも競合する)
  useLayoutEffect(() => {
    const container = scrollContainerRef.current;
    const currentLength = logs.length;
    const prevLength = prevLogsLengthRef.current;
    const currentFirstLogId = logs[0]?.id ?? null;
    const prevFirstLogId = prevFirstLogIdRef.current;
    const hadNextPage = prevHasNextPageRef.current;

    prevLogsLengthRef.current = currentLength;
    prevFirstLogIdRef.current = currentFirstLogId;
    prevHasNextPageRef.current = hasNextPage;

    if (!container || currentLength === 0) return;

    if (positionedViewRef.current !== queryKey) {
      positionedViewRef.current = queryKey;
      // 初期ロード - 検索結果へのジャンプならその行、それ以外は最下部にスクロール
      const anchorIndex =
        anchorId !== null ? logs.findIndex((log) => log.id === anchorId) : -1;
      if (anchorIndex >= 0) {
        scrollRowToCenter(container, anchorIndex);
      } else {
        container.scrollTop = container.scrollHeight;
      }
    } else if (currentFirstLogId !== prevFirstLogId) {
      // 古いログが先頭に追加された - 追加分だけずらして同じ位置を維持
      const prependedCount = logs.findIndex((log) => log.id === prevFirstLogId);
      if (prependedCount > 0) {
        container.scrollTop += prependedCount * ROW_HEIGHT_PX;
      }
    } else if (currentLength > prevLength && !hadNextPage) {
      // 末尾まで読み込み済みの状態で新しいログが届いた - 最下部を見ていた場合のみ追従
      // (途中から下方向へページングしている間は追従しない。追従すると末尾まで連鎖的に読み込んでしまう)
      // 追加前の高さで判定する。scrollTop は末尾への追加では変わらない
      const wasAtBottom =
        prevLength * ROW_HEIGHT_PX -
          container.scrollTop -
          container.clientHeight <
        BOTTOM_THRESHOLD_PX;
      if (wasAtBottom) {
        container.scrollTop = container.scrollHeight;
      }
    }
  }, [logs, queryKey, anchorId, hasNextPage]);

  // スクロールイベント監視
  useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container) return;

    const handleScroll = () => {
      const distanceToBottom =
        container.scrollHeight - container.scrollTop - container.clientHeight;
      const isFetching = isFetchingNextPage || isFetchingPreviousPage;

      // 上端到達 → 古いログ取得
      if (container.scrollTop < 100 && hasPreviousPage && !isFetching) {
        fetchPreviousPage();
      }
      // 下端到達 → 新しいログ取得 (末尾まで読み込み済みなら tailing に任せる)
      if (distanceToBottom < 100 && hasNextPage && !isFetching) {
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
  ]);

  // Tailing - 直接APIを呼び出して新しいログをマージ
  const logsRef = useRef(logs);
  logsRef.current = logs;
  const isTailingFetchingRef = useRef(false);
  const isPollingNewLogs = useLinger(tailing, TAIL_LINGER_MS);

  useEffect(() => {
    // 末尾まで読み込んでいない間 (検索結果へジャンプした直後など) はスクロールでのページングに任せる
    // 初回ロードが済むまでは「まだ 1 行も無い」と区別できないので待つ
    if (!isPollingNewLogs || hasNextPage || !isLoaded) return;

    let cancelled = false;

    const fetchNewLogs = async () => {
      if (isTailingFetchingRef.current) return;
      isTailingFetchingRef.current = true;

      try {
        const currentLogs = logsRef.current;
        // まだ 1 行も無い場合 (起動直後など) は最初のログから取得する
        const lastLogId = currentLogs[currentLogs.length - 1]?.id ?? 0n;

        const pageParam: LogCursor = { case: "afterId", value: lastLogId };
        const newLogs: typeof currentLogs = [];
        let cursor = pageParam;
        let hasMoreAfter = true;

        // 1回で取り切れない量が溜まっていたら追いつくまで続けて取得する
        while (hasMoreAfter && !cancelled) {
          const response = await callUnaryMethod(
            transport,
            getHeadlessHostLogs,
            { hostId, instanceId, limit: TAIL_FETCH_LIMIT, cursor },
          );
          newLogs.push(...response.logs);
          hasMoreAfter = response.hasMoreAfter && response.logs.length > 0;
          if (hasMoreAfter) {
            cursor = {
              case: "afterId",
              value: response.logs[response.logs.length - 1].id,
            };
          }
        }

        if (newLogs.length === 0) return;

        // 途中で打ち切られた場合は hasMoreAfter が true のまま残り、続きはスクロールでのページングが取得する
        queryClient.setQueryData<typeof data>(queryKey, (old) => {
          // 取得中に表示内容が入れ替わっていたら (検索結果へのジャンプなど) 継ぎ足さない
          const oldLastLogs = old?.pages[old.pages.length - 1]?.logs;
          const oldLastLogId = oldLastLogs?.[oldLastLogs.length - 1]?.id ?? 0n;
          if (!old || oldLastLogId !== lastLogId) return old;
          return {
            pages: [
              ...old.pages,
              { logs: newLogs, hasMoreBefore: true, hasMoreAfter },
            ],
            pageParams: [...old.pageParams, pageParam],
          };
        });
      } finally {
        isTailingFetchingRef.current = false;
      }
    };

    const timer = setInterval(fetchNewLogs, 2000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [
    isPollingNewLogs,
    hasNextPage,
    isLoaded,
    transport,
    hostId,
    instanceId,
    queryClient,
    queryKey,
  ]);

  const isLoading = isPending || isFetchingNextPage || isFetchingPreviousPage;

  const virtualItems = virtualizer.getVirtualItems();

  // 検索
  const highlightRegex = useMemo(
    () =>
      searchQuery !== ""
        ? new RegExp(`(${escapeRegExp(searchQuery)})`, "gi")
        : null,
    [searchQuery],
  );

  // 実行中の検索を打ち切り、現在の一致を解除する
  const resetSearch = () => {
    searchAbortRef.current?.abort();
    setCurrentMatchId(null);
    setSearchState("idle");
  };

  useEffect(() => () => searchAbortRef.current?.abort(), []);

  const handleSearchQueryChange = (value: string) => {
    resetSearch();
    setSearchQuery(value);
  };

  const canSearch =
    searchQuery !== "" && logs.length > 0 && searchState !== "searching";

  const runSearch = async (direction: SearchDirection) => {
    if (!canSearch) return;
    const currentLogs = logsRef.current;

    // 検索の起点: 現在の一致行があればその次から。
    // 無ければ、いま見えている範囲も検索対象に含めるため表示範囲の端から探す
    let cursorId: bigint;
    if (currentMatchId !== null) {
      cursorId = currentMatchId;
    } else {
      const lastIndex = currentLogs.length - 1;
      const range = virtualizer.range;
      if (direction === "older") {
        const bottomIndex = Math.min(range?.endIndex ?? lastIndex, lastIndex);
        cursorId = currentLogs[bottomIndex].id + 1n;
      } else {
        const topIndex = Math.min(range?.startIndex ?? 0, lastIndex);
        cursorId = currentLogs[topIndex].id - 1n;
      }
    }

    const abort = new AbortController();
    searchAbortRef.current = abort;
    setSearchState("searching");

    try {
      const response = await callUnaryMethod(
        transport,
        searchHeadlessHostLogs,
        {
          hostId,
          instanceId,
          query: searchQuery,
          cursor:
            direction === "older"
              ? { case: "beforeId" as const, value: cursorId }
              : { case: "afterId" as const, value: cursorId },
        },
        { signal: abort.signal },
      );
      if (abort.signal.aborted) return;

      const matchId = response.logId;
      if (matchId === undefined) {
        setSearchState("noMatch");
        return;
      }

      setSearchState("idle");
      setCurrentMatchId(matchId);

      const loadedIndex = logsRef.current.findIndex(
        (log) => log.id === matchId,
      );
      if (loadedIndex >= 0) {
        if (scrollContainerRef.current) {
          scrollRowToCenter(scrollContainerRef.current, loadedIndex);
        }
      } else {
        // 未ロードの位置なので、一致行を中心に読み込み直す
        setAnchorId(matchId);
      }
    } catch (error) {
      if (abort.signal.aborted) return;
      setSearchState("idle");
      toast.error(
        error instanceof Error
          ? error.message
          : t("hostLogViewer.searchFailed"),
      );
    }
  };

  const handleJumpToLatest = () => {
    resetSearch();
    setAnchorId(null);
  };

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
        limit: PAGE_SIZE,
      });
      allLogs.push(...response.logs);

      // 古いログを全て取得
      while (response.hasMoreBefore && response.logs.length > 0) {
        const firstLog = response.logs[0];
        response = await callUnaryMethod(transport, getHeadlessHostLogs, {
          hostId,
          instanceId,
          limit: PAGE_SIZE,
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
          : t("hostLogViewer.downloadFailed"),
      );
    } finally {
      setIsDownloading(false);
    }
  }, [transport, hostId, instanceId, t]);

  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
        <h3 className="text-lg font-semibold">Logs</h3>
        <div className="flex flex-wrap items-center justify-end gap-2">
          <div className="flex items-center gap-1">
            {searchState === "searching" && (
              <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
            )}
            {searchState === "noMatch" && (
              <span className="text-sm text-muted-foreground" role="status">
                {t("hostLogViewer.searchNoMatch")}
              </span>
            )}
            <Input
              type="search"
              className="h-8 w-56"
              placeholder={t("hostLogViewer.searchPlaceholder")}
              aria-label={t("hostLogViewer.searchPlaceholder")}
              value={searchQuery}
              maxLength={200}
              onChange={(e) => handleSearchQueryChange(e.target.value)}
              onKeyDown={(e) => {
                // IME の変換確定の Enter では検索しない
                // (Safari は確定の Enter で isComposing が false になるので keyCode も見る)
                if (
                  e.key !== "Enter" ||
                  e.nativeEvent.isComposing ||
                  e.keyCode === 229
                ) {
                  return;
                }
                e.preventDefault();
                runSearch(e.shiftKey ? "newer" : "older");
              }}
            />
            <Button
              variant="outline"
              size="icon"
              className="size-8"
              title={t("hostLogViewer.searchOlder")}
              aria-label={t("hostLogViewer.searchOlder")}
              onClick={() => runSearch("older")}
              disabled={!canSearch}
            >
              <ChevronUp className="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              size="icon"
              className="size-8"
              title={t("hostLogViewer.searchNewer")}
              aria-label={t("hostLogViewer.searchNewer")}
              onClick={() => runSearch("newer")}
              disabled={!canSearch}
            >
              <ChevronDown className="h-4 w-4" />
            </Button>
          </div>
          {anchorId !== null && (
            <Button variant="outline" size="sm" onClick={handleJumpToLatest}>
              <ArrowDownToLine className="h-4 w-4 mr-1" />
              {t("hostLogViewer.jumpToLatest")}
            </Button>
          )}
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
            {isDownloading
              ? t("hostLogViewer.downloading")
              : t("hostLogViewer.download")}
          </Button>
        </div>
      </CardHeader>
      <CardContent className="relative" style={{ height }}>
        {isFetchingPreviousPage && (
          <div className="pointer-events-none absolute inset-x-0 top-0 z-10 flex justify-center py-2 text-sm text-muted-foreground">
            <span className="rounded bg-background/90 px-2">
              {t("common.loading")}
            </span>
          </div>
        )}
        <div
          ref={scrollContainerRef}
          className="absolute inset-0 overflow-auto [overflow-anchor:none]"
        >
          {isLoading && logs.length === 0 && (
            <div className="flex justify-center py-4 text-muted-foreground">
              {t("common.loading")}
            </div>
          )}
          {!isLoading && logs.length === 0 && (
            <div className="flex justify-center py-4 text-muted-foreground">
              {t("hostLogViewer.noLogs")}
            </div>
          )}
          {logs.length > 0 && (
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
                const isCurrentMatch = log.id === currentMatchId;
                return (
                  <div
                    key={virtualItem.key}
                    style={{
                      position: "absolute",
                      top: 0,
                      left: 0,
                      minWidth: "100%",
                      height: ROW_HEIGHT_PX,
                      transform: `translateY(${virtualItem.start}px)`,
                    }}
                    className={cn(
                      "font-mono text-sm whitespace-nowrap pr-4",
                      isCurrentMatch && "bg-primary/15",
                    )}
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
                      {highlightRegex
                        ? highlightMatches(
                            log.body,
                            highlightRegex,
                            isCurrentMatch,
                          )
                        : log.body}
                    </span>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
