import { useMemo } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui";
import { DEFAULT_PAGE_SIZE_OPTIONS } from "../../hooks/usePaginationState";
import { cn } from "@/libs/cssUtils";

export type DataTablePaginationBarProps = {
  /** 0-based current page index */
  pageIndex: number;
  pageSize: number;
  totalCount: number;
  pageSizeOptions?: number[];
  onPageIndexChange: (n: number) => void;
  onPageSizeChange: (n: number) => void;
};

type PageItem = number | "ellipsis-left" | "ellipsis-right";

/**
 * Build a compact page item list, aiming to keep around 7 entries.
 * Example: [1, "ellipsis-left", 4, 5, 6, "ellipsis-right", 20]
 */
function buildPageItems(currentPage: number, totalPages: number): PageItem[] {
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, i) => i + 1);
  }

  const items: PageItem[] = [];
  const showLeftEllipsis = currentPage > 4;
  const showRightEllipsis = currentPage < totalPages - 3;

  items.push(1);

  if (showLeftEllipsis) {
    items.push("ellipsis-left");
  }

  // The "window" around the current page.
  let windowStart: number;
  let windowEnd: number;
  if (!showLeftEllipsis) {
    windowStart = 2;
    windowEnd = 5;
  } else if (!showRightEllipsis) {
    windowStart = totalPages - 4;
    windowEnd = totalPages - 1;
  } else {
    windowStart = currentPage - 1;
    windowEnd = currentPage + 1;
  }

  for (let p = windowStart; p <= windowEnd; p++) {
    if (p > 1 && p < totalPages) {
      items.push(p);
    }
  }

  if (showRightEllipsis) {
    items.push("ellipsis-right");
  }

  items.push(totalPages);
  return items;
}

export function DataTablePaginationBar({
  pageIndex,
  pageSize,
  totalCount,
  pageSizeOptions = DEFAULT_PAGE_SIZE_OPTIONS,
  onPageIndexChange,
  onPageSizeChange,
}: DataTablePaginationBarProps) {
  const totalPages = Math.max(1, Math.ceil(totalCount / pageSize));
  const currentPage = pageIndex + 1; // 1-based for display
  const pageItems = useMemo(
    () => buildPageItems(currentPage, totalPages),
    [currentPage, totalPages],
  );

  if (totalCount === 0) {
    return (
      <div className="flex justify-end text-sm text-muted-foreground">0 件</div>
    );
  }

  const rangeStart = pageIndex * pageSize + 1;
  const rangeEnd = Math.min(totalCount, (pageIndex + 1) * pageSize);

  const isPrevDisabled = pageIndex <= 0;
  const isNextDisabled = pageIndex >= totalPages - 1;

  const handleClickPage = (e: React.MouseEvent, page: number) => {
    e.preventDefault();
    if (page < 1 || page > totalPages || page === currentPage) return;
    onPageIndexChange(page - 1);
  };

  const handlePrev = (e: React.MouseEvent) => {
    e.preventDefault();
    if (isPrevDisabled) return;
    onPageIndexChange(pageIndex - 1);
  };

  const handleNext = (e: React.MouseEvent) => {
    e.preventDefault();
    if (isNextDisabled) return;
    onPageIndexChange(pageIndex + 1);
  };

  return (
    <div className="flex flex-wrap items-center justify-end gap-4">
      <div className="text-sm text-muted-foreground">
        全 {totalCount} 件中 {rangeStart}-{rangeEnd} 件目
      </div>

      <Pagination className="mx-0 w-auto justify-end">
        <PaginationContent>
          <PaginationItem>
            <PaginationLink
              href="#"
              size="default"
              onClick={handlePrev}
              aria-label="前へ"
              aria-disabled={isPrevDisabled}
              tabIndex={isPrevDisabled ? -1 : undefined}
              className={cn(
                "gap-1 pl-2.5",
                isPrevDisabled
                  ? "pointer-events-none opacity-50"
                  : "cursor-pointer",
              )}
            >
              <ChevronLeft className="h-4 w-4" />
              <span>前へ</span>
            </PaginationLink>
          </PaginationItem>
          {pageItems.map((item) =>
            item === "ellipsis-left" || item === "ellipsis-right" ? (
              <PaginationItem key={item}>
                <PaginationEllipsis />
              </PaginationItem>
            ) : (
              <PaginationItem key={item}>
                <PaginationLink
                  href="#"
                  isActive={item === currentPage}
                  onClick={(e) => handleClickPage(e, item)}
                  className="cursor-pointer"
                >
                  {item}
                </PaginationLink>
              </PaginationItem>
            ),
          )}
          <PaginationItem>
            <PaginationLink
              href="#"
              size="default"
              onClick={handleNext}
              aria-label="次へ"
              aria-disabled={isNextDisabled}
              tabIndex={isNextDisabled ? -1 : undefined}
              className={cn(
                "gap-1 pr-2.5",
                isNextDisabled
                  ? "pointer-events-none opacity-50"
                  : "cursor-pointer",
              )}
            >
              <span>次へ</span>
              <ChevronRight className="h-4 w-4" />
            </PaginationLink>
          </PaginationItem>
        </PaginationContent>
      </Pagination>

      <div className="flex items-center gap-2">
        <span className="text-sm text-muted-foreground">表示件数:</span>
        <Select
          value={String(pageSize)}
          onValueChange={(value) => onPageSizeChange(Number(value))}
        >
          <SelectTrigger size="sm" className="w-[80px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {pageSizeOptions.map((size) => (
              <SelectItem key={size} value={String(size)}>
                {size}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  );
}
