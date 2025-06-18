import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";

import { EditableTextField } from "./EditableTextField";

const meta = {
  title: "base/EditableTextField",
  component: EditableTextField,
  tags: ["autodocs"],
  argTypes: {
    readonly: {
      control: "boolean",
    },
    isLoading: {
      control: "boolean",
    },
    disabled: {
      control: "boolean",
    },
    helperText: {
      control: "text",
    },
  },
  args: {
    label: "label",
    onSave: fn(() => Promise.resolve({ ok: true })),
    value: "Editable Text",
  },
} satisfies Meta<typeof EditableTextField>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
};

export const WithError: Story = {
  args: {
    onSave: fn(() => Promise.resolve({ ok: false, error: "Error saving" })),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button"));
    await userEvent.click(canvas.getByTestId("editable-field-save-button"));
  },
};
