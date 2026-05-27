import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { LayoutTemplate, Plus, RefreshCw, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "@/components/ui/toast";
import { useDebounce } from "@/hooks/use-debounce";
import { useApiError } from "@/hooks/use-api-error";
import { RuleSetList } from "@/components/rule-set/rule-set-list";
import { RuleSetFormDialog } from "@/components/rule-set/rule-set-form-dialog";
import { RuleSetPresetPicker } from "@/components/rule-set/rule-set-preset-picker";
import { useRuleSets, useSyncAllRuleSets } from "@/api/rule-set";
import { cn } from "@/lib/cn";
import type { RuleSetProvider } from "@/types/api";

export const Route = createFileRoute("/_authed/rule-sets")({
  component: RuleSetsPage,
});

/**
 * Rule providers management page. Mirrors /rules' layout: a sticky top-bar
 * with search + "preset" + "new" CTAs, then a single DataTable that owns
 * row-level mutations (enable toggle, sync-now, delete).
 */
function RuleSetsPage() {
  const { t } = useTranslation(["rule-set", "common"]);

  const [searchInput, setSearchInput] = React.useState("");
  const keyword = useDebounce(searchInput, 250);

  const { handle: handleError } = useApiError();
  const [editing, setEditing] = React.useState<RuleSetProvider | null>(null);
  const [creating, setCreating] = React.useState(false);
  const [presetOpen, setPresetOpen] = React.useState(false);
  const syncAllMutation = useSyncAllRuleSets();

  const handleSyncAll = async () => {
    try {
      const res = await syncAllMutation.mutateAsync();
      toast.success(t("rule-set:toast.sync_all_ok", { ok: res.ok, failed: res.failed }));
    } catch (err) {
      handleError(err);
    }
  };

  const { data, isLoading, isError, error, refetch } = useRuleSets({
    page: 1,
    pageSize: 200,
    keyword,
  });
  const items = data?.items;

  const formOpen = creating || editing !== null;

  return (
    <div className="flex h-[calc(100vh-var(--app-header-height,3.5rem))] flex-col">
      <header className="border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-6 py-4">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="min-w-0">
            <h1 className="text-[var(--font-size-xl)] font-semibold text-[var(--color-text-primary)]">
              {t("rule-set:title")}
            </h1>
            <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
              {t("rule-set:subtitle")}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative">
              <Search className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
              <Input
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder={t("rule-set:list.search_placeholder")}
                className="w-64 pl-7"
              />
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={handleSyncAll}
              disabled={syncAllMutation.isPending}
            >
              <RefreshCw className={cn("h-3.5 w-3.5", syncAllMutation.isPending && "animate-spin")} />
              {t("rule-set:list.sync_all")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPresetOpen(true)}
            >
              <LayoutTemplate className="h-3.5 w-3.5" />
              {t("rule-set:list.from_preset")}
            </Button>
            <Button size="sm" onClick={() => setCreating(true)}>
              <Plus className="h-3.5 w-3.5" />
              {t("rule-set:list.add")}
            </Button>
          </div>
        </div>
      </header>

      <main className="flex-1 overflow-y-auto bg-[var(--color-bg)] p-6">
        <RuleSetList
          items={items}
          isLoading={isLoading}
          isError={isError}
          errorMessage={error?.message}
          onRetry={() => void refetch()}
          onEdit={(rs) => setEditing(rs)}
          onNew={() => setCreating(true)}
        />
      </main>

      <RuleSetFormDialog
        open={formOpen}
        onOpenChange={(open) => {
          if (!open) {
            setCreating(false);
            setEditing(null);
          }
        }}
        ruleSet={editing}
      />

      <RuleSetPresetPicker
        open={presetOpen}
        onOpenChange={setPresetOpen}
      />
    </div>
  );
}
