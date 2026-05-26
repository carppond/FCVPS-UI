/**
 * T-29: Cmd+K command palette.
 *
 * Five groups, in display order:
 *   1. Recent  — last 5 jumps (localStorage)
 *   2. Pages   — sidebar destinations
 *   3. Actions — quick mutations (new sub / new agent / sync-all / toggles)
 *   4. Admin   — admin-only ops (backup / OTA check / silent-mode rotate)
 *   5. Resources — debounced cross-collection search (nodes + subs + agents)
 */
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { Command } from "cmdk";
import {
  BarChart2,
  BellRing,
  BookOpen,
  Bell,
  Bot,
  ClipboardList,
  Code2,
  Database,
  GitBranch,
  Globe,
  HardDrive,
  LayoutDashboard,
  Moon,
  Plus,
  Radio,
  RefreshCcw,
  Search,
  Server,
  Settings,
  Shield,
  ShieldAlert,
  Sun,
  Users,
  WifiOff,
} from "lucide-react";

import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { cn } from "@/lib/cn";
import i18n from "@/lib/i18n";
import { useCmdK, useCmdKShortcut, pushRecent, loadRecents } from "@/hooks/use-cmd-k";
import { useDebounce } from "@/hooks/use-debounce";
import { useAuthStore } from "@/stores/auth-store";
import { useUIStore } from "@/stores/ui-store";
import { useCmdKActions } from "@/components/layout/cmd-k-actions";

import { useNodesQuery } from "@/api/node";
import { useSubscriptionsQuery } from "@/api/subscription";
import { useAgentsQuery } from "@/api/agent";

import zhCNCmdk from "@/locales/zh-CN/cmdk.json";
import enCmdk from "@/locales/en/cmdk.json";
import jaCmdk from "@/locales/ja/cmdk.json";
import koCmdk from "@/locales/ko/cmdk.json";

function ensureCmdkBundles() {
  if (!i18n.hasResourceBundle("zh-CN", "cmdk")) {
    i18n.addResourceBundle("zh-CN", "cmdk", zhCNCmdk, true, true);
  }
  if (!i18n.hasResourceBundle("en", "cmdk")) {
    i18n.addResourceBundle("en", "cmdk", enCmdk, true, true);
  }
  if (!i18n.hasResourceBundle("ja", "cmdk")) {
    i18n.addResourceBundle("ja", "cmdk", jaCmdk, true, true);
  }
  if (!i18n.hasResourceBundle("ko", "cmdk")) {
    i18n.addResourceBundle("ko", "cmdk", koCmdk, true, true);
  }
}

interface PageEntry {
  id: string;
  to: string;
  icon: React.ReactNode;
  labelKey: string;
  adminOnly?: boolean;
}

const PAGES: PageEntry[] = [
  { id: "page:dashboard", to: "/dashboard", icon: <LayoutDashboard className="h-4 w-4" />, labelKey: "pages.dashboard" },
  { id: "page:subscriptions", to: "/subscriptions", icon: <BookOpen className="h-4 w-4" />, labelKey: "pages.subscriptions" },
  { id: "page:nodes", to: "/nodes", icon: <Server className="h-4 w-4" />, labelKey: "pages.nodes" },
  { id: "page:pipelines", to: "/pipelines", icon: <GitBranch className="h-4 w-4" />, labelKey: "pages.pipelines" },
  { id: "page:rules", to: "/rules", icon: <Shield className="h-4 w-4" />, labelKey: "pages.rules" },
  { id: "page:scripts", to: "/scripts", icon: <Code2 className="h-4 w-4" />, labelKey: "pages.scripts" },
  { id: "page:vps-assets", to: "/vps-assets", icon: <HardDrive className="h-4 w-4" />, labelKey: "pages.vps_assets" },
  { id: "page:agents", to: "/agents", icon: <Radio className="h-4 w-4" />, labelKey: "pages.agents" },
  { id: "page:traffic", to: "/traffic", icon: <BarChart2 className="h-4 w-4" />, labelKey: "pages.traffic" },
  { id: "page:notifications", to: "/notifications", icon: <Bell className="h-4 w-4" />, labelKey: "pages.notifications" },
  { id: "page:settings", to: "/admin/settings", icon: <Settings className="h-4 w-4" />, labelKey: "pages.settings", adminOnly: true },
  { id: "page:users", to: "/admin/users", icon: <Users className="h-4 w-4" />, labelKey: "pages.users", adminOnly: true },
  { id: "page:audit", to: "/admin/audit", icon: <ClipboardList className="h-4 w-4" />, labelKey: "pages.audit", adminOnly: true },
];

