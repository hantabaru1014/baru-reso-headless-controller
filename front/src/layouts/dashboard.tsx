import { Outlet, Navigate, useLocation, Link } from "react-router";
import { Home, Users, Server, Earth, LogOut } from "lucide-react";
import { useAtom } from "jotai";
import { sessionAtom, Session } from "../atoms/sessionAtom";
import { useAuth } from "../hooks/useAuth";
import {
  Button,
  Avatar,
  AvatarFallback,
  AvatarImage,
  SidebarProvider,
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarTrigger,
} from "@/components/ui";
import { ThemeToggle } from "@/components/ThemeToggle";
import { cn } from "@/libs/cssUtils";

const navigation = [
  {
    title: "Dashboard",
    href: "/",
    icon: Home,
  },
  {
    title: "Headless Accounts",
    href: "/headlessAccounts",
    icon: Users,
  },
  {
    title: "Hosts",
    href: "/hosts",
    icon: Server,
  },
  {
    title: "Sessions",
    href: "/sessions",
    icon: Earth,
  },
];

function AppSidebar() {
  const location = useLocation();

  return (
    <Sidebar variant="inset" collapsible="icon">
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {navigation.map((item) => (
                <SidebarMenuItem key={item.href}>
                  <SidebarMenuButton
                    asChild
                    isActive={
                      location.pathname === item.href ||
                      (item.href !== "/" &&
                        location.pathname.startsWith(item.href))
                    }
                  >
                    <Link to={item.href}>
                      <item.icon />
                      <span>{item.title}</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
    </Sidebar>
  );
}

function Header({
  session,
  signOut,
}: {
  session?: Session;
  signOut: () => void;
}) {
  return (
    <header className="bg-background border-b px-4 flex items-center justify-between gap-3 lg:px-6">
      <SidebarTrigger />

      {/* Logo/Branding */}
      <div className="p-3">
        <h1 className="text-xl font-bold">BRHDL</h1>
      </div>

      {/* Spacer */}
      <div className="flex-1" />

      <ThemeToggle />
      {/* User Account */}
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <Avatar className="h-8 w-8">
            <AvatarImage src={session?.user?.image} alt={session?.user?.name} />
            <AvatarFallback>
              {session?.user?.name?.charAt(0) || "U"}
            </AvatarFallback>
          </Avatar>
          <div className="hidden md:block text-sm">
            <div className="font-medium">{session?.user?.name}</div>
            <div className="text-muted-foreground">{session?.user?.email}</div>
          </div>
        </div>
        <Button
          variant="ghost"
          size="icon"
          onClick={signOut}
          title="サインアウト"
        >
          <LogOut className="h-4 w-4" />
        </Button>
      </div>
    </header>
  );
}

export default function Layout() {
  const [session] = useAtom(sessionAtom);
  const location = useLocation();
  const { signOut } = useAuth("/");

  if (!session) {
    const redirectTo = `/sign-in?callbackUrl=${encodeURIComponent(location.pathname)}`;
    return <Navigate to={redirectTo} replace />;
  }

  return (
    <SidebarProvider>
      <AppSidebar />
      <div
        data-slot="sidebar-inset"
        className={cn(
          "bg-background relative flex w-full flex-1 flex-col",
          "md:peer-data-[variant=inset]:m-2 md:peer-data-[variant=inset]:ml-0 md:peer-data-[variant=inset]:rounded-xl md:peer-data-[variant=inset]:shadow-sm md:peer-data-[variant=inset]:peer-data-[state=collapsed]:ml-2",
        )}
      >
        <Header session={session} signOut={signOut} />
        <main className="p-6">
          <Outlet />
        </main>
      </div>
    </SidebarProvider>
  );
}
