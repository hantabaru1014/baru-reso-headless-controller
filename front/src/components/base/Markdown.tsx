import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { cn } from "@/libs/cssUtils";

/**
 * markdown 文字列を安全にレンダリングする.
 *
 * react-markdown は既定で生 HTML をエスケープするため, 本文に埋め込まれた
 * HTML は実行されない (rehype-raw 等は意図的に有効化しない). 画像は標準の
 * `![](url)` 記法でのみ表示され, `max-w-full` で幅を制約する. 外部リンクは
 * 新規タブで開く.
 */
export function Markdown({
  children,
  className,
}: {
  children: string;
  className?: string;
}) {
  return (
    <div
      className={cn("prose prose-sm dark:prose-invert max-w-none", className)}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ ...props }) => (
            <a {...props} target="_blank" rel="noopener noreferrer" />
          ),
          img: ({ ...props }) => (
            <img {...props} className="max-w-full h-auto" />
          ),
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