export function CmdK() {
  const { t, i18n: i18next } = useTranslation(["cmdk", "common"]);
  const [bundlesReady, setBundlesReady] = useState(false);
  const { open, setOpen, close } = useCmdK();
  useCmdKShortcut();

  useEffect(() => {
    ensureCmdkBundles();
    setBundlesReady(true);
  }, []);

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebounce(search, 200);

  useEffect(() => {
    if (!open) setSearch("");
  }, [open]);

  const navigate = useNavigate();
  const { user } = useAuthStore();
  const { theme } = useUIStore();
  const isAdmin = user?.role === "admin";
  const actions = useCmdKActions(close);

  const enabledSearch = debouncedSearch.trim().length >= 1;

  const nodesQ = useNodesQuery(
    enabledSearch
      ? { search: debouncedSearch, page: 1, pageSize: 10 }
      : { page: 1, pageSize: 0 },
  );
  const subsQ = useSubscriptionsQuery(
    enabledSearch
      ? { keyword: debouncedSearch, page: 1, pageSize: 10 }
      : { page: 1, pageSize: 0 },
  );
  const agentsQ = useAgentsQuery(
    enabledSearch
      ? { keyword: debouncedSearch, page: 1, pageSize: 10 }
      : { page: 1, pageSize: 0 },
  );

  const isResourcesLoading =
    enabledSearch && (nodesQ.isFetching || subsQ.isFetching || agentsQ.isFetching);

  const nodes = enabledSearch ? (nodesQ.data?.items ?? []) : [];
  const subs = enabledSearch ? (subsQ.data?.items ?? []) : [];
  const agents = enabledSearch ? (agentsQ.data?.items ?? []) : [];

  const [recents, setRecents] = useState(loadRecents());
  useEffect(() => {
    if (open) setRecents(loadRecents());
  }, [open]);

  const goTo = (page: PageEntry) => {
    pushRecent({ id: page.id, label: t(`cmdk:${page.labelKey}`), to: page.to });
    close();
    void navigate({ to: page.to as never });
  };

  const goToNode = (id: string, name: string) => {
    const path = `/nodes/${id}`;
    pushRecent({ id: `node:${id}`, label: name, to: path });
    close();
    void navigate({ to: "/nodes/$nodeId" as never, params: { nodeId: id } as never });
  };

  const goToSub = (id: string, name: string) => {
    const path = `/subscriptions/${id}`;
    pushRecent({ id: `sub:${id}`, label: name, to: path });
    close();
    void navigate({ to: "/subscriptions/$id" as never, params: { id } as never });
  };

  const goToAgentDetail = () => {
    pushRecent({ id: "page:agents", label: t("cmdk:pages.agents"), to: "/agents" });
    close();
    void navigate({ to: "/agents" as never });
  };

  const visiblePages = useMemo(
    () => PAGES.filter((p) => !p.adminOnly || isAdmin),
    [isAdmin],
  );

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent
        className="max-w-[560px] gap-0 overflow-hidden rounded-2xl border-white/[.08] p-0 shadow-[0_32px_80px_rgba(0,0,0,0.6),0_0_0_1px_rgba(255,255,255,0.04)]"
        data-cmdk-root
      >
        <DialogTitle className="sr-only">
          {bundlesReady ? t("cmdk:title") : "Command palette"}
        </DialogTitle>
        <Command
          label="Command Menu"
          loop
          className={cn(
            "flex max-h-[32rem] flex-col",
            "[&_[cmdk-group-heading]]:px-4",
            "[&_[cmdk-group-heading]]:pt-3",
            "[&_[cmdk-group-heading]]:pb-1.5",
            "[&_[cmdk-group-heading]]:text-[10px]",
            "[&_[cmdk-group-heading]]:font-bold",
            "[&_[cmdk-group-heading]]:uppercase",
            "[&_[cmdk-group-heading]]:tracking-[.08em]",
            "[&_[cmdk-group-heading]]:text-[var(--color-text-disabled)]",
          )}
        >
          {/* Search input */}
          <div className="flex items-center gap-3 border-b border-[var(--color-border)] px-4">
            <Search className="h-4 w-4 shrink-0 text-[var(--color-text-tertiary)]" />
            <Command.Input
              value={search}
              onValueChange={setSearch}
              placeholder={bundlesReady ? t("cmdk:placeholder") : "Search…"}
              autoFocus
              className={cn(
                "h-[52px] w-full bg-transparent text-[15px]",
                "text-[var(--color-text-primary)] outline-none placeholder:text-[var(--color-text-disabled)]",
              )}
            />
            <kbd className="shrink-0 rounded-md border border-[var(--color-border)] bg-white/[.04] px-1.5 py-0.5 font-mono text-[10px] text-[var(--color-text-disabled)]">
              ESC
            </kbd>
          </div>

          <Command.List
            className={cn(
              "max-h-[26rem] overflow-y-auto px-2 py-2",
              "[scrollbar-width:thin] [scrollbar-color:rgba(255,255,255,.08)_transparent]",
              "[&::-webkit-scrollbar]:w-1 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-white/[.08]",
            )}
          >
            <Command.Empty className="flex flex-col items-center gap-2 py-10 text-center">
              <Search className="h-8 w-8 text-[var(--color-text-disabled)]" />
              <span className="text-[13px] text-[var(--color-text-tertiary)]">
                {bundlesReady ? t("cmdk:empty") : "No results"}
              </span>
            </Command.Empty>

            {isResourcesLoading && (
              <Command.Loading className="px-4 py-2 text-[11px] text-[var(--color-text-disabled)]">
                {t("cmdk:loading")}
              </Command.Loading>
            )}

            {/* Recent */}
            {recents.length > 0 && (
              <Command.Group heading={t("cmdk:groups.recent")}>
                {recents.map((r) => (
                  <CmdItem
                    key={r.id}
                    value={`recent ${r.id} ${r.label}`}
                    onSelect={() => actions.goToRecent(r)}
                    icon={<RefreshCcw className="h-3.5 w-3.5" />}
                    iconBg="rgba(255,255,255,.04)"
                    iconColor="var(--color-text-tertiary)"
                    label={r.label}
                    hint={r.to}
                  />
                ))}
              </Command.Group>
            )}

            {/* Pages */}
            <Command.Group heading={t("cmdk:groups.pages")}>
              {visiblePages.map((p) => (
                <CmdItem
                  key={p.id}
                  value={`page ${p.to} ${t(`cmdk:${p.labelKey}`)}`}
                  onSelect={() => goTo(p)}
                  icon={p.icon}
                  iconBg="rgba(255,255,255,.04)"
                  iconColor="var(--color-text-secondary)"
                  label={t(`cmdk:${p.labelKey}`)}
                  hint={p.to}
                />
              ))}
            </Command.Group>

            {/* Actions */}
            <Command.Group heading={t("cmdk:groups.actions")}>
              <CmdItem
                value="action new-sub"
                onSelect={actions.handleCreateSub}
                icon={<Plus className="h-3.5 w-3.5" />}
                iconBg="rgba(255,99,99,.1)"
                iconColor="var(--color-primary)"
                label={t("cmdk:actions.create_subscription")}
              />
              <CmdItem
                value="action new-agent"
                onSelect={actions.handleCreateAgent}
                icon={<Bot className="h-3.5 w-3.5" />}
                iconBg="rgba(255,99,99,.1)"
                iconColor="var(--color-primary)"
                label={t("cmdk:actions.create_agent")}
              />
              <CmdItem
                value="action sync-all"
                onSelect={actions.handleSyncAll}
                icon={<RefreshCcw className="h-3.5 w-3.5" />}
                iconBg="rgba(96,165,250,.1)"
                iconColor="var(--color-info)"
                label={t("cmdk:actions.sync_all")}
              />
              <CmdItem
                value="action toggle-theme"
                onSelect={actions.handleToggleTheme}
                icon={
                  theme === "dark" ? (
                    <Moon className="h-3.5 w-3.5" />
                  ) : (
                    <Sun className="h-3.5 w-3.5" />
                  )
                }
                iconBg="rgba(255,255,255,.04)"
                iconColor="var(--color-text-secondary)"
                label={t("cmdk:actions.toggle_theme")}
                hint={t(`common:theme.${theme}`)}
              />
              <CmdItem
                value="action toggle-lang"
                onSelect={actions.handleToggleLang}
                icon={<Globe className="h-3.5 w-3.5" />}
                iconBg="rgba(255,255,255,.04)"
                iconColor="var(--color-text-secondary)"
                label={t("cmdk:actions.toggle_lang")}
                hint={i18next.language}
              />
            </Command.Group>

            {/* Admin */}
            {isAdmin && (
              <Command.Group heading={t("cmdk:groups.admin")}>
                <CmdItem
                  value="admin backup"
                  onSelect={actions.handleBackup}
                  icon={<Database className="h-3.5 w-3.5" />}
                  iconBg="rgba(96,165,250,.1)"
                  iconColor="var(--color-info)"
                  label={t("cmdk:admin.backup")}
                />
                <CmdItem
                  value="admin ota"
                  onSelect={actions.handleOtaCheck}
                  icon={<WifiOff className="h-3.5 w-3.5" />}
                  iconBg="rgba(251,191,36,.1)"
                  iconColor="var(--color-warning)"
                  label={t("cmdk:admin.ota_check")}
                />
                <CmdItem
                  value="admin silent-rotate"
                  onSelect={actions.handleSilentRotate}
                  icon={<ShieldAlert className="h-3.5 w-3.5" />}
                  iconBg="rgba(248,113,113,.1)"
                  iconColor="var(--color-error)"
                  label={t("cmdk:admin.silent_rotate")}
                />
              </Command.Group>
            )}

            {/* Resources */}
            {enabledSearch && (subs.length > 0 || nodes.length > 0 || agents.length > 0) && (
              <Command.Group heading={t("cmdk:groups.resources")}>
                {subs.slice(0, 5).map((s) => (
                  <CmdItem
                    key={`sub:${s.id}`}
                    value={`sub ${s.id} ${s.name}`}
                    onSelect={() => goToSub(s.id, s.name)}
                    icon={<BookOpen className="h-3.5 w-3.5" />}
                    iconBg="rgba(255,255,255,.04)"
                    iconColor="var(--color-text-tertiary)"
                    label={s.name}
                    badge={t("cmdk:resource_kind.subscription")}
                  />
                ))}
                {nodes.slice(0, 5).map((n) => (
                  <CmdItem
                    key={`node:${n.id}`}
                    value={`node ${n.id} ${n.tag}`}
                    onSelect={() => goToNode(n.id, n.tag)}
                    icon={<Server className="h-3.5 w-3.5" />}
                    iconBg="rgba(255,255,255,.04)"
                    iconColor="var(--color-text-tertiary)"
                    label={n.tag}
                    badge={t("cmdk:resource_kind.node")}
                  />
                ))}
                {agents.slice(0, 5).map((a) => (
                  <CmdItem
                    key={`agent:${a.id}`}
                    value={`agent ${a.id} ${a.name}`}
                    onSelect={() => goToAgentDetail()}
                    icon={<BellRing className="h-3.5 w-3.5" />}
                    iconBg="rgba(255,255,255,.04)"
                    iconColor="var(--color-text-tertiary)"
                    label={a.name}
                    badge={t("cmdk:resource_kind.agent")}
                  />
                ))}
              </Command.Group>
            )}
          </Command.List>

          {/* Footer */}
          <div className="flex items-center justify-between border-t border-[var(--color-border)] px-4 py-2.5">
            <div className="flex items-center gap-3 text-[10px] text-[var(--color-text-disabled)]">
              <span className="flex items-center gap-1">
                <kbd className="rounded border border-[var(--color-border)] bg-white/[.03] px-1 py-0.5 font-mono text-[9px]">↑↓</kbd>
                {t("cmdk:hint.navigate")}
              </span>
              <span className="flex items-center gap-1">
                <kbd className="rounded border border-[var(--color-border)] bg-white/[.03] px-1 py-0.5 font-mono text-[9px]">↵</kbd>
                {t("cmdk:hint.enter")}
              </span>
            </div>
            <span className="text-[10px] text-[var(--color-text-disabled)]">
              {t("cmdk:hint.esc")}
            </span>
          </div>
        </Command>
      </DialogContent>
    </Dialog>
  );
}

