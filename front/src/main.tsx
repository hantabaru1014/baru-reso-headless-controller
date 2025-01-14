import * as React from "react";
import * as ReactDOM from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router";
import App from "./App";
import Layout from "./layouts/dashboard";
import DashboardPage from "./pages";
import SignInPage from "./pages/signin";
import Sessions from "./pages/sessions";
import SessionDetail from "./pages/sessions/detail";

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
                path: ":id",
                Component: SessionDetail,
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
