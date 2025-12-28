import { useMemo } from "react";
import { parseRichText } from "@/libs/richTextUtils";

interface RichTextProps {
  text: string;
  className?: string;
  /**
   * レイアウトを崩す可能性のあるタグを無視するかどうか
   * 対象: br, size, align, line-height
   */
  ignoreLayoutTags?: boolean;
}

/**
 * Resoniteの装飾タグを含むテキストをレンダリングするコンポーネント
 *
 * 対応タグ:
 * - 書式: <b>, <i>, <u>, <s>, <sub>, <sup>
 * - 改行: <br>, <nobr>
 * - テキスト変換: <lowercase>, <uppercase>, <smallcaps>
 * - スタイル: <color>, <alpha>, <size>, <mark>, <align>, <line-height>, <gradient>
 */
export function RichText({
  text,
  className,
  ignoreLayoutTags = false,
}: RichTextProps) {
  const html = useMemo(
    () => parseRichText(text, { ignoreLayoutTags }),
    [text, ignoreLayoutTags],
  );

  return (
    <span className={className} dangerouslySetInnerHTML={{ __html: html }} />
  );
}
