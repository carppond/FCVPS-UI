import * as React from "react";
import { Topbar } from "@/components/layout/topbar";
import { Sidebar } from "@/components/layout/sidebar";
import { CmdK } from "@/components/layout/cmd-k";

interface AppShellProps {
  children: React.ReactNode;
}

/**
 * Root layout: CSS Grid with topbar (56px) + sidebar (240px) + main content area.
 *
 *  ┌──────────────────────────────────┐
 *  │           topbar  56px           │
 *  ├──────────┬───────────────────────┤
 *  │ sidebar  │     main content      │
 *  │  240px   │                       │
 *  └──────────┴───────────────────────┘
 */
export function AppShell({ children }: AppShellProps) {
  return (
    <div
      className="h-screen overflow-hidden"
      style={{
        display: "grid",
        gridTemplateAreas: `"topbar topbar" "sidebar main"`,
        gridTemplateRows: "56px 1fr",
        gridTemplateColumns: "240px 1fr",
      }}
    >
      <Topbar />
      <Sidebar />
      <main
        className="overflow-auto bg-[var(--color-bg)] p-6"
        style={{ gridArea: "main" }}
      >
        {children}
      </main>
      <CmdK />
    </div>
  );
}
