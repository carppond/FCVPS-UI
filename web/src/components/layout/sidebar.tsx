import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import {
  LayoutDashboard,
  BookOpen,
  Server,
  GitBranch,
  Shield,
  ShieldCheck,
  Target,
  Code2,
  Radio,
  BarChart2,
  Bell,
  Settings,
  Users,
  ClipboardList,
  Link2,
  PackageOpen,
  UserCircle2,
} from "lucide-react";
import { cn } from "@/lib/cn";
import { useAuthStore } from "@/stores/auth-store";

interface NavItem {
  to: string;
  icon: React.ReactNode;
  labelKey: string;
}

// Paths below are validated against routeTree.gen.ts. TanStack Router does
// not require trailing slashes for index routes (`/agents` matches the
// `/agents/` index route).
const USER_NAV_ITEMS: NavItem[] = [
  { to: "/dashboard", icon: <LayoutDashboard className="h-4 w-4" />, labelKey: "nav.dashboard" },
  { to: "/subscriptions", icon: <BookOpen className="h-4 w-4" />, labelKey: "nav.subscriptions" },
  { to: "/nodes", icon: <Server className="h-4 w-4" />, labelKey: "nav.nodes" },
  { to: "/pipelines", icon: <GitBranch className="h-4 w-4" />, labelKey: "nav.pipelines" },
  { to: "/rules", icon: <Shield className="h-4 w-4" />, labelKey: "nav.rules" },
  { to: "/rule-sets", icon: <ShieldCheck className="h-4 w-4" />, labelKey: "nav.rule_sets" },
  { to: "/proxy-groups", icon: <Target className="h-4 w-4" />, labelKey: "nav.proxy_groups" },
  { to: "/scripts", icon: <Code2 className="h-4 w-4" />, labelKey: "nav.scripts" },
  { to: "/agents", icon: <Radio className="h-4 w-4" />, labelKey: "nav.agents" },
  { to: "/traffic", icon: <BarChart2 className="h-4 w-4" />, labelKey: "nav.traffic" },
  { to: "/notifications", icon: <Bell className="h-4 w-4" />, labelKey: "nav.notify" },
  { to: "/shortlinks", icon: <Link2 className="h-4 w-4" />, labelKey: "nav.shortlinks" },
];

const ADMIN_NAV_ITEMS: NavItem[] = [
  { to: "/admin/users", icon: <Users className="h-4 w-4" />, labelKey: "nav.users" },
  { to: "/admin/audit", icon: <ClipboardList className="h-4 w-4" />, labelKey: "nav.audit" },
  { to: "/admin/ota", icon: <PackageOpen className="h-4 w-4" />, labelKey: "nav.ota" },
  { to: "/admin/settings", icon: <Settings className="h-4 w-4" />, labelKey: "nav.settings" },
];

const FOOTER_NAV_ITEMS: NavItem[] = [
  { to: "/profile", icon: <UserCircle2 className="h-4 w-4" />, labelKey: "nav.profile" },
];

function NavLink({ item }: { item: NavItem }) {
  const { t } = useTranslation("common");
  return (
    <Link
      // Sidebar items mix concrete paths and index-route shorthand
      // (`/agents` → `/agents/`). TanStack Router resolves both, but the
      // typed `to` union does not contain shorthand forms, so we cast.
      to={item.to as unknown as "/"}
      className={cn(
        "flex items-center gap-2.5 rounded-[var(--radius-md)] px-3 py-1.5",
        "text-[var(--font-size-sm)] font-medium text-[var(--color-text-secondary)]",
        "transition-colors duration-[var(--duration-fast)]",
        "hover:bg-[var(--color-surface)] hover:text-[var(--color-text-primary)]",
        "[&.active]:bg-[var(--color-primary-soft)] [&.active]:text-[var(--color-text-primary)] [&.active]:shadow-[inset_2px_0_0_var(--color-primary)]",
      )}
    >
      {item.icon}
      {t(item.labelKey)}
    </Link>
  );
}

function NavGroup({ title, items }: { title?: string; items: NavItem[] }) {
  return (
    <div className="flex flex-col gap-0.5">
      {title && (
        <p className="mb-1 px-3 text-xs font-semibold uppercase tracking-wider text-[var(--color-text-disabled)]">
          {title}
        </p>
      )}
      {items.map((item) => (
        <NavLink key={item.to} item={item} />
      ))}
    </div>
  );
}

/** Sidebar navigation. Renders admin-only groups when user role is 'admin'. */
export function Sidebar() {
  const { t } = useTranslation("common");
  const { user } = useAuthStore();
  const isAdmin = user?.role === "admin";

  return (
    <nav
      className="flex w-60 flex-col gap-4 border-r border-[var(--color-border)] bg-[rgba(15,15,18,0.45)] p-3 backdrop-blur-xl"
      style={{ gridArea: "sidebar" }}
    >
      <NavGroup items={USER_NAV_ITEMS} />

      {isAdmin && (
        <NavGroup title={t("nav.admin")} items={ADMIN_NAV_ITEMS} />
      )}

      <div className="mt-auto">
        <NavGroup items={FOOTER_NAV_ITEMS} />
      </div>
    </nav>
  );
}
