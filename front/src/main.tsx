import * as React from "react";
import * as ReactDOM from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router";
import App from "./App";
import Layout from "./layouts/dashboard";
import DashboardPage from "./pages";
import SignInPage from "./pages/signin";
import Sessions from "./pages/sessions";
import SessionDetail from "./pages/sessions/detail";
import SessionNew from "./pages/sessions/new";
import Hosts from "./pages/hosts";

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
          },
          {
            path: "sessions",
            children: [
              {
                index: true,
                Component: Sessions,
              },
              {
                path: "new",
                Component: SessionNew,
              },
              {
                path: ":id",
                Component: SessionDetail,
              },
            ],
          },
          {
            path: "hosts",
            children: [
              {
                index: true,
                Component: Hosts,
              },
            ],
          },
        ],
      },
      {
        path: "/sign-in",
        Component: SignInPage,
      },
    ],
  },
]);

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>,
);
