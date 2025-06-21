import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";
import { EditableSelectField } from "./EditableSelectField";

const meta = {
  title: "base/EditableSelectField",
  component: EditableSelectField,
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
    options: [
      { id: "option1", label: "Option 1" },
      { id: "option2", label: "Option 2" },
      { id: "option3", label: "Option 3" },
    ],
    selectedId: "option1",
  },
} satisfies Meta<typeof EditableSelectField<string>>;
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
