import * as React from "react";
import DashboardIcon from "@mui/icons-material/Dashboard";
import ShoppingCartIcon from "@mui/icons-material/ShoppingCart";
import { Outlet } from "react-router";
import { ReactRouterAppProvider } from "@toolpad/core/react-router";
import type { Navigation } from "@toolpad/core/AppProvider";
import SessionContext, { type Session } from "./SessionContext";

const NAVIGATION: Navigation = [
  {
    kind: "header",
    title: "Main items",
  },
  {
    title: "Dashboard",
    icon: <DashboardIcon />,
  },
  {
    segment: "orders",
    title: "Orders",
    icon: <ShoppingCartIcon />,
  },
];

const BRANDING = {
  title: "BRHDL",
};

export default function App() {
  const [session, setSession] = React.useState<Session | null>(null);

  return (
    <ReactRouterAppProvider
      navigation={NAVIGATION}
      branding={BRANDING}
      session={session}
      authentication={{
        signIn: () => {},
        signOut: () => {
          setSession(null);
        },
      }}
    >
      <SessionContext.Provider
        value={{
          session,
          setSession,
        }}
      >
        <Outlet />
      </SessionContext.Provider>
    </ReactRouterAppProvider>
  );
}
