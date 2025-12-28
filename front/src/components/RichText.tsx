import { useMemo } from "react";
import { parseRichText } from "@/libs/richTextUtils";

interface RichTextProps {
  text: string;
  className?: string;
}

/**
 * Resoniteの装飾タグを含むテキストをレンダリングするコンポーネント
 * 対応タグ: <color=...>text</color>
 */
export function RichText({ text, className }: RichTextProps) {
  const html = useMemo(() => parseRichText(text), [text]);

  return (
    <span className={className} dangerouslySetInnerHTML={{ __html: html }} />
  );
}
