import {
  ColumnDef,
  flexRender,
  getCoreRowModel,
  Header,
  Table as TableInstance,
  useReactTable,
} from "@tanstack/react-table";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../ui";
import { DataTablePaginationBar } from "./DataTablePaginationBar";
import { cn } from "@/libs/cssUtils";

/**
 * カラム幅リサイズ (隣接列トレード方式) のロジック。
 *
 * - 初回リサイズまでは columnSizing は空で、全列 content による auto-sizing。
 *   一度リサイズすると全列の実測幅をスナップショットして table-layout: fixed に切り替える。
 *   auto レイアウトのまま1列だけ width を指定すると、ブラウザが残り幅を他列へ再配分して
 *   ドラッグがカーソルに追従しないため。
 * - ドラッグ中の幅のトレードは境界の両隣 (対象列と右隣) だけで行い、それより右の列は
 *   位置も幅も動かさない。列幅の合計が常に一定なので横スクロールも発生しない。
 * - どちらかの列が minSize / maxSize に達したところでドラッグは停止する。
 * - columnSizing state はテーブル幅に対する % で保持する。px 固定だとブラウザ幅を
 *   狭めたときに合計がコンテナを超えて横スクロールや最終列の消失が起きるため、
 *   % にして全列を比例スケールさせる。
 */
function useColumnResizing<TData>(table: TableInstance<TData>) {
  const [resizingColumnId, setResizingColumnId] = useState<string | null>(null);

  const columnSizing = table.getState().columnSizing;
  const isSizingActive = Object.keys(columnSizing).length > 0;

  const leafColumns = table.getVisibleLeafColumns();
  const lastLeafColumnId = leafColumns[leafColumns.length - 1]?.id;

  // 最終列には explicit width を与えず、fixed レイアウトの余り (丸め誤差など) の吸収役にする
  const widthStyle = (id: string): React.CSSProperties | undefined =>
    columnSizing[id] !== undefined && id !== lastLeafColumnId
      ? { width: `${columnSizing[id]}%` }
      : undefined;

  // 最終 leaf カラムは右に隣接列がないのでリサイズハンドルを出さない
  const hasResizeHandle = (header: Header<TData, unknown>) =>
    header.column.getCanResize() && header.column.id !== lastLeafColumnId;

  const startResize = (
    e: React.PointerEvent<HTMLDivElement>,
    header: Header<TData, unknown>,
  ) => {
    if (e.pointerType === "mouse" && e.button !== 0) return;

    const colIndex = leafColumns.findIndex((c) => c.id === header.column.id);
    const neighbor = leafColumns[colIndex + 1];
    // handle は th の直接の子なので parentElement は常に th
    const th = e.currentTarget.parentElement as HTMLElement;
    const headerCells = Array.from(
      (th.parentElement as HTMLElement).children,
    ) as HTMLElement[];
    const neighborTh = headerCells[colIndex + 1];
    if (!neighbor || !neighborTh) return; // 最終列にはハンドルが無いので通常到達しない

    e.preventDefault();
    e.stopPropagation();

    const handle = e.currentTarget;
    const pointerId = e.pointerId;
    const startX = e.clientX;
    const startWidth = th.getBoundingClientRect().width;
    const neighborStartWidth = neighborTh.getBoundingClientRect().width;
    // ドラッグ中の計算は px で行い、state へは % に変換して保存する
    const tableWidth = (
      th.closest("table") as HTMLElement
    ).getBoundingClientRect().width;
    const toPercent = (px: number) => (px / tableWidth) * 100;

    // minSize: 20 等のデフォルトは TanStack Table が columnDef へマージ済み
    const { minSize = 20, maxSize = Number.MAX_SAFE_INTEGER } =
      header.column.columnDef;
    const {
      minSize: neighborMinSize = 20,
      maxSize: neighborMaxSize = Number.MAX_SAFE_INTEGER,
    } = neighbor.columnDef;

    // 両列の min/max を同時に満たす範囲 (トレード相手の制約は自列の幅に読み替える)
    const lowerBound = Math.max(
      minSize,
      startWidth - (neighborMaxSize - neighborStartWidth),
    );
    const upperBound = Math.min(
      maxSize,
      startWidth + (neighborStartWidth - neighborMinSize),
    );

    // setPointerCapture により pointermove/up は cursor がハンドル外に出ても確実に届く
    handle.setPointerCapture(pointerId);
    setResizingColumnId(header.column.id);

    // 最終列以外の全列の実測幅をスナップショット → auto から fixed レイアウトへの
    // 切り替わりで見た目の列幅が変わらないようにする
    const snapshot: Record<string, number> = {};
    leafColumns.forEach((column, i) => {
      if (headerCells[i] && column.id !== lastLeafColumnId) {
        snapshot[column.id] = toPercent(
          headerCells[i].getBoundingClientRect().width,
        );
      }
    });
    table.setColumnSizing((prev) => ({ ...prev, ...snapshot }));

    const onMove = (moveEvent: PointerEvent) => {
      const next = Math.min(
        upperBound,
        Math.max(lowerBound, startWidth + (moveEvent.clientX - startX)),
      );
      table.setColumnSizing((prev) => ({
        ...prev,
        [header.column.id]: toPercent(next),
        // 増減分は右隣だけが吸収する
        [neighbor.id]: toPercent(neighborStartWidth - (next - startWidth)),
      }));
    };
    const onEnd = () => {
      handle.removeEventListener("pointermove", onMove);
      handle.removeEventListener("pointerup", onEnd);
      handle.removeEventListener("pointercancel", onEnd);
      if (handle.hasPointerCapture(pointerId)) {
        handle.releasePointerCapture(pointerId);
      }
      document.body.style.removeProperty("cursor");
      document.body.style.removeProperty("user-select");
      setResizingColumnId(null);
    };
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    handle.addEventListener("pointermove", onMove);
    handle.addEventListener("pointerup", onEnd);
    handle.addEventListener("pointercancel", onEnd);
  };

  return {
    isSizingActive,
    resizingColumnId,
    widthStyle,
    hasResizeHandle,
    startResize,
  };
}

