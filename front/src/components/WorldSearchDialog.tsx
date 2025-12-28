import { useState, useEffect, useCallback } from "react";
import { useMutation } from "@connectrpc/connect-query";
import { searchWorlds } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
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

interface WorldSearchDialogProps {
  open: boolean;
  onClose: () => void;
  onSelect: (worldUrl: string) => void;
}

export function WorldSearchDialog({
  open,
  onClose,
  onSelect,
}: WorldSearchDialogProps) {
  const [query, setQuery] = useState("");
  const [featuredOnly, setFeaturedOnly] = useState(false);
  const [pageIndex, setPageIndex] = useState(0);

  const debouncedQuery = useDebounce(query, 300);

  const {
    data: searchResult,
    mutateAsync: mutateSearch,
    isPending,
  } = useMutation(searchWorlds);

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

  // Trigger search when debounced query or filters change
  useEffect(() => {
    if (open) {
      doSearch(debouncedQuery, featuredOnly, pageIndex);
    }
  }, [open, debouncedQuery, featuredOnly, pageIndex, doSearch]);

  // Reset state when dialog opens
  useEffect(() => {
    if (open) {
      setQuery("");
      setFeaturedOnly(false);
      setPageIndex(0);
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
    if (pageIndex > 0) {
      setPageIndex(pageIndex - 1);
    }
  };

  const handleNextPage = () => {
    if (searchResult?.hasMore) {
      setPageIndex(pageIndex + 1);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="sm:max-w-[1000px] max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>ワールド検索</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 flex-1 flex flex-col min-h-0">
          {/* Search input and featured checkbox */}
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

          {/* Results list */}
          <ScrollBase height="60vh">
            {isPending ? (
              <div className="flex items-center justify-center h-32 text-muted-foreground">
                検索中...
              </div>
            ) : searchResult?.records && searchResult.records.length > 0 ? (
              <div className="space-y-2 p-1">
                {searchResult.records.map((world) => (
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
                {query
                  ? "検索結果がありません"
                  : "検索ワードを入力してください"}
              </div>
            )}
          </ScrollBase>

          {/* Pagination */}
          <div className="flex justify-center gap-2 pt-2 border-t">
            <Button
              variant="outline"
              size="sm"
              onClick={handlePrevPage}
              disabled={pageIndex === 0 || isPending}
            >
              <ChevronLeft className="h-4 w-4" />
              前へ
            </Button>
            <span className="flex items-center px-3 text-sm text-muted-foreground">
              ページ {pageIndex + 1}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={handleNextPage}
              disabled={!searchResult?.hasMore || isPending}
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
