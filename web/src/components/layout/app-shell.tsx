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
// Layout constants — token阶梯 audited. The shell is the single source of
// truth for the topbar / sidebar geometry; keeping these as explicit grid
// pixel values (instead of synthesising one-off CSS vars) keeps the layout
// math readable while still matching the design tokens documented in
// docs/02-ui-design.md (TOPBAR_HEIGHT=56, SIDEBAR_WIDTH=240).
const SHELL_GRID_STYLE: React.CSSProperties = {
  display: "grid",
  gridTemplateAreas: `"topbar topbar" "sidebar main"`,
  gridTemplateRows: "56px 1fr",
  gridTemplateColumns: "240px 1fr",
};

const MAIN_STYLE: React.CSSProperties = { gridArea: "main" };

export function AppShell({ children }: AppShellProps) {
  return (
    <div
      className="h-screen overflow-hidden"
      style={SHELL_GRID_STYLE}
    >
      <Topbar />
      <Sidebar />
      <main
        className="overflow-auto bg-[var(--color-bg)] p-6"
        style={MAIN_STYLE}
      >
        {children}
      </main>
      <CmdK />
    </div>
  );
}