interface CmdItemProps {
  value: string;
  onSelect: () => void;
  icon: React.ReactNode;
  iconBg: string;
  iconColor: string;
  label: string;
  hint?: string;
  badge?: string;
}

function CmdItem({ value, onSelect, icon, iconBg, iconColor, label, hint, badge }: CmdItemProps) {
  return (
    <Command.Item
      value={value}
      onSelect={onSelect}
      className={cn(
        "flex cursor-pointer items-center gap-3 rounded-[10px] px-3 py-2.5",
        "text-[13px] text-[var(--color-text-primary)]",
        "transition-colors duration-100",
        "data-[selected=true]:bg-white/[.05]",
        "aria-selected:bg-white/[.05]",
      )}
    >
      <span
        className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg"
        style={{ background: iconBg, color: iconColor }}
      >
        {icon}
      </span>
      <span className="grow truncate">{label}</span>
      {badge && (
        <span className="shrink-0 rounded-md bg-white/[.04] px-1.5 py-0.5 text-[10px] font-medium text-[var(--color-text-disabled)]">
          {badge}
        </span>
      )}
      {hint && (
        <span className="shrink-0 font-mono text-[11px] text-[var(--color-text-disabled)]">
          {hint}
        </span>
      )}
    </Command.Item>
  );
}
