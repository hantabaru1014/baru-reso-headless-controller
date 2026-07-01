import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router";

export type UsePaginationStateOptions = {
  /** Default page size when the URL has no (or an invalid) pageSize. Default: 20 */
  defaultPageSize?: number;
  /** Allowed page sizes. Default: [20, 50, 100] */
  pageSizeOptions?: number[];
  /**
   * Prefix for the search params. Allows multiple paginations to coexist
   * on the same URL (e.g. `sessionsPage`, `sessionsPageSize`).
   * Default: '' (no prefix → `page`, `pageSize`)
   */
  paramPrefix?: string;
};

export type UsePaginationStateReturn = {
  /** 0-based page index */
  pageIndex: number;
  pageSize: number;
  pageSizeOptions: number[];
  setPageIndex: (n: number) => void;
  /** Setting page size always resets pageIndex to 0. */
  setPageSize: (n: number) => void;
};

export const DEFAULT_PAGE_SIZE = 20;
export const DEFAULT_PAGE_SIZE_OPTIONS = [20, 50, 100];

function buildParamNames(prefix: string) {
  if (!prefix) {
    return { pageParam: "page", pageSizeParam: "pageSize" };
  }
  return {
    pageParam: `${prefix}Page`,
    pageSizeParam: `${prefix}PageSize`,
  };
}

function parseIntegerParam(value: string | null): number | null {
  if (value === null) return null;
  const trimmed = value.trim();
  if (trimmed === "") return null;
  const num = Number(trimmed);
  if (!Number.isFinite(num) || !Number.isInteger(num)) return null;
  return num;
}

export function usePaginationState(
  options: UsePaginationStateOptions = {},
): UsePaginationStateReturn {
  const {
    defaultPageSize = DEFAULT_PAGE_SIZE,
    pageSizeOptions = DEFAULT_PAGE_SIZE_OPTIONS,
    paramPrefix = "",
  } = options;

  const [searchParams, setSearchParams] = useSearchParams();

  const { pageParam, pageSizeParam } = useMemo(
    () => buildParamNames(paramPrefix),
    [paramPrefix],
  );

  // URL stores `page` as 1-based for user friendliness; internal is 0-based.
  const parsedUrlPage = parseIntegerParam(searchParams.get(pageParam));
  const pageIndex =
    parsedUrlPage !== null && parsedUrlPage >= 1 ? parsedUrlPage - 1 : 0;

  const parsedUrlPageSize = parseIntegerParam(searchParams.get(pageSizeParam));
  const pageSize =
    parsedUrlPageSize !== null && pageSizeOptions.includes(parsedUrlPageSize)
      ? parsedUrlPageSize
      : defaultPageSize;

  const setPageIndex = useCallback(
    (n: number) => {
      const safe = Math.max(0, Math.floor(n));
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (safe === 0) {
            next.delete(pageParam);
          } else {
            next.set(pageParam, String(safe + 1));
          }
          return next;
        },
        { replace: false },
      );
    },
    [setSearchParams, pageParam],
  );

  const setPageSize = useCallback(
    (n: number) => {
      const safe = pageSizeOptions.includes(n) ? n : defaultPageSize;
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (safe === defaultPageSize) {
            next.delete(pageSizeParam);
          } else {
            next.set(pageSizeParam, String(safe));
          }
          // Reset page index when page size changes.
          next.delete(pageParam);
          return next;
        },
        { replace: false },
      );
    },
    [
      setSearchParams,
      pageSizeParam,
      pageParam,
      pageSizeOptions,
      defaultPageSize,
    ],
  );

  return {
    pageIndex,
    pageSize,
    pageSizeOptions,
    setPageIndex,
    setPageSize,
  };
}
