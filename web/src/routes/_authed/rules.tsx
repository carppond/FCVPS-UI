import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Eye, LayoutTemplate, Plus, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { useDebounce } from "@/hooks/use-debounce";
import { RuleList } from "@/components/rule/rule-list";
import { RuleForm } from "@/components/rule/rule-form";
import { RulePreviewPane } from "@/components/rule/rule-preview-pane";
import { RuleTemplatesDialog } from "@/components/rule/rule-templates";
import { useRulesQuery } from "@/api/rule";
import i18n from "@/lib/i18n";
import ruleZh from "@/locales/zh-CN/rule.json";
import ruleEn from "@/locales/en/rule.json";
import ruleJa from "@/locales/ja/rule.json";
import ruleKo from "@/locales/ko/rule.json";
import type { CustomRule, RuleTemplate } from "@/types/api";

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
 * Rules management page — single-column DataTable + modal-driven editing.
 *
 *  - Header: title, search, "templates" trigger, "preview" trigger,
 *    "new rule" CTA.
 *  - Body: RuleList (DataTable rows with drag-reorder + enable toggle +
 *    overflow menu).
 *  - Dialog: RuleForm (create or edit, depending on `editingRule`).
 *  - Sheet: RulePreviewPane (subscription picker + Monaco YAML).
 *
 * The previous three-column layout collapsed badly on narrow viewports;
 * this version keeps the table comfortable at any width and pushes
 * secondary panes behind modal triggers.
 */
function RulesPage() {
  const { t } = useTranslation(["rule", "common"]);

  const [searchInput, setSearchInput] = React.useState("");
  const keyword = useDebounce(searchInput, 250);

  const [editingRule, setEditingRule] = React.useState<CustomRule | null>(null);
  const [creating, setCreating] = React.useState(false);
  const [previewOpen, setPreviewOpen] = React.useState(false);
  const [templateOpen, setTemplateOpen] = React.useState(false);
  // When the user picks a template from the top-bar dialog we forward it to
  // the form's default content via this seed.
  const [templateSeed, setTemplateSeed] =
    React.useState<RuleTemplate | null>(null);

  const { data, isLoading, isError, error, refetch } = useRulesQuery({
    page: 1,
    pageSize: 200,
    keyword,
  });
  const rules = data?.items;

  const dialogOpen = creating || editingRule !== null;

  const closeDialog = React.useCallback(() => {
    setCreating(false);
    setEditingRule(null);
    setTemplateSeed(null);
  }, []);

  const handleNew = () => {
    setEditingRule(null);
    setTemplateSeed(null);
    setCreating(true);
  };

  const handleEdit = (rule: CustomRule) => {
    setCreating(false);
    setTemplateSeed(null);
    setEditingRule(rule);
  };

  const handleSaved = (_rule: CustomRule | null) => {
    closeDialog();
  };

  // When the user picks a template, push its content + name into the form
  // via initialValues. Type defaults to "rules" since every built-in
  // template targets that section.
  const formInitialValues = React.useMemo(
    () =>
      templateSeed
        ? {
            name: templateSeed.name,
            content: templateSeed.content,
            type: "rules" as const,
          }
        : undefined,
    [templateSeed],
  );

  return (
    <div className="flex h-[calc(100vh-var(--app-header-height,3.5rem))] flex-col">
      {/* ── Top bar ─────────────────────────────────────────────────────── */}
      <header className="border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-6 py-4">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="min-w-0">
            <h1 className="text-[var(--font-size-xl)] font-semibold text-[var(--color-text-primary)]">
              {t("rule:title")}
            </h1>
            <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
              {t("rule:subtitle")}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative">
              <Search className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
              <Input
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder={t("rule:list.search_placeholder")}
                className="w-64 pl-7"
              />
            </div>
            <Button variant="outline" size="sm" onClick={() => setTemplateOpen(true)}>
              <LayoutTemplate className="h-3.5 w-3.5" />
              {t("rule:templates.button")}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setPreviewOpen(true)}>
              <Eye className="h-3.5 w-3.5" />
              {t("rule:preview.button")}
            </Button>
            <Button size="sm" onClick={handleNew}>
              <Plus className="h-3.5 w-3.5" />
              {t("rule:list.add_rule")}
            </Button>
          </div>
        </div>
      </header>

      {/* ── DataTable body ──────────────────────────────────────────────── */}
      <main className="flex-1 overflow-y-auto bg-[var(--color-bg)] p-6">
        <RuleList
          rules={rules}
          isLoading={isLoading}
          isError={isError}
          errorMessage={error?.message}
          onRetry={() => void refetch()}
          onEdit={handleEdit}
          onNew={handleNew}
        />
      </main>

      {/* ── Edit / Create dialog ───────────────────────────────────────── */}
      <Dialog open={dialogOpen} onOpenChange={(open) => { if (!open) closeDialog(); }}>
        <DialogContent className="max-w-3xl">
          <DialogHeader>
            <DialogTitle>
              {creating
                ? t("rule:form.new_title")
                : t("rule:form.edit_title")}
            </DialogTitle>
          </DialogHeader>
          <RuleForm
            // Re-key per session so the form fully resets between
            // sessions — react-hook-form's defaultValues memo doesn't pick
            // up "null → null" transitions otherwise.
            key={editingRule?.id ?? templateSeed?.id ?? "new"}
            rule={editingRule}
            initialValues={formInitialValues}
            onSaved={handleSaved}
            onCancel={closeDialog}
          />
        </DialogContent>
      </Dialog>

      {/* ── Preview sheet ──────────────────────────────────────────────── */}
      <Sheet open={previewOpen} onOpenChange={setPreviewOpen}>
        <SheetContent side="right" className="w-full sm:max-w-3xl">
          <SheetHeader>
            <SheetTitle>{t("rule:preview.section_title")}</SheetTitle>
          </SheetHeader>
          <div className="flex min-h-0 flex-1 flex-col px-6 pb-6">
            <RulePreviewPane />
          </div>
        </SheetContent>
      </Sheet>

      {/* ── Template picker dialog ─────────────────────────────────────── */}
      <RuleTemplatesDialog
        open={templateOpen}
        onOpenChange={setTemplateOpen}
        onTemplateSelect={(tpl) => {
          setTemplateOpen(false);
          setEditingRule(null);
          setTemplateSeed(tpl);
          setCreating(true);
        }}
      />
    </div>
  );
}
