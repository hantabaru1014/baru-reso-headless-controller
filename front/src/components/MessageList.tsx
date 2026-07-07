import { Message } from "../../pbgen/hdlctrl/v1/message_pb";
import { cn } from "@/libs/cssUtils";
import { formatTimestamp } from "../libs/datetimeUtils";

/**
 * お知らせ一覧. タイトル・更新日時・最終更新者を表示する選択可能なリスト.
 * 並び順はサーバーが返す updated_at 降順のまま.
 */
export function MessageList({
  messages,
  selectedId,
  onSelect,
}: {
  messages: Message[];
  selectedId: string | undefined;
  onSelect: (message: Message) => void;
}) {
  if (messages.length === 0) {
    return (
      <div className="text-muted-foreground p-4 text-center text-sm">
        お知らせはありません
      </div>
    );
  }

  return (
    <ul className="divide-y">
      {messages.map((message) => {
        const isSelected = message.id === selectedId;
        return (
          <li key={message.id}>
            <button
              type="button"
              onClick={() => onSelect(message)}
              className={cn(
                "hover:bg-muted/50 w-full px-3 py-2 text-left transition-colors",
                isSelected && "bg-muted",
              )}
            >
              <div className="truncate text-sm font-medium">
                {message.title}
              </div>
              <div className="text-muted-foreground mt-0.5 flex flex-wrap gap-x-2 text-xs">
                <span>{formatTimestamp(message.updatedAt)}</span>
                {message.lastUpdatedBy && (
                  <span
                    className="font-mono truncate"
                    title={message.lastUpdatedBy}
                  >
                    {message.lastUpdatedBy}
                  </span>
                )}
              </div>
            </button>
          </li>
        );
      })}
    </ul>
  );
}
