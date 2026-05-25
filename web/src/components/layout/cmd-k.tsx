/**
 * T-29: Cmd+K command palette (replaces the T-2 placeholder).
 *
 * Five groups, in display order:
 *   1. Recent  — last 5 jumps (localStorage)
 *   2. Pages   — sidebar destinations
 *   3. Actions — quick mutations (new sub / new agent / sync-all / toggles)
 *   4. Admin   — admin-only ops (backup / OTA check / silent-mode rotate)
 *   5. Resources — debounced cross-collection search (nodes + subs + agents)
 *
 * Keyboard: ↑↓ navigation, Enter selects, Esc closes — all provided by `cmdk`.
 * Cmd+K to toggle is registered globally via `useCmdKShortcut`.
 *
 * The "Resources" group is filled in parallel by `useQueries`-style independent
 * hooks; we deliberately do NOT add a server `/api/search` endpoint per the
 * task brief (frontend-only fan-out is sufficient and avoids a new backend
 * surface).
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
  LayoutDashboard,
  Moon,
  Plus,
  Radio,
  RefreshCcw,
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

// API hooks (resource search only — action mutations live in cmd-k-actions.ts)
import { useNodesQuery } from "@/api/node";
import { useSubscriptionsQuery } from "@/api/subscription";
import { useAgentsQuery } from "@/api/agent";

// Locale bundles for the cmdk namespace — eagerly registered the first time the
// dialog mounts so opening the palette never flashes raw keys.
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

// ── page nav table ──────────────────────────────────────────────────────────

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
  { id: "page:agents", to: "/agents", icon: <Radio className="h-4 w-4" />, labelKey: "pages.agents" },
  { id: "page:traffic", to: "/traffic", icon: <BarChart2 className="h-4 w-4" />, labelKey: "pages.traffic" },
  { id: "page:notifications", to: "/notifications", icon: <Bell className="h-4 w-4" />, labelKey: "pages.notifications" },
  { id: "page:settings", to: "/settings", icon: <Settings className="h-4 w-4" />, labelKey: "pages.settings", adminOnly: true },
  { id: "page:users", to: "/users", icon: <Users className="h-4 w-4" />, labelKey: "pages.users", adminOnly: true },
  { id: "page:audit", to: "/audit", icon: <ClipboardList className="h-4 w-4" />, labelKey: "pages.audit", adminOnly: true },
];

// ── main component ──────────────────────────────────────────────────────────

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

  // Reset the input each time the palette re-opens so the previous query
  // does not leak between sessions.
  useEffect(() => {
    if (!open) setSearch("");
  }, [open]);

  const navigate = useNavigate();
  const { user } = useAuthStore();
  const { theme } = useUIStore();
  const isAdmin = user?.role === "admin";
  const actions = useCmdKActions(close);

  // ── resource search (only when query is non-empty) ───────────────────────
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

  // ── recent entries (re-read each open so navigation elsewhere updates) ──
  const [recents, setRecents] = useState(loadRecents());
  useEffect(() => {
    if (open) setRecents(loadRecents());
  }, [open]);

  // ── nav handlers (action / mutation handlers are in `actions`) ───────────
  const goTo = (page: PageEntry) => {
    pushRecent({ id: page.id, label: t(`cmdk:${page.labelKey}`), to: page.to });
    close();
    void navigate({ to: page.to as never });
  };

  const goToNode = (id: string, name: string) => {
    const path = `/nodes/${id}`;
    pushRecent({ id: `node:${id}`, label: name, to: path });
    close();
    void navigate({
      to: "/nodes/$nodeId" as never,
      params: { nodeId: id } as never,
    });
  };

  const goToSub = (id: string, name: string) => {
    const path = `/subscriptions/${id}`;
    pushRecent({ id: `sub:${id}`, label: name, to: path });
    close();
    void navigate({
      to: "/subscriptions/$id" as never,
      params: { id } as never,
    });
  };

  const goToAgentDetail = () => {
    pushRecent({ id: "page:agents", label: t("cmdk:pages.agents"), to: "/agents" });
    close();
    void navigate({ to: "/agents" as never });
  };

  // ── render ───────────────────────────────────────────────────────────────

  const visiblePages = useMemo(
    () => PAGES.filter((p) => !p.adminOnly || isAdmin),
    [isAdmin],
  );

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent
        className="max-w-xl gap-0 overflow-hidden p-0 border-white/12 shadow-[0_24px_64px_rgba(0,0,0,0.5)]"
        data-cmdk-root
      >
        <DialogTitle className="sr-only">
          {bundlesReady ? t("cmdk:title") : "Command palette"}
        </DialogTitle>
        <Command
          label="Command Menu"
          loop
          className={cn(
            // 30rem = 480px, on the rem-based size scale (token阶梯 audited).
            "flex max-h-[30rem] flex-col",
            "[&_[cmdk-group-heading]]:px-3",
            "[&_[cmdk-group-heading]]:pt-3",
            "[&_[cmdk-group-heading]]:pb-1",
            "[&_[cmdk-group-heading]]:text-[var(--font-size-xs)]",
            "[&_[cmdk-group-heading]]:font-medium",
            "[&_[cmdk-group-heading]]:uppercase",
            "[&_[cmdk-group-heading]]:tracking-wider",
            "[&_[cmdk-group-heading]]:text-[var(--color-text-tertiary)]",
          )}
        >
          <div className="flex items-center border-b border-[var(--color-border)] px-3">
            <Command.Input
              value={search}
              onValueChange={setSearch}
              placeholder={bundlesReady ? t("cmdk:placeholder") : "Search…"}
              autoFocus
              className={cn(
                "h-12 w-full bg-transparent text-[var(--font-size-base)]",
                "text-[var(--color-text-primary)] outline-none placeholder:text-[var(--color-text-tertiary)]",
              )}
            />
          </div>

          <Command.List className="max-h-96 overflow-y-auto p-2">
            <Command.Empty className="py-8 text-center text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
              {bundlesReady ? t("cmdk:empty") : "No results"}
            </Command.Empty>

            {isResourcesLoading && (
              <Command.Loading className="px-3 py-2 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                {t("cmdk:loading")}
              </Command.Loading>
            )}

            {/* 1. Recent */}
            {recents.length > 0 && (
              <Command.Group heading={t("cmdk:groups.recent")}>
                {recents.map((r) => (
                  <CmdItem
                    key={r.id}
                    value={`recent ${r.id} ${r.label}`}
                    onSelect={() => actions.goToRecent(r)}
                    icon={<RefreshCcw className="h-4 w-4 text-[var(--color-text-tertiary)]" />}
                    label={r.label}
                    hint={r.to}
                  />
                ))}
              </Command.Group>
            )}

            {/* 2. Pages */}
            <Command.Group heading={t("cmdk:groups.pages")}>
              {visiblePages.map((p) => (
                <CmdItem
                  key={p.id}
                  value={`page ${p.to} ${t(`cmdk:${p.labelKey}`)}`}
                  onSelect={() => goTo(p)}
                  icon={p.icon}
                  label={t(`cmdk:${p.labelKey}`)}
                  hint={p.to}
                />
              ))}
            </Command.Group>

            {/* 3. Actions */}
            <Command.Group heading={t("cmdk:groups.actions")}>
              <CmdItem
                value="action new-sub"
                onSelect={actions.handleCreateSub}
                icon={<Plus className="h-4 w-4 text-[var(--color-primary)]" />}
                label={t("cmdk:actions.create_subscription")}
              />
              <CmdItem
                value="action new-agent"
                onSelect={actions.handleCreateAgent}
                icon={<Bot className="h-4 w-4 text-[var(--color-primary)]" />}
                label={t("cmdk:actions.create_agent")}
              />
              <CmdItem
                value="action sync-all"
                onSelect={actions.handleSyncAll}
                icon={<RefreshCcw className="h-4 w-4 text-[var(--color-info)]" />}
                label={t("cmdk:actions.sync_all")}
              />
              <CmdItem
                value="action toggle-theme"
                onSelect={actions.handleToggleTheme}
                icon={
                  theme === "dark" ? (
                    <Moon className="h-4 w-4 text-[var(--color-text-secondary)]" />
                  ) : (
                    <Sun className="h-4 w-4 text-[var(--color-text-secondary)]" />
                  )
                }
                label={t("cmdk:actions.toggle_theme")}
                hint={t(`common:theme.${theme}`)}
              />
              <CmdItem
                value="action toggle-lang"
                onSelect={actions.handleToggleLang}
                icon={<Globe className="h-4 w-4 text-[var(--color-text-secondary)]" />}
                label={t("cmdk:actions.toggle_lang")}
                hint={i18next.language}
              />
            </Command.Group>

            {/* 4. Admin */}
            {isAdmin && (
              <Command.Group heading={t("cmdk:groups.admin")}>
                <CmdItem
                  value="admin backup"
                  onSelect={actions.handleBackup}
                  icon={<Database className="h-4 w-4 text-[var(--color-info)]" />}
                  label={t("cmdk:admin.backup")}
                />
                <CmdItem
                  value="admin ota"
                  onSelect={actions.handleOtaCheck}
                  icon={<WifiOff className="h-4 w-4 text-[var(--color-warning)]" />}
                  label={t("cmdk:admin.ota_check")}
                />
                <CmdItem
                  value="admin silent-rotate"
                  onSelect={actions.handleSilentRotate}
                  icon={<ShieldAlert className="h-4 w-4 text-[var(--color-error)]" />}
                  label={t("cmdk:admin.silent_rotate")}
                />
              </Command.Group>
            )}

            {/* 5. Resources */}
            {enabledSearch && (subs.length > 0 || nodes.length > 0 || agents.length > 0) && (
              <Command.Group heading={t("cmdk:groups.resources")}>
                {subs.slice(0, 5).map((s) => (
                  <CmdItem
                    key={`sub:${s.id}`}
                    value={`sub ${s.id} ${s.name}`}
                    onSelect={() => goToSub(s.id, s.name)}
                    icon={<BookOpen className="h-4 w-4 text-[var(--color-text-tertiary)]" />}
                    label={s.name}
                    hint={t("cmdk:resource_kind.subscription")}
                  />
                ))}
                {nodes.slice(0, 5).map((n) => (
                  <CmdItem
                    key={`node:${n.id}`}
                    value={`node ${n.id} ${n.tag}`}
                    onSelect={() => goToNode(n.id, n.tag)}
                    icon={<Server className="h-4 w-4 text-[var(--color-text-tertiary)]" />}
                    label={n.tag}
                    hint={t("cmdk:resource_kind.node")}
                  />
                ))}
                {agents.slice(0, 5).map((a) => (
                  <CmdItem
                    key={`agent:${a.id}`}
                    value={`agent ${a.id} ${a.name}`}
                    onSelect={() => goToAgentDetail()}
                    icon={<BellRing className="h-4 w-4 text-[var(--color-text-tertiary)]" />}
                    label={a.name}
                    hint={t("cmdk:resource_kind.agent")}
                  />
                ))}
              </Command.Group>
            )}
          </Command.List>

          <div className="flex items-center justify-between border-t border-[var(--color-border)] px-3 py-2 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            <span>{t("cmdk:hint.navigate")}</span>
            <span>
              {t("cmdk:hint.enter")} · {t("cmdk:hint.esc")}
            </span>
          </div>
        </Command>
      </DialogContent>
    </Dialog>
  );
}

// ── shared item renderer ────────────────────────────────────────────────────

interface CmdItemProps {
  value: string;
  onSelect: () => void;
  icon: React.ReactNode;
  label: string;
  hint?: string;
}

function CmdItem({ value, onSelect, icon, label, hint }: CmdItemProps) {
  return (
    <Command.Item
      value={value}
      onSelect={onSelect}
      className={cn(
        "flex cursor-pointer items-center gap-3 rounded-[var(--radius-md)] px-3 py-2",
        "text-[var(--font-size-sm)] text-[var(--color-text-primary)]",
        "data-[selected=true]:bg-[var(--color-surface-hover)]",
        "aria-selected:bg-[var(--color-surface-hover)]",
      )}
    >
      <span className="flex h-5 w-5 items-center justify-center">{icon}</span>
      <span className="grow truncate">{label}</span>
      {hint && (
        <span className="ml-2 shrink-0 font-mono text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {hint}
        </span>
      )}
    </Command.Item>
  );
}
