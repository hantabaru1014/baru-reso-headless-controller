import { Outlet, Navigate, useLocation } from "react-router";
import { DashboardLayout, ThemeSwitcher } from "@toolpad/core/DashboardLayout";
import { PageContainer } from "@toolpad/core/PageContainer";
import { Account } from "@toolpad/core/Account";
import { useAtom } from "jotai";
import { sessionAtom } from "../atoms/sessionAtom";
import { Stack } from "@mui/material";

function CustomAccount() {
  return (
    <Account
      slotProps={{
        preview: { slotProps: { avatarIconButton: { sx: { border: "0" } } } },
      }}
    />
  );
}

function ToolbarActions() {
  return (
    <Stack direction="row" gap="0.75rem" sx={{ alignItems: "center" }}>
      <ThemeSwitcher />
    </Stack>
  );
}

export default function Layout() {
  const [session] = useAtom(sessionAtom);
  const location = useLocation();

  if (!session) {
    const redirectTo = `/sign-in?callbackUrl=${encodeURIComponent(location.pathname)}`;
    return <Navigate to={redirectTo} replace />;
  }

  return (
    <DashboardLayout
      slots={{ toolbarAccount: CustomAccount, toolbarActions: ToolbarActions }}
    >
      <PageContainer>
        <Outlet />
      </PageContainer>
    </DashboardLayout>
  );
}
