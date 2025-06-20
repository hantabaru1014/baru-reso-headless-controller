import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { UserMenuDropdown } from "./UserMenuDropdown";

const meta = {
  title: "base/UserMenuDropdown",
  component: UserMenuDropdown,
  tags: ["autodocs"],
  args: {
    user: {
      name: "User Name",
      email: "user@example.com",
    },
    signOut: fn(),
  },
} satisfies Meta<typeof UserMenuDropdown>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
};
