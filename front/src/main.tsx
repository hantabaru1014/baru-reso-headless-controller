import * as React from "react";
import * as ReactDOM from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router";
import "./index.css";
import App from "./App";
import Layout from "./layouts/dashboard";
import DashboardPage from "./pages";
import SignInPage from "./pages/signin";
import RegisterPage from "./pages/register";
import Sessions from "./pages/sessions";
import SessionDetail from "./pages/sessions/detail";
import SessionNew from "./pages/sessions/new";
import ScheduledOperationsIndex from "./pages/sessions/scheduled";
import ScheduledOperationNew from "./pages/sessions/scheduled/new";
import Hosts from "./pages/hosts";
import HostDetail from "./pages/hosts/detail";
import HeadlessAccounts from "./pages/headlessAccounts";
import UserSettings from "./pages/userSettings";
import Groups from "./pages/groups";
import GroupDetail from "./pages/groups/detail";
import AdminIndex from "./pages/admin";
import AdminGroupsPage from "./pages/admin/groups";
import AdminRolesPage from "./pages/admin/roles";
import AdminUsersPage from "./pages/admin/users";

const router = createBrowserRouter([
  {
    Component: App,
    children: [
      {
        path: "/",
        Component: Layout,
        children: [
          {
            index: true,
            Component: DashboardPage,
            handle: { title: "ダッシュボード" },
          },
          {
            path: "sessions",
            children: [
              {
                index: true,
                Component: Sessions,
                handle: { title: "セッション" },
              },
              {
                path: "new",
                Component: SessionNew,
                handle: { title: "新規セッション" },
              },
              {
                path: "scheduled",
                children: [
                  {
                    index: true,
                    Component: ScheduledOperationsIndex,
                    handle: { title: "予約操作" },
                  },
                  {
                    path: "new",
                    Component: ScheduledOperationNew,
                    handle: { title: "新規予約" },
                  },
                ],
              },
              {
                path: ":id",
                Component: SessionDetail,
                handle: { title: "セッション詳細" },
              },
            ],
          },
          {
            path: "hosts",
            children: [
              {
                index: true,
                Component: Hosts,
                handle: { title: "ホスト" },
              },
              {
                path: ":id",
                Component: HostDetail,
                handle: { title: "ホスト詳細" },
              },
            ],
          },
          {
            path: "headlessAccounts",
            children: [
              {
                index: true,
                Component: HeadlessAccounts,
                handle: { title: "ヘッドレスアカウント" },
              },
            ],
          },
          {
            path: "groups",
            children: [
              {
                index: true,
                Component: Groups,
                handle: { title: "グループ" },
              },
              {
                path: ":id",
                Component: GroupDetail,
                handle: { title: "グループ詳細" },
              },
            ],
          },
          {
            path: "admin",
            children: [
              {
                index: true,
                Component: AdminIndex,
                handle: { title: "システム管理" },
              },
              {
                path: "groups",
                Component: AdminGroupsPage,
                handle: { title: "全グループ" },
              },
              {
                path: "roles",
                Component: AdminRolesPage,
                handle: { title: "グローバルロール" },
              },
              {
                path: "users",
                Component: AdminUsersPage,
                handle: { title: "ユーザー管理" },
              },
            ],
          },
          {
            path: "user-settings",
            Component: UserSettings,
            handle: { title: "ユーザー設定" },
          },
        ],
      },
      {
        path: "/sign-in",
        Component: SignInPage,
      },
      {
        path: "/register/:token",
        Component: RegisterPage,
      },
    ],
  },
]);

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>,
);