export type DataTablePaginationProps = {
  /** 0-based current page index */
  pageIndex: number;
  pageSize: number;
  totalCount: number;
  pageSizeOptions?: number[];
  onPageIndexChange: (n: number) => void;
  onPageSizeChange: (n: number) => void;
};

interface DataTableProps<TData, TValue> {
  columns: ColumnDef<TData, TValue>[];
  data: TData[];
  isLoading?: boolean;
  loadingSkeletonCount?: number;
  onClickRow?: (row: TData) => void;
  pagination?: DataTablePaginationProps;
  /** ヘッダー右端をドラッグしてカラム幅を変更可能にする。デフォルト true。 */
  enableColumnResizing?: boolean;
}

export function DataTable<TData, TValue>({
  columns,
  data,
  isLoading,
  loadingSkeletonCount = 5,
  onClickRow,
  pagination,
  enableColumnResizing = true,
}: DataTableProps<TData, TValue>) {
  const { t } = useTranslation();
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    enableColumnResizing,
    ...(pagination
      ? {
          manualPagination: true,
          pageCount:
            pagination.totalCount > 0
              ? Math.ceil(pagination.totalCount / pagination.pageSize)
              : 0,
          state: {
            pagination: {
              pageIndex: pagination.pageIndex,
              pageSize: pagination.pageSize,
            },
          },
        }
      : {}),
  });

  const {
    isSizingActive,
    resizingColumnId,
    widthStyle,
    hasResizeHandle,
    startResize,
  } = useColumnResizing(table);

  return (
    <div className="space-y-4">
      <div className="rounded-md border">
        {/* fixed レイアウト時もテーブル幅は w-full のまま (コンテナ幅ちょうど)。
            幅は隣接列間のトレードなので合計は常に一定で、横スクロールは発生しない */}
        <Table className={isSizingActive ? "table-fixed" : undefined}>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    className="relative"
                    style={widthStyle(header.column.id)}
                  >
                    {header.isPlaceholder ? null : (
                      // th 自体に overflow-hidden を付けるとリサイズハンドルが
                      // クリップされるため、中身側で溢れを抑える
                      <div className="overflow-hidden text-ellipsis">
                        {flexRender(
                          header.column.columnDef.header,
                          header.getContext(),
                        )}
                      </div>
                    )}
                    {hasResizeHandle(header) && (
                      <div
                        onPointerDown={(e) => startResize(e, header)}
                        onClick={(e) => e.stopPropagation()}
                        className="group/resize absolute top-0 right-0 z-10 flex h-full w-3 translate-x-1/2 cursor-col-resize touch-none items-center justify-center select-none"
                        aria-hidden
                      >
                        <div
                          className={cn(
                            "bg-border h-full w-px transition-opacity group-hover/resize:opacity-100",
                            resizingColumnId === header.column.id
                              ? "bg-primary w-0.5 opacity-100"
                              : "opacity-0",
                          )}
                        />
                      </div>
                    )}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: loadingSkeletonCount }).map((_, index) => (
                <TableRow key={`skeleton-${index}`}>
                  {table.getVisibleLeafColumns().map((column, colIndex) => (
                    <TableCell
                      key={`skeleton-cell-${index}-${colIndex}`}
                      style={widthStyle(column.id)}
                    >
                      <Skeleton className="h-4 rounded" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && "selected"}
                  onClick={() => onClickRow?.(row.original)}
                  className={onClickRow ? "cursor-pointer" : ""}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell
                      key={cell.id}
                      // fixed レイアウトでコンテンツ幅より狭めた際に隣の列へはみ出さないようにする
                      className={
                        isSizingActive
                          ? "overflow-hidden text-ellipsis"
                          : undefined
                      }
                      style={widthStyle(cell.column.id)}
                    >
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext(),
                      )}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className="h-24 text-center"
                >
                  {t("dataTable.noResults")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      {pagination && (
        <DataTablePaginationBar
          pageIndex={pagination.pageIndex}
          pageSize={pagination.pageSize}
          totalCount={pagination.totalCount}
          pageSizeOptions={pagination.pageSizeOptions}
          onPageIndexChange={pagination.onPageIndexChange}
          onPageSizeChange={pagination.onPageSizeChange}
        />
      )}
    </div>
  );
}
