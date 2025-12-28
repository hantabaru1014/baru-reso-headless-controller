import type { Meta, StoryObj } from "@storybook/react-vite";
import { RichText } from "./RichText";

const meta = {
  title: "base/RichText",
  component: RichText,
  tags: ["autodocs"],
  parameters: {
    layout: "padded",
  },
  argTypes: {
    text: {
      control: "text",
      description: "Resoniteリッチテキスト形式のテキスト",
    },
    ignoreLayoutTags: {
      control: "boolean",
      description:
        "レイアウトを崩すタグ(br, size, align, line-height)を無視する",
    },
    className: {
      control: "text",
      description: "追加のCSSクラス",
    },
  },
  args: {
    ignoreLayoutTags: false,
  },
} satisfies Meta<typeof RichText>;
export default meta;
type Story = StoryObj<typeof meta>;

/** 基本的な書式タグ: 太字、斜体、下線、取り消し線 */
export const BasicFormatting: Story = {
  args: {
    text: "<b>太字</b> <i>斜体</i> <u>下線</u> <s>取り消し線</s>",
  },
};

/** 色のバリエーション: Hex、名前付きカラー、パレットカラー */
export const ColorVariants: Story = {
  args: {
    text: "<color=#ff0000>Hex赤</color> <color=red>名前付き赤</color> <color=hero.yellow>パレット黄</color> <color=#ff000080>半透明赤</color>",
  },
};

/** すべての名前付きカラー */
export const AllNamedColors: Story = {
  args: {
    text: "<color=clear>clear</color> <color=white>white</color> <color=gray>gray</color> <color=black>black</color> <color=red>red</color> <color=green>green</color> <color=blue>blue</color> <color=yellow>yellow</color> <color=cyan>cyan</color> <color=magenta>magenta</color> <color=orange>orange</color> <color=purple>purple</color> <color=lime>lime</color> <color=pink>pink</color> <color=brown>brown</color>",
  },
  decorators: [
    (Story) => (
      <div className="bg-neutral-500 p-4">
        <Story />
      </div>
    ),
  ],
};

/** パレットカラー: hero, mid, sub, dark, neutrals */
export const PaletteColors: Story = {
  args: {
    text: "<color=hero.yellow>hero.yellow</color> <color=mid.green>mid.green</color> <color=sub.red>sub.red</color> <color=dark.purple>dark.purple</color> <color=neutrals.light>neutrals.light</color>",
  },
  decorators: [
    (Story) => (
      <div className="bg-neutral-800 p-4">
        <Story />
      </div>
    ),
  ],
};

/** サイズ指定: パーセント、em、数値 */
export const SizeVariants: Story = {
  args: {
    text: "通常 <size=150%>150%</size> <size=50%>50%</size> <size=2em>2em</size> <size=20>数値20</size>",
  },
};

/** テキスト変換: 小文字、大文字、スモールキャップス */
export const TextTransform: Story = {
  args: {
    text: "<lowercase>LOWERCASE</lowercase> <uppercase>uppercase</uppercase> <smallcaps>SmallCaps</smallcaps>",
  },
};

/** 上付き/下付き文字 */
export const ScriptMode: Story = {
  args: {
    text: "H<sub>2</sub>O, E=mc<sup>2</sup>, x<sub>1</sub><sup>2</sup>",
  },
};

/** マークとアルファ */
export const MarkAndAlpha: Story = {
  args: {
    text: "<mark=#ffff00>ハイライト</mark> <mark=#00ff00>緑ハイライト</mark> <alpha=#80>半透明テキスト</alpha> <alpha=#40>より透明</alpha>",
  },
};

/** グラデーション */
export const Gradient: Story = {
  args: {
    text: "<gradient=#ff0000,#00ff00,#0000ff>レインボーグラデーション</gradient> <gradient=#ff6b6b,#feca57>暖色系</gradient>",
  },
};

/** ネストしたタグ */
export const NestedTags: Story = {
  args: {
    text: "<color=#ff0000><b><i>赤太字斜体</i></b></color> <color=hero.cyan><u><b>シアン下線太字</b></u></color>",
  },
};

/** 改行制御: br と nobr */
export const LineBreakAndNobr: Story = {
  args: {
    text: "1行目<br>2行目<br>3行目 | <nobr>この部分は 改行されない 長いテキスト</nobr>",
  },
  decorators: [
    (Story) => (
      <div className="w-64 border p-2">
        <Story />
      </div>
    ),
  ],
};

/** アラインメント */
export const Alignment: Story = {
  args: {
    text: "<align=left>左寄せ</align><align=center>中央寄せ</align><align=right>右寄せ</align>",
  },
  decorators: [
    (Story) => (
      <div className="w-64 border p-2">
        <Story />
      </div>
    ),
  ],
};

