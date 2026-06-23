import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";
import { ColumnDef } from "@tanstack/react-table";
import { useMemo, useState } from "react";

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

const SEX_VALUES: SampleData["sex"][] = ["male", "female", "other"];

function generateSampleData(count: number): SampleData[] {
  const names = [
    "Alice",
    "Bob",
    "Charlie",
    "Daniel",
    "Eve",
    "Frank",
    "Grace",
    "Hannah",
    "Ivan",
    "Judy",
    "Kevin",
    "Linda",
    "Mallory",
    "Nina",
    "Oscar",
    "Peggy",
  ];
  return Array.from({ length: count }, (_, i) => ({
    id: String(i + 1),
    name: `${names[i % names.length]} ${Math.floor(i / names.length) + 1}`,
    age: 18 + (i % 50),
    sex: SEX_VALUES[i % SEX_VALUES.length],
  }));
}

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

/**
 * 250 件のダミーデータをサーバーサイドページング相当の挙動でレンダーする。
 * ページ番号や表示件数の切り替えで `pagination` prop がどう動くか確認できる。
 */
export const WithPagination: Story = {
  args: {
    onClickRow: fn(),
  },
  render: function Render(args) {
    const allData = useMemo(() => generateSampleData(250), []);
    const [pageIndex, setPageIndex] = useState(0);
    const [pageSize, setPageSize] = useState(20);

    const slicedData = useMemo(
      () => allData.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize),
      [allData, pageIndex, pageSize],
    );

    return (
      <DataTable
        {...args}
        columns={columns}
        data={slicedData}
        pagination={{
          pageIndex,
          pageSize,
          totalCount: allData.length,
          onPageIndexChange: setPageIndex,
          onPageSizeChange: (n) => {
            setPageSize(n);
            setPageIndex(0);
          },
        }}
      />
    );
  },
};

/** データが少なくページ数が 1 のケース。前へ/次へが両方 disabled になる挙動を確認。 */
export const WithPaginationSinglePage: Story = {
  render: function Render(args) {
    const data = useMemo(() => generateSampleData(8), []);
    const [pageIndex, setPageIndex] = useState(0);
    const [pageSize, setPageSize] = useState(20);

    return (
      <DataTable
        {...args}
        columns={columns}
        data={data}
        pagination={{
          pageIndex,
          pageSize,
          totalCount: data.length,
          onPageIndexChange: setPageIndex,
          onPageSizeChange: setPageSize,
        }}
      />
    );
  },
};

/** 0 件 (totalCount=0) のときに pagination が「0 件」のみを表示する挙動を確認。 */
export const WithPaginationEmpty: Story = {
  render: function Render(args) {
    return (
      <DataTable
        {...args}
        columns={columns}
        data={[]}
        pagination={{
          pageIndex: 0,
          pageSize: 20,
          totalCount: 0,
          onPageIndexChange: fn(),
          onPageSizeChange: fn(),
        }}
      />
    );
  },
};
