// Resoniteのカラーパレット
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

// 名前付きカラー
const NAMED_COLORS: Record<string, string> = {
  clear: "transparent",
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
};

// レイアウトに影響するタグ
const LAYOUT_TAGS = new Set(["br", "size", "align", "line-height"]);

// 完全に除去するタグ（中身は表示）
const UNSUPPORTED_TAGS = new Set([
  "font",
  "sprite",
  "glyph",
  "noparse",
  "closeallblock",
]);

export interface ParseRichTextOptions {
  /**
   * レイアウトを崩す可能性のあるタグを無視するかどうか
   * 対象: br, size, align, line-height
   */
  ignoreLayoutTags?: boolean;
}

/**
 * Hexカラーコードを正規化する
 * #rgb → #rrggbb, #rgba → #rrggbbaa
 */
function normalizeHexColor(hex: string): string | null {
  const value = hex.replace(/^#/, "");
  if (!/^[\da-f]+$/i.test(value)) return null;

  switch (value.length) {
    case 3: // #rgb → #rrggbb
      return `#${value[0]}${value[0]}${value[1]}${value[1]}${value[2]}${value[2]}`;
    case 4: // #rgba → #rrggbbaa
      return `#${value[0]}${value[0]}${value[1]}${value[1]}${value[2]}${value[2]}${value[3]}${value[3]}`;
    case 6: // #rrggbb
    case 8: // #rrggbbaa
      return `#${value}`;
    default:
      return null;
  }
}

/**
 * Resoniteのカラー値を解決する
 */
function resolveColor(colorValue: string): string | null {
  const value = colorValue.trim();

  // Hex color: #rgb, #rgba, #rrggbb, #rrggbbaa
  if (value.startsWith("#")) {
    return normalizeHexColor(value);
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
 * アルファ値（#XX形式）をopacity値（0-1）に変換
 */
function parseAlpha(value: string): number | null {
  const trimmed = value.trim();
  if (!trimmed.startsWith("#")) return null;
  const hex = trimmed.slice(1);
  if (hex.length !== 2 || !/^[\da-f]{2}$/i.test(hex)) return null;
  return parseInt(hex, 16) / 255;
}

/**
 * サイズ値をCSS値に変換
 * 数値 → 0.1em単位として解釈
 * 数値% → そのままpercent
 * 数値em → そのままem
 */
function parseSize(value: string): string | null {
  const trimmed = value.trim();
  const match = trimmed.match(/^([\d.]+)(em|%)?$/i);
  if (!match) return null;

  const num = parseFloat(match[1]);
  if (isNaN(num)) return null;

  const unit = match[2]?.toLowerCase();
  if (unit === "%") {
    return `${num}%`;
  } else if (unit === "em") {
    return `${num}em`;
  } else {
    // 数値のみの場合は0.1em単位（Resonite互換）
    return `${num * 0.1}em`;
  }
}

/**
 * line-height値（XX%形式）をCSS値に変換
 */
function parseLineHeight(value: string): string | null {
  const trimmed = value.trim();
  if (!trimmed.endsWith("%")) return null;
  const num = parseInt(trimmed.slice(0, -1), 10);
  if (isNaN(num)) return null;
  return `${num}%`;
}

/**
 * gradientのカラーリストをCSS gradientに変換
 */
function parseGradient(value: string): string | null {
  const colors = value.split(",").map((c) => normalizeHexColor(c.trim()));
  if (colors.some((c) => c === null) || colors.length < 2) return null;
  return `linear-gradient(90deg, ${colors.join(", ")})`;
}

interface TagInfo {
  tag: string;
  parameter: string;
  isClosing: boolean;
  endIndex: number;
}

/**
 * タグをパースする（Resoniteの状態マシンを模倣）
 */
function parseTag(str: string, startIndex: number): TagInfo | null {
  if (str[startIndex] !== "<") return null;
  if (startIndex + 2 >= str.length) return null;

  let i = startIndex + 1;
  let isClosing = false;

  // 閉じタグかチェック
  if (str[i] === "/") {
    isClosing = true;
    i++;
  }

  // タグ名の開始（アルファベットで始まる必要がある）
  if (!/[a-zA-Z]/.test(str[i])) return null;

  // タグ名をスキャン
  const tagStart = i;
  while (i < str.length && /[a-zA-Z-]/.test(str[i])) {
    i++;
  }
  const tag = str.slice(tagStart, i).toLowerCase();

  // タグ名が空の場合は無効
  if (tag.length === 0) return null;

  // 閉じタグの場合はパラメータなし
  if (isClosing) {
    // '>'を探す
    while (i < str.length && str[i] !== ">") {
      i++;
    }
    if (str[i] !== ">") return null;
    return { tag, parameter: "", isClosing: true, endIndex: i + 1 };
  }

  // パラメータをスキャン
  let parameter = "";
  if (str[i] === "=" || str[i] === " ") {
    i++; // '=' or ' ' をスキップ
    const paramStart = i;
    while (i < str.length && str[i] !== ">") {
      i++;
    }
    parameter = str.slice(paramStart, i).trim();
  }

  if (str[i] !== ">") return null;
  return { tag, parameter, isClosing: false, endIndex: i + 1 };
}

/**
 * 開始タグをHTML開始タグに変換
 */
function convertOpenTag(
  tag: string,
  parameter: string,
  options: ParseRichTextOptions,
): string {
  const ignoreLayout = options.ignoreLayoutTags ?? false;

  // レイアウトタグで無視オプションが有効な場合
  if (ignoreLayout && LAYOUT_TAGS.has(tag)) {
    return "";
  }

  // 未サポートタグは除去
  if (UNSUPPORTED_TAGS.has(tag)) {
    return "";
  }

  // パラメータなしタグ
  switch (tag) {
    case "b":
      return "<strong>";
    case "i":
      return "<em>";
    case "u":
      return "<u>";
    case "s":
      return "<s>";
    case "br":
      return "<br>";
    case "nobr":
      return '<span style="white-space:nowrap">';
    case "sub":
      return "<sub>";
    case "sup":
      return "<sup>";
    case "lowercase":
      return '<span style="text-transform:lowercase">';
    case "uppercase":
      return '<span style="text-transform:uppercase">';
    case "smallcaps":
    case "allcaps":
      return '<span style="font-variant:small-caps">';
  }

  // パラメータ付きタグ
  switch (tag) {
    case "color": {
      const color = resolveColor(parameter);
      if (color) {
        return `<span style="color:${color}">`;
      }
      return "";
    }
    case "alpha": {
      const opacity = parseAlpha(parameter);
      if (opacity !== null) {
        return `<span style="opacity:${opacity.toFixed(3)}">`;
      }
      return "";
    }
    case "size": {
      const size = parseSize(parameter);
      if (size) {
        return `<span style="font-size:${size}">`;
      }
      return "";
    }
    case "mark": {
      const color = normalizeHexColor(parameter);
      if (color) {
        return `<mark style="background-color:${color}">`;
      }
      return "";
    }
    case "align": {
      const align = parameter.toLowerCase();
      if (["left", "right", "center", "justify"].includes(align)) {
        return `<span style="display:block;text-align:${align}">`;
      }
      return "";
    }
    case "line-height": {
      const height = parseLineHeight(parameter);
      if (height) {
        return `<span style="line-height:${height}">`;
      }
      return "";
    }
    case "gradient": {
      const gradient = parseGradient(parameter);
      if (gradient) {
        return `<span style="background:${gradient};-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text">`;
      }
      return "";
    }
  }

  return "";
}

/**
 * 閉じタグをHTML閉じタグに変換
 */
function convertCloseTag(tag: string, options: ParseRichTextOptions): string {
  const ignoreLayout = options.ignoreLayoutTags ?? false;

  // レイアウトタグで無視オプションが有効な場合
  if (ignoreLayout && LAYOUT_TAGS.has(tag)) {
    return "";
  }

  // 未サポートタグは除去
  if (UNSUPPORTED_TAGS.has(tag)) {
    return "";
  }

  // brは自己終結タグなので閉じタグなし
  if (tag === "br") {
    return "";
  }

  // HTML閉じタグに変換
  switch (tag) {
    case "b":
      return "</strong>";
    case "i":
      return "</em>";
    case "u":
      return "</u>";
    case "s":
      return "</s>";
    case "sub":
      return "</sub>";
    case "sup":
      return "</sup>";
    case "mark":
      return "</mark>";
    // span系タグ
    case "nobr":
    case "lowercase":
    case "uppercase":
    case "smallcaps":
    case "allcaps":
    case "color":
    case "alpha":
    case "size":
    case "align":
    case "line-height":
    case "gradient":
      return "</span>";
  }

  return "";
}

/**
 * テキストをHTMLエスケープする
 */
function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

/**
 * Resoniteの装飾タグを含むテキストをHTML形式に変換する
 */
export function parseRichText(
  text: string,
  options: ParseRichTextOptions = {},
): string {
  if (!text) return "";

  let result = "";
  let i = 0;
  const tagStack: string[] = [];

  while (i < text.length) {
    if (text[i] === "<") {
      const tagInfo = parseTag(text, i);
      if (tagInfo) {
        if (tagInfo.isClosing) {
          // closeall は特殊処理
          if (tagInfo.tag === "closeall") {
            // スタック内の全タグを閉じる
            while (tagStack.length > 0) {
              const openTag = tagStack.pop()!;
              result += convertCloseTag(openTag, options);
            }
          } else {
            // 通常の閉じタグ
            const closeHtml = convertCloseTag(tagInfo.tag, options);
            if (closeHtml) {
              result += closeHtml;
              // スタックから対応するタグを削除
              const idx = tagStack.lastIndexOf(tagInfo.tag);
              if (idx !== -1) {
                tagStack.splice(idx, 1);
              }
            }
          }
        } else {
          // 開きタグ
          const openHtml = convertOpenTag(
            tagInfo.tag,
            tagInfo.parameter,
            options,
          );
          if (openHtml) {
            result += openHtml;
            // brは自己終結タグなのでスタックに追加しない
            if (tagInfo.tag !== "br") {
              tagStack.push(tagInfo.tag);
            }
          }
        }
        i = tagInfo.endIndex;
        continue;
      }
    }

    // 通常のテキスト（HTMLエスケープ）
    result += escapeHtml(text[i]);
    i++;
  }

  // 閉じられていないタグを自動で閉じる
  while (tagStack.length > 0) {
    const openTag = tagStack.pop()!;
    result += convertCloseTag(openTag, options);
  }

  return result;
}

// サポートされている全タグのパターン
const ALL_TAGS_PATTERN =
  /<\/?(?:b|i|u|s|br|nobr|sub|sup|lowercase|uppercase|smallcaps|allcaps|color|alpha|size|mark|align|line-height|gradient|font|sprite|glyph|noparse|closeallblock|closeall)(?:[^>]*)>/gi;

/**
 * テキストにRichTextタグが含まれているかどうかを判定する
 */
export function hasRichTextTags(text: string): boolean {
  if (!text) return false;
  ALL_TAGS_PATTERN.lastIndex = 0; // グローバルフラグによるlastIndexをリセット
  return ALL_TAGS_PATTERN.test(text);
}

/**
 * Resoniteのテキストからタグを除去してプレーンテキストを取得する
 */
export function stripRichTextTags(text: string): string {
  if (!text) return "";
  return text.replace(ALL_TAGS_PATTERN, "");
}
