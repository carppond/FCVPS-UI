import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Plus, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useDebounce } from "@/hooks/use-debounce";
import { RuleList } from "@/components/rule/rule-list";
import { RuleForm } from "@/components/rule/rule-form";
import { RulePreviewPane } from "@/components/rule/rule-preview-pane";
import { useRulesQuery } from "@/api/rule";
import i18n from "@/lib/i18n";
import ruleZh from "@/locales/zh-CN/rule.json";
import ruleEn from "@/locales/en/rule.json";
import ruleJa from "@/locales/ja/rule.json";
import ruleKo from "@/locales/ko/rule.json";
import type { CustomRule } from "@/types/api";

// Lazy-register the "rule" namespace before the route mounts; mirrors the
// /nodes route's strategy so first-screen bundles stay slim.
function ensureRuleNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "rule")) {
    i18n.addResourceBundle("zh-CN", "rule", ruleZh, true, true);
    i18n.addResourceBundle("en", "rule", ruleEn, true, true);
    i18n.addResourceBundle("ja", "rule", ruleJa, true, true);
    i18n.addResourceBundle("ko", "rule", ruleKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/rules")({
  beforeLoad: () => {
    ensureRuleNamespace();
  },
  component: RulesPage,
});

/**
 * Three-column layout: rule list / edit form / preview.
 *
 *  - Left: scrollable, drag-reorder rule list with enable toggle.
 *  - Center: form for the currently selected rule (or "new rule" mode).
 *  - Right: subscription picker + final Clash YAML preview (Monaco read-only).
 *
 * State is intentionally minimal — every server interaction owns its own
 * cache invalidation via TanStack Query, so the parent route only tracks
 * which rule is selected + the keyword filter.
 */
function RulesPage() {
  const { t } = useTranslation(["rule", "common"]);

  const [searchInput, setSearchInput] = React.useState("");
  const keyword = useDebounce(searchInput, 250);

  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const [creating, setCreating] = React.useState(false);

  const { data, isLoading, isError, error, refetch } = useRulesQuery({
    page: 1,
    pageSize: 200,
    keyword,
  });
  const rules = data?.items;

  // Resolve the selected rule object from the current list snapshot. When the
  // selection disappears (deleted / filtered out) we reset to "no selection".
  const selectedRule = React.useMemo(() => {
    if (creating) return null;
    if (!selectedId) return null;
    return rules?.find((r) => r.id === selectedId) ?? null;
  }, [selectedId, rules, creating]);

  React.useEffect(() => {
    if (!creating && selectedId && rules && !rules.find((r) => r.id === selectedId)) {
      setSelectedId(null);
    }
  }, [rules, selectedId, creating]);

  const handleSelect = (rule: CustomRule | null) => {
    setCreating(false);
    setSelectedId(rule?.id ?? null);
  };

  const handleNew = () => {
    setSelectedId(null);
    setCreating(true);
  };

  const handleSaved = (rule: CustomRule | null) => {
    setCreating(false);
    if (rule) setSelectedId(rule.id);
  };

  return (
    <div className="flex h-[calc(100vh-var(--app-header-height,3.5rem))] flex-col">
      <header className="border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-6 py-4">
        <h1 className="text-[var(--font-size-xl)] font-semibold text-[var(--color-text-primary)]">
          {t("rule:title")}
        </h1>
        <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("rule:subtitle")}
        </p>
      </header>

      <div className="grid min-h-0 flex-1 grid-cols-[20rem,minmax(0,1fr),28rem]">
        {/* ── Left: list ─────────────────────────────────────────────────── */}
        <section className="flex min-h-0 flex-col border-r border-[var(--color-border)] bg-[var(--color-bg)]">
          <div className="flex items-center justify-between gap-2 border-b border-[var(--color-border)] p-3">
            <div className="relative flex-1">
              <Search className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
              <Input
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder={t("rule:list.search_placeholder")}
                className="pl-7"
              />
            </div>
            <Button size="sm" onClick={handleNew}>
              <Plus className="mr-1 h-3.5 w-3.5" />
              {t("rule:list.add_rule")}
            </Button>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto">
            <RuleList
              rules={rules}
              isLoading={isLoading}
              isError={isError}
              errorMessage={error?.message}
              onRetry={() => void refetch()}
              selectedId={selectedId}
              onSelect={handleSelect}
              onNew={handleNew}
            />
          </div>
        </section>

        {/* ── Center: form ───────────────────────────────────────────────── */}
        <section className="min-h-0 overflow-y-auto">
          {creating || selectedRule ? (
            <RuleForm
              rule={selectedRule}
              onSaved={handleSaved}
              onCancel={() => {
                setCreating(false);
                setSelectedId(null);
              }}
            />
          ) : (
            <div className="flex h-full items-center justify-center p-6 text-center text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
              {t("rule:form.select_rule_hint")}
            </div>
          )}
        </section>

        {/* ── Right: preview ─────────────────────────────────────────────── */}
        <RulePreviewPane />
      </div>
    </div>
  );
}
