import * as React from "react";
import DashboardIcon from "@mui/icons-material/Dashboard";
import { Outlet } from "react-router";
import { ReactRouterAppProvider } from "@toolpad/core/react-router";
import type { Navigation } from "@toolpad/core/AppProvider";
import { createConnectTransport } from "@connectrpc/connect-web";
import { TransportProvider } from "@connectrpc/connect-query";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAtom } from "jotai";
import { sessionAtom } from "./atoms/sessionAtom";
import { useAuth } from "./hooks/useAuth";
import { DialogsProvider } from "@toolpad/core/useDialogs";

const NAVIGATION: Navigation = [
  {
    title: "Dashboard",
    icon: <DashboardIcon />,
  },
  {
    segment: "hosts",
    title: "Hosts",
  },
  {
    segment: "sessions",
    title: "Sessions",
  },
];

const BRANDING = {
  title: "BRHDL",
};

const queryClient = new QueryClient();

export default function App() {
  const [session] = useAtom(sessionAtom);

  const { configuredFetch, signOut } = useAuth("/");
  const finalTransport = React.useMemo(
    () =>
      createConnectTransport({
        baseUrl: "/",
        fetch: configuredFetch,
      }),
    [configuredFetch],
  );

  return (
    <ReactRouterAppProvider
      navigation={NAVIGATION}
      branding={BRANDING}
      session={session}
      authentication={{
        signIn: () => {},
        signOut,
      }}
    >
      <TransportProvider transport={finalTransport}>
        <QueryClientProvider client={queryClient}>
          <DialogsProvider>
            <Outlet />
          </DialogsProvider>
        </QueryClientProvider>
      </TransportProvider>
    </ReactRouterAppProvider>
  );
}