/** 行の高さ */
export const LineHeight: Story = {
  args: {
    text: "<line-height=200%>行の高さ200%<br>2行目<br>3行目</line-height>",
  },
};

/** レイアウトタグ無視オプション: オン */
export const IgnoreLayoutTagsEnabled: Story = {
  args: {
    text: "<size=200%>大きいテキスト</size><br>改行<br><align=center>中央</align>",
    ignoreLayoutTags: true,
  },
  parameters: {
    docs: {
      description: {
        story:
          "ignoreLayoutTags=true の場合、size, br, align, line-height タグは無視されます",
      },
    },
  },
};

/** レイアウトタグ無視オプション: オフ (比較用) */
export const IgnoreLayoutTagsDisabled: Story = {
  args: {
    text: "<size=200%>大きいテキスト</size><br>改行<br><align=center>中央</align>",
    ignoreLayoutTags: false,
  },
  decorators: [
    (Story) => (
      <div className="w-64 border p-2">
        <Story />
      </div>
    ),
  ],
};

/** 未サポートタグ: 中身のテキストのみ表示 */
export const UnsupportedTags: Story = {
  args: {
    text: "<font=Arial>フォント指定は無視</font> <sprite=icon>スプライト</sprite> <glyph=star>グリフ</glyph>",
  },
  parameters: {
    docs: {
      description: {
        story:
          "font, sprite, glyph タグはWebでは表現できないため、タグは除去され中身のテキストのみ表示されます",
      },
    },
  },
};

/** noparse タグ */
export const NoParseTag: Story = {
  args: {
    text: "通常<noparse><b>これは太字にならない</b></noparse>通常",
  },
  parameters: {
    docs: {
      description: {
        story:
          "noparse タグ内のテキストはそのまま表示されます（タグ自体は除去）",
      },
    },
  },
};

/** 閉じタグがない場合の動作(Resoniteではなくても表示できる) */
export const MissingCloseTags: Story = {
  args: {
    text: "<b>太字<i>斜体 <u>下線 <s>取り消し線",
  },
};

/** closeall タグ */
export const CloseAllTag: Story = {
  args: {
    text: "<color=#ff0000><b><i>複数タグ</closeall>閉じられた後",
  },
  parameters: {
    docs: {
      description: {
        story: "closeall タグはスタック内のすべてのタグを一度に閉じます",
      },
    },
  },
};

/** 実際の使用例: ゲーム内メッセージ風 */
export const RealWorldExample: Story = {
  args: {
    text: "<color=hero.yellow><b>ようこそ！</b></color><br>プレイヤー名: <color=#00ff00>TestUser</color><br>ステータス: <color=red>オフライン</color>",
  },
  decorators: [
    (Story) => (
      <div className="bg-neutral-900 p-4 rounded">
        <Story />
      </div>
    ),
  ],
};

/** 複雑なネスト */
export const ComplexNesting: Story = {
  args: {
    text: "<color=hero.cyan><b><size=120%><u>見出し</u></size></b></color><br><i><color=#aaaaaa>説明文テキスト</color></i>",
  },
  decorators: [
    (Story) => (
      <div className="bg-neutral-800 p-4 rounded">
        <Story />
      </div>
    ),
  ],
};

/** Hexカラー形式のバリエーション */
export const HexColorFormats: Story = {
  args: {
    text: "<color=#f00>#f00 (3桁)</color> <color=#ff00>#ff00 (4桁RGBA)</color> <color=#ff0000>#ff0000 (6桁)</color> <color=#ff000080>#ff000080 (8桁RGBA)</color>",
  },
  decorators: [
    (Story) => (
      <div className="bg-white p-4">
        <Story />
      </div>
    ),
  ],
};

/** エスケープ処理: HTMLエンティティ */
export const HtmlEscaping: Story = {
  args: {
    text: "HTML特殊文字: <b>&lt;script&gt;</b> &amp; <i>&quot;引用符&quot;</i>",
  },
  parameters: {
    docs: {
      description: {
        story:
          '通常のテキスト内のHTML特殊文字（<, >, &, " 等）は適切にエスケープされます',
      },
    },
  },
};

/** 無効なタグ: そのままテキストとして表示 */
export const InvalidTags: Story = {
  args: {
    text: "無効なタグ: <invalid>これは通常テキスト</invalid> <123>数字開始</123>",
  },
  parameters: {
    docs: {
      description: {
        story: "認識されないタグはそのままテキストとして表示されます",
      },
    },
  },
};
