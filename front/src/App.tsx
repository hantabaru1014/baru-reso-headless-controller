import * as React from "react";
import { Outlet } from "react-router";
import { createConnectTransport } from "@connectrpc/connect-web";
import { TransportProvider } from "@connectrpc/connect-query";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAuth } from "./hooks/useAuth";
import { useResoniteUserSync } from "./hooks/useResoniteUserSync";
import { Toaster } from "./components/ui";
import { ThemeProvider } from "./components/ThemeProvider";

const queryClient = new QueryClient();

function AppContent() {
  useResoniteUserSync();
  return (
    <>
      <Outlet />
      <Toaster position="top-center" />
    </>
  );
}

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
          <AppContent />
        </QueryClientProvider>
      </TransportProvider>
    </ThemeProvider>
  );
}
