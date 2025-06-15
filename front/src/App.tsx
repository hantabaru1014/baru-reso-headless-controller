import * as React from "react";
import { Outlet } from "react-router";
import { createConnectTransport } from "@connectrpc/connect-web";
import { TransportProvider } from "@connectrpc/connect-query";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAuth } from "./hooks/useAuth";
import { Toaster } from "./components/base";
import { ThemeProvider } from "./components/ThemeProvider";

const queryClient = new QueryClient();

export default function App() {
  const { configuredFetch } = useAuth("/");
  const finalTransport = React.useMemo(
    () =>
      createConnectTransport({
        baseUrl: "/",
        fetch: configuredFetch,
      }),
    [configuredFetch],
  );

  return (
    <ThemeProvider>
      <TransportProvider transport={finalTransport}>
        <QueryClientProvider client={queryClient}>
          <Outlet />
          <Toaster position="top-center" />
        </QueryClientProvider>
      </TransportProvider>
    </ThemeProvider>
  );
}
