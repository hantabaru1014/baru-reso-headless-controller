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

const NAVIGATION: Navigation = [
  {
    title: "Dashboard",
    icon: <DashboardIcon />,
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
  const [session, setSession] = useAtom(sessionAtom);
  const finalTransport = React.useMemo(
    () =>
      createConnectTransport({
        baseUrl: "/",
        interceptors: [
          (next) => async (request) => {
            if (session) {
              request.header.append("authorization", `Bearer ${session.token}`);
            }
            return next(request);
          },
        ],
      }),
    [session],
  );

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
      <TransportProvider transport={finalTransport}>
        <QueryClientProvider client={queryClient}>
          <Outlet />
        </QueryClientProvider>
      </TransportProvider>
    </ReactRouterAppProvider>
  );
}
