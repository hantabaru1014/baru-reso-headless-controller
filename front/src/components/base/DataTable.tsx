import {
  ColumnDef,
  flexRender,
  getCoreRowModel,
  Header,
  useReactTable,
} from "@tanstack/react-table";
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

  // 「リサイズされた」カラムだけ explicit width を持つ。未リサイズの列は content auto-sizing。
  const columnSizing = table.getState().columnSizing;

  const startResize = (
    e: React.PointerEvent<HTMLDivElement>,
    header: Header<TData, unknown>,
  ) => {
    if (e.button !== 0 && e.pointerType === "mouse") return;
    const handle = e.currentTarget;
    e.preventDefault();
    e.stopPropagation();

    // handle は th の直接の子なので parentElement は常に th
    const startWidth = (
      handle.parentElement as HTMLElement
    ).getBoundingClientRect().width;
    const startX = e.clientX;
    const pointerId = e.pointerId;
    const minSize = header.column.columnDef.minSize ?? 30;
    const maxSize = header.column.columnDef.maxSize ?? Number.MAX_SAFE_INTEGER;

    // setPointerCapture により pointermove/up は cursor が iframe 外に出ても確実にこのハンドルに届く
    handle.setPointerCapture(pointerId);

    table.setColumnSizing((prev) => ({
      ...prev,
      [header.column.id]: startWidth,
    }));

    const onMove = (moveEvent: PointerEvent) => {
      const next = Math.min(
        maxSize,
        Math.max(minSize, startWidth + (moveEvent.clientX - startX)),
      );
      table.setColumnSizing((prev) => ({
        ...prev,
        [header.column.id]: next,
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
    };
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    handle.addEventListener("pointermove", onMove);
    handle.addEventListener("pointerup", onEnd);
    handle.addEventListener("pointercancel", onEnd);
  };

  const widthStyle = (id: string): React.CSSProperties | undefined =>
    columnSizing[id] !== undefined ? { width: columnSizing[id] } : undefined;

  // 最終 leaf カラムは右に隣接列がないのでリサイズハンドルを出さない (慣例 & 横スクロール抑止)
  const leafColumns = table.getVisibleLeafColumns();
  const lastLeafColumnId = leafColumns[leafColumns.length - 1]?.id;

  return (
    <div className="space-y-4">
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => {
                  return (
                    <TableHead
                      key={header.id}
                      className="relative"
                      style={widthStyle(header.column.id)}
                    >
                      {header.isPlaceholder
                        ? null
                        : flexRender(
                            header.column.columnDef.header,
                            header.getContext(),
                          )}
                      {enableColumnResizing &&
                        header.column.getCanResize() &&
                        header.column.id !== lastLeafColumnId && (
                          <div
                            onPointerDown={(e) => startResize(e, header)}
                            onClick={(e) => e.stopPropagation()}
                            className="group/resize absolute top-0 right-0 z-10 flex h-full w-3 translate-x-1/2 cursor-col-resize touch-none items-center justify-center select-none"
                            aria-hidden
                          >
                            <div
                              className={cn(
                                "bg-border h-full w-px transition-opacity group-hover/resize:opacity-100",
                                header.column.getIsResizing()
                                  ? "bg-primary w-0.5 opacity-100"
                                  : "opacity-0",
                              )}
                            />
                          </div>
                        )}
                    </TableHead>
                  );
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: loadingSkeletonCount }).map((_, index) => (
                <TableRow key={`skeleton-${index}`}>
                  {leafColumns.map((column, colIndex) => (
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
                    <TableCell key={cell.id} style={widthStyle(cell.column.id)}>
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
                  No results.
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
