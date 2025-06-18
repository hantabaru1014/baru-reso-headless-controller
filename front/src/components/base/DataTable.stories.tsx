import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";
import { ColumnDef } from "@tanstack/react-table";

import { DataTable } from "./DataTable";

type SampleData = {
  id: string;
  name: string;
  age: number;
  sex: "male" | "female" | "other";
};

function sexToLabel(sex: "male" | "female" | "other"): string {
  switch (sex) {
    case "male":
      return "男性";
    case "female":
      return "女性";
    case "other":
      return "その他";
  }
}

const columns: ColumnDef<SampleData>[] = [
  {
    accessorKey: "id",
    header: "ID",
  },
  {
    accessorKey: "name",
    header: "Name",
  },
  {
    accessorKey: "age",
    header: "Age",
  },
  {
    accessorKey: "sex",
    header: "Sex",
    cell: ({ cell }) =>
      sexToLabel(cell.getValue<"male" | "female" | "other">()),
  },
];

const meta = {
  title: "base/DataTable",
  component: DataTable,
  tags: ["autodocs"],
  argTypes: {
    isLoading: {
      control: "boolean",
    },
    loadingSkeletonCount: {
      control: "number",
    },
  },
  args: {
    // @ts-expect-error なんかエラー出るけどとりあえず動くから無視
    columns,
    data: [
      { id: "1", name: "Alice", age: 30, sex: "female" },
      { id: "2", name: "Bob", age: 25, sex: "male" },
      { id: "3", name: "Charlie", age: 35, sex: "other" },
    ],
  },
} satisfies Meta<typeof DataTable>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
};

export const RowClickable: Story = {
  args: {
    onClickRow: fn(),
  },
};
