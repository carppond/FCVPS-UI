import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { LayoutTemplate, Plus, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ProxyGroupList } from "@/components/proxy-group/proxy-group-list";
import { ProxyGroupFormDialog } from "@/components/proxy-group/proxy-group-form-dialog";
import { ProxyGroupPresetPicker } from "@/components/proxy-group/proxy-group-preset-picker";
import { useProxyGroups } from "@/api/proxy-group";
import type { ProxyGroupCategory } from "@/types/api";

export const Route = createFileRoute("/_authed/proxy-groups")({
  component: ProxyGroupsPage,
});

/**
 * Proxy groups page. The /api/proxy-groups endpoint returns a flat array
 * (already sorted) — we filter client-side by name keyword so the search box
 * is instant, and we don't re-query while the user types.
 */
function ProxyGroupsPage() {
  const { t } = useTranslation(["proxy-group", "common"]);

  const [searchInput, setSearchInput] = React.useState("");
  const [editing, setEditing] = React.useState<ProxyGroupCategory | null>(null);
  const [creating, setCreating] = React.useState(false);
  const [presetOpen, setPresetOpen] = React.useState(false);

  const { data, isLoading, isError, error, refetch } = useProxyGroups();

  const filtered = React.useMemo(() => {
    if (!data) return undefined;
    const kw = searchInput.trim().toLowerCase();
    if (!kw) return data;
    return data.filter((g) => g.name.toLowerCase().includes(kw));
  }, [data, searchInput]);

  const formOpen = creating || editing !== null;

  return (
    <div className="flex h-[calc(100vh-var(--app-header-height,3.5rem))] flex-col">
      <header className="border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-6 py-4">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="min-w-0">
            <h1 className="text-[var(--font-size-xl)] font-semibold text-[var(--color-text-primary)]">
              {t("proxy-group:title")}
            </h1>
            <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
              {t("proxy-group:subtitle")}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative">
              <Search className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
              <Input
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder={t("proxy-group:list.search_placeholder")}
                className="w-64 pl-7"
              />
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPresetOpen(true)}
            >
              <LayoutTemplate className="h-3.5 w-3.5" />
              {t("proxy-group:list.from_preset")}
            </Button>
            <Button size="sm" onClick={() => setCreating(true)}>
              <Plus className="h-3.5 w-3.5" />
              {t("proxy-group:list.add")}
            </Button>
          </div>
        </div>
      </header>

      <main className="flex-1 overflow-y-auto bg-[var(--color-bg)] p-6">
        <ProxyGroupList
          items={filtered}
          isLoading={isLoading}
          isError={isError}
          errorMessage={error?.message}
          onRetry={() => void refetch()}
          onEdit={(g) => setEditing(g)}
          onNew={() => setCreating(true)}
        />
      </main>

      <ProxyGroupFormDialog
        open={formOpen}
        onOpenChange={(open) => {
          if (!open) {
            setCreating(false);
            setEditing(null);
          }
        }}
        group={editing}
      />

      <ProxyGroupPresetPicker
        open={presetOpen}
        onOpenChange={setPresetOpen}
      />
    </div>
  );
}
