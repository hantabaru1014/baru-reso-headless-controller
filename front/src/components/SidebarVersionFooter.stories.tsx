import type { Meta, StoryObj } from "@storybook/react-vite";
import { Clock, Earth, Home, Server, Users } from "lucide-react";
import { Link, MemoryRouter } from "react-router";

import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui";
import { SidebarVersionFooter } from "./SidebarVersionFooter";

const navigation = [
  { title: "Dashboard", href: "/", icon: Home },
  { title: "Headless Accounts", href: "/headlessAccounts", icon: Users },
  { title: "Hosts", href: "/hosts", icon: Server },
  { title: "Sessions", href: "/sessions", icon: Earth, active: true },
  { title: "Scheduled Ops", href: "/sessions/scheduled", icon: Clock },
];

const meta = {
  title: "components/SidebarVersionFooter",
  component: SidebarVersionFooter,
  tags: ["autodocs"],
  decorators: [
    (Story) => (
      <MemoryRouter>
        <SidebarProvider>
          <Sidebar variant="inset" collapsible="icon">
            <SidebarContent>
              <SidebarGroup>
                <SidebarGroupContent>
                  <SidebarMenu>
                    {navigation.map((item) => (
                      <SidebarMenuItem key={item.href}>
                        <SidebarMenuButton asChild isActive={item.active}>
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
            <Story />
          </Sidebar>
          <div className="flex-1 p-4">
            <SidebarTrigger />
          </div>
        </SidebarProvider>
      </MemoryRouter>
    ),
  ],
} satisfies Meta<typeof SidebarVersionFooter>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Tag: Story = {
  args: { version: "v0.1.0" },
};

export const CommitHash: Story = {
  args: { version: "f3fbfa6" },
};

export const Dev: Story = {
  args: { version: "dev" },
};
