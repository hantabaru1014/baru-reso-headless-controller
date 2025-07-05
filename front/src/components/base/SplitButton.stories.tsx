import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";
import { SplitButton } from "./SplitButton";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";

const meta = {
  title: "base/SplitButton",
  component: SplitButton,
  tags: ["autodocs"],
  argTypes: {
    variant: {
      control: "select",
      options: ["default", "destructive", "outline", "secondary", "ghost", "link"],
    },
    size: {
      control: "select",
      options: ["default", "sm", "lg", "icon"],
    },
    disabled: {
      control: "boolean",
    },
    mainDisabled: {
      control: "boolean",
    },
    dropdownDisabled: {
      control: "boolean",
    },
  },
  args: {
    children: "メインアクション",
    onMainClick: fn(),
    dropdownContent: (
      <>
        <DropdownMenuItem onClick={fn()}>オプション1</DropdownMenuItem>
        <DropdownMenuItem onClick={fn()}>オプション2</DropdownMenuItem>
        <DropdownMenuItem onClick={fn()}>オプション3</DropdownMenuItem>
      </>
    ),
  },
} satisfies Meta<typeof SplitButton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
};

export const Outline: Story = {
  args: {
    variant: "outline",
  },
};

export const Destructive: Story = {
  args: {
    variant: "destructive",
    children: "削除",
    dropdownContent: (
      <>
        <DropdownMenuItem onClick={fn()}>選択したアイテムを削除</DropdownMenuItem>
        <DropdownMenuItem onClick={fn()}>すべて削除</DropdownMenuItem>
      </>
    ),
  },
};

export const Small: Story = {
  args: {
    size: "sm",
  },
};

export const Large: Story = {
  args: {
    size: "lg",
  },
};

export const Disabled: Story = {
  args: {
    disabled: true,
  },
};

export const MainDisabled: Story = {
  args: {
    mainDisabled: true,
  },
};

export const DropdownDisabled: Story = {
  args: {
    dropdownDisabled: true,
  },
};

export const WithLongText: Story = {
  args: {
    children: "長いテキストのボタン",
    dropdownContent: (
      <>
        <DropdownMenuItem onClick={fn()}>長いテキストのオプション1</DropdownMenuItem>
        <DropdownMenuItem onClick={fn()}>長いテキストのオプション2</DropdownMenuItem>
        <DropdownMenuItem onClick={fn()}>長いテキストのオプション3</DropdownMenuItem>
      </>
    ),
  },
};