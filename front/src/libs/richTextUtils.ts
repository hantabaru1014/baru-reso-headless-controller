// Resoniteのカラーパレット (based on go.resonite.com)
const COLOR_PALETTE: Record<string, Record<string, string>> = {
  hero: {
    yellow: "#ffd966",
    green: "#6aa84f",
    red: "#cc0000",
    purple: "#9900ff",
    cyan: "#00ffff",
    orange: "#ff9900",
  },
  mid: {
    yellow: "#f1c232",
    green: "#38761d",
    red: "#990000",
    purple: "#7600bf",
    cyan: "#00bfbf",
    orange: "#e69138",
  },
  sub: {
    yellow: "#bf9000",
    green: "#274e13",
    red: "#660000",
    purple: "#4c0080",
    cyan: "#008080",
    orange: "#b45f06",
  },
  dark: {
    yellow: "#7f6000",
    green: "#1e3a0f",
    red: "#4c0000",
    purple: "#2d004d",
    cyan: "#004d4d",
    orange: "#783f04",
  },
  neutrals: {
    dark: "#333333",
    mid: "#666666",
    light: "#999999",
  },
};

// 標準のカラー名
const NAMED_COLORS: Record<string, string> = {
  white: "#ffffff",
  gray: "#808080",
  black: "#000000",
  red: "#ff0000",
  green: "#00ff00",
  blue: "#0000ff",
  yellow: "#ffff00",
  cyan: "#00ffff",
  magenta: "#ff00ff",
  orange: "#ffa500",
  purple: "#800080",
  lime: "#00ff00",
  pink: "#ffc0cb",
  brown: "#a52a2a",
  clear: "transparent",
};

/**
 * Resoniteのカラー値を解決する
 */
function resolveColor(colorValue: string): string | null {
  // Remove surrounding quotes if present
  const value = colorValue.replace(/^["']|["']$/g, "").trim();

  // Hex color: #rgb, #rgba, #rrggbb, #rrggbbaa
  if (/^#[\da-f]{3,8}$/i.test(value)) {
    return value.toLowerCase();
  }

  // Palette color: hero.yellow, neutrals.dark, etc.
  const paletteMatch = value.match(/^(\w+)\.(\w+)$/);
  if (paletteMatch) {
    const [, paletteName, colorName] = paletteMatch;
    const palette = COLOR_PALETTE[paletteName.toLowerCase()];
    if (palette) {
      const color = palette[colorName.toLowerCase()];
      if (color) return color;
    }
  }

  // Named color: white, red, etc.
  const namedColor = NAMED_COLORS[value.toLowerCase()];
  if (namedColor) return namedColor;

  return null;
}

/**
 * Resoniteの装飾タグを含むテキストをHTML形式に変換する
 * 対応タグ: <color=...>text</color>
 */
export function parseRichText(text: string): string {
  if (!text) return "";

  // <color=value> または <color value> を処理
  const colorRegex =
    /<color[=\s]\s*(?<colorValue>(?:"[^"]*"|'[^']*'|[^>]+))\s*>/gi;

  let result = text.replace(colorRegex, (_match, colorValue: string) => {
    const resolvedColor = resolveColor(colorValue);
    if (resolvedColor) {
      return `<span style="color: ${resolvedColor}">`;
    }
    return "";
  });

  // </color> を </span> に変換
  result = result.replace(/<\/color>/gi, "</span>");

  return result;
}

/**
 * Resoniteのテキストからタグを除去してプレーンテキストを取得する
 */
export function stripRichTextTags(text: string): string {
  if (!text) return "";
  return text.replace(/<\/?color[^>]*>/gi, "");
}
