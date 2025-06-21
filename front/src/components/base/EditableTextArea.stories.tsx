import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";
import { EditableTextArea } from "./EditableTextArea";
import { useState } from "react";

const meta = {
  title: "base/EditableTextArea",
  component: EditableTextArea,
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
    value: "Editable Text\n\nthird line",
  },
} satisfies Meta<typeof EditableTextArea>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
};

export const Interactive: Story = {
  render: (args) => {
    const [value, setValue] = useState(args.value as string);

    return (
      <EditableTextArea
        {...args}
        value={value}
        onSave={(newValue) => {
          setValue(newValue);
          return Promise.resolve({ ok: true });
        }}
      />
    );
  },
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
