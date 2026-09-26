import { Code, ConnectError } from "@connectrpc/connect";

export const isNotFoundError = (e: unknown) =>
  e instanceof ConnectError && e.code === Code.NotFound;

/**
 * リトライしても結果が変わらないエラーはリトライしない.
 * それ以外は TanStack Query のデフォルト (3 回) に合わせる.
 */
export const shouldRetryQuery = (failureCount: number, e: unknown) => {
  if (
    e instanceof ConnectError &&
    (e.code === Code.NotFound || e.code === Code.PermissionDenied)
  ) {
    return false;
  }
  return failureCount < 3;
};
