import { useState, useEffect, useCallback } from "react";
import { useMutation } from "@connectrpc/connect-query";
import {
  searchWorlds,
  getOwnWorlds,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  Button,
  Checkbox,
  Label,
} from "./ui";
import { Input } from "./ui/input";
import { Search, ChevronLeft, ChevronRight, Star } from "lucide-react";
import { ScrollBase } from "./base";
import { useDebounce } from "../hooks/useDebounce";
import { resolveUrl } from "@/libs/skyfrostUtils";
import { RichText } from "./base/RichText";
import type { SearchWorldsResponse_WorldRecord } from "../../pbgen/hdlctrl/v1/controller_pb";

type TabType = "search" | "own";

interface WorldSearchDialogProps {
  open: boolean;
  onClose: () => void;
  onSelect: (worldUrl: string) => void;
  hostId?: string;
}

export function WorldSearchDialog({
  open,
  onClose,
  onSelect,
  hostId,
}: WorldSearchDialogProps) {
  const [activeTab, setActiveTab] = useState<TabType>("search");
  const [query, setQuery] = useState("");
  const [featuredOnly, setFeaturedOnly] = useState(false);
  const [pageIndex, setPageIndex] = useState(0);
  const [ownPageIndex, setOwnPageIndex] = useState(0);

  const debouncedQuery = useDebounce(query, 300);

  const {
    data: searchResult,
    mutateAsync: mutateSearch,
    isPending: isSearchPending,
  } = useMutation(searchWorlds);

  const {
    data: ownWorldsResult,
    mutateAsync: mutateOwnWorlds,
    isPending: isOwnWorldsPending,
  } = useMutation(getOwnWorlds);

  const doSearch = useCallback(
    (searchQuery: string, featured: boolean, page: number) => {
      mutateSearch({
        query: searchQuery,
        featuredOnly: featured,
        pageIndex: page,
      });
    },
    [mutateSearch],
  );

  const doFetchOwnWorlds = useCallback(
    (page: number) => {
      if (!hostId) return;
      mutateOwnWorlds({
        hostId,
        pageIndex: page,
      });
    },
    [mutateOwnWorlds, hostId],
  );

  // Trigger search when debounced query or filters change
  useEffect(() => {
    if (open && activeTab === "search") {
      doSearch(debouncedQuery, featuredOnly, pageIndex);
    }
  }, [open, activeTab, debouncedQuery, featuredOnly, pageIndex, doSearch]);

  // Trigger own worlds fetch
  useEffect(() => {
    if (open && activeTab === "own") {
      doFetchOwnWorlds(ownPageIndex);
    }
  }, [open, activeTab, ownPageIndex, doFetchOwnWorlds]);

  // Reset state when dialog opens
  useEffect(() => {
    if (open) {
      setActiveTab("search");
      setQuery("");
      setFeaturedOnly(false);
      setPageIndex(0);
      setOwnPageIndex(0);
    }
  }, [open]);

  const handleSelect = (ownerId: string, recordId: string) => {
    const worldUrl = `resrec:///${ownerId}/${recordId}`;
    onSelect(worldUrl);
    onClose();
  };

  const handleFeaturedChange = (checked: boolean) => {
    setFeaturedOnly(checked);
    setPageIndex(0);
  };

  const handlePrevPage = () => {
    if (activeTab === "search") {
      if (pageIndex > 0) setPageIndex(pageIndex - 1);
    } else {
      if (ownPageIndex > 0) setOwnPageIndex(ownPageIndex - 1);
    }
  };

  const handleNextPage = () => {
    if (activeTab === "search") {
      if (searchResult?.hasMore) setPageIndex(pageIndex + 1);
    } else {
      if (ownWorldsResult?.hasMore) setOwnPageIndex(ownPageIndex + 1);
    }
  };

  const handleTabChange = (tab: TabType) => {
    setActiveTab(tab);
  };

  const isPending =
    activeTab === "search" ? isSearchPending : isOwnWorldsPending;
  const currentPageIndex = activeTab === "search" ? pageIndex : ownPageIndex;
  const hasMore =
    activeTab === "search" ? searchResult?.hasMore : ownWorldsResult?.hasMore;
  const records: SearchWorldsResponse_WorldRecord[] | undefined =
    activeTab === "search" ? searchResult?.records : ownWorldsResult?.records;

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="sm:max-w-[1000px] max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>ワールド検索</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 flex-1 flex flex-col min-h-0">
          {/* Tab buttons */}
          {hostId && (
            <div className="flex gap-2">
              <Button
                type="button"
                variant={activeTab === "search" ? "default" : "outline"}
                size="sm"
                onClick={() => handleTabChange("search")}
              >
                ワールド検索
              </Button>
              <Button
                type="button"
                variant={activeTab === "own" ? "default" : "outline"}
                size="sm"
                onClick={() => handleTabChange("own")}
              >
                自分のワールド
              </Button>
            </div>
          )}

          {/* Search input and featured checkbox (search tab only) */}
          {activeTab === "search" && (
            <div className="flex gap-4 items-center">
              <div className="relative flex-1">
                <Search className="absolute left-3 top-3 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="検索ワード..."
                  value={query}
                  onChange={(e) => {
                    setQuery(e.target.value);
                    setPageIndex(0);
                  }}
                  className="pl-10"
                />
              </div>
              <div className="flex items-center gap-2">
                <Checkbox
                  id="featuredOnly"
                  checked={featuredOnly}
                  onCheckedChange={handleFeaturedChange}
                />
                <Label
                  htmlFor="featuredOnly"
                  className="text-sm whitespace-nowrap"
                >
                  Featuredのみ
                </Label>
              </div>
            </div>
          )}

          {/* Results list */}
          <ScrollBase height="60vh">
            {isPending ? (
              <div className="flex items-center justify-center h-32 text-muted-foreground">
                {activeTab === "search" ? "検索中..." : "読み込み中..."}
              </div>
            ) : records && records.length > 0 ? (
              <div className="space-y-2 p-1">
                {records.map((world) => (
                  <button
                    key={`${world.ownerId}/${world.id}`}
                    type="button"
                    className="w-full text-left p-3 rounded-lg border hover:bg-accent transition-colors flex gap-3"
                    onClick={() => handleSelect(world.ownerId, world.id)}
                  >
                    {/* Thumbnail */}
                    <div className="w-24 h-12 flex-shrink-0 rounded overflow-hidden bg-muted">
                      {world.thumbnailUrl ? (
                        <img
                          src={resolveUrl(world.thumbnailUrl)}
                          alt="world thumbnail"
                          className="w-full h-full object-cover"
                          loading="lazy"
                        />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center text-muted-foreground text-xs">
                          No Image
                        </div>
                      )}
                    </div>
                    {/* Info */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        {world.isFeatured && (
                          <Star className="h-4 w-4 text-yellow-500 fill-yellow-500 flex-shrink-0" />
                        )}
                        <RichText
                          text={world.name}
                          className="font-medium truncate"
                        />
                      </div>
                      <div className="text-sm text-muted-foreground">
                        By {world.ownerName}
                      </div>
                      {world.description && (
                        <RichText
                          text={world.description}
                          className="text-xs text-muted-foreground truncate block mt-1"
                        />
                      )}
                    </div>
                  </button>
                ))}
              </div>
            ) : (
              <div className="flex items-center justify-center h-32 text-muted-foreground">
                {activeTab === "search"
                  ? query
                    ? "検索結果がありません"
                    : "検索ワードを入力してください"
                  : "ワールドがありません"}
              </div>
            )}
          </ScrollBase>

          {/* Pagination */}
          <div className="flex justify-center gap-2 pt-2 border-t">
            <Button
              variant="outline"
              size="sm"
              onClick={handlePrevPage}
              disabled={currentPageIndex === 0 || isPending}
            >
              <ChevronLeft className="h-4 w-4" />
              前へ
            </Button>
            <span className="flex items-center px-3 text-sm text-muted-foreground">
              ページ {currentPageIndex + 1}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={handleNextPage}
              disabled={!hasMore || isPending}
            >
              次へ
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
