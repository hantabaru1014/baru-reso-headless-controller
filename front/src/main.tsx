import * as React from "react";
import * as ReactDOM from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router";
import "./index.css";
import "./libs/i18n";
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
            handle: { titleKey: "routes.dashboard" },
          },
          {
            path: "sessions",
            children: [
              {
                index: true,
                Component: Sessions,
                handle: { titleKey: "routes.sessions" },
              },
              {
                path: "new",
                Component: SessionNew,
                handle: { titleKey: "routes.sessionNew" },
              },
              {
                path: "scheduled",
                children: [
                  {
                    index: true,
                    Component: ScheduledOperationsIndex,
                    handle: { titleKey: "routes.scheduledOps" },
                  },
                  {
                    path: "new",
                    Component: ScheduledOperationNew,
                    handle: { titleKey: "routes.scheduledOpNew" },
                  },
                ],
              },
              {
                path: ":id",
                Component: SessionDetail,
                handle: { titleKey: "routes.sessionDetail" },
              },
            ],
          },
          {
            path: "hosts",
            children: [
              {
                index: true,
                Component: Hosts,
                handle: { titleKey: "routes.hosts" },
              },
              {
                path: ":id",
                Component: HostDetail,
                handle: { titleKey: "routes.hostDetail" },
              },
            ],
          },
          {
            path: "headlessAccounts",
            children: [
              {
                index: true,
                Component: HeadlessAccounts,
                handle: { titleKey: "routes.headlessAccounts" },
              },
            ],
          },
          {
            path: "groups",
            children: [
              {
                index: true,
                Component: Groups,
                handle: { titleKey: "routes.groups" },
              },
              {
                path: ":id",
                Component: GroupDetail,
                handle: { titleKey: "routes.groupDetail" },
              },
            ],
          },
          {
            path: "admin",
            children: [
              {
                index: true,
                Component: AdminIndex,
                handle: { titleKey: "routes.admin" },
              },
              {
                path: "groups",
                Component: AdminGroupsPage,
                handle: { titleKey: "routes.adminGroups" },
              },
              {
                path: "roles",
                Component: AdminRolesPage,
                handle: { titleKey: "routes.adminRoles" },
              },
              {
                path: "users",
                Component: AdminUsersPage,
                handle: { titleKey: "routes.adminUsers" },
              },
            ],
          },
          {
            path: "user-settings",
            Component: UserSettings,
            handle: { titleKey: "routes.userSettings" },
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
