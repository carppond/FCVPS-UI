import * as React from "react";
import { useTranslation } from "react-i18next";
import Editor, { type OnMount } from "@monaco-editor/react";
import { Copy, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { EmptyState } from "@/components/ui/empty-state";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { toast } from "@/components/ui/toast";
import { YamlDiffViewer } from "@/components/pipeline/yaml-diff-viewer";
import { useRulePreviewQuery } from "@/api/rule";
import { useSubscriptionsQuery } from "@/api/subscription";
import { useUIStore } from "@/stores/ui-store";
import { cn } from "@/lib/cn";

interface RulePreviewPaneProps {
  className?: string;
}

/**
 * Subscription picker + final Clash YAML preview. Designed to live inside
 * a Sheet (right-side drawer). The host Sheet supplies its own header /
 * close button, so this component only renders the toolbar + body.
 */
export function RulePreviewPane({ className }: RulePreviewPaneProps) {
  const { t } = useTranslation(["rule", "common"]);
  const themePref = useUIStore((s) => s.theme);
  const resolvedTheme = useResolvedTheme(themePref);

  const [subscriptionId, setSubscriptionId] = React.useState<string | null>(
    null,
  );

  const subsQuery = useSubscriptionsQuery({ page: 1, pageSize: 100 });
  const subs = React.useMemo(() => subsQuery.data?.items ?? [], [subsQuery.data]);

  // Auto-select the first subscription when the dropdown gets populated.
  React.useEffect(() => {
    if (!subscriptionId && subs.length > 0) {
      setSubscriptionId(subs[0].id);
    }
  }, [subs, subscriptionId]);

  const previewQuery = useRulePreviewQuery(subscriptionId ?? undefined);

  const [copied, setCopied] = React.useState(false);
  const handleCopy = async () => {
    if (!previewQuery.data?.final_yaml) return;
    try {
      await navigator.clipboard.writeText(previewQuery.data.final_yaml);
      setCopied(true);
      toast.success(t("rule:preview.copied"));
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // ignore — user can copy from the editor pane manually.
    }
  };

  return (
    <div
      data-testid="rule-preview-pane"
      className={cn(
        "flex min-h-0 flex-1 flex-col gap-3",
        className,
      )}
    >
      {/* ── Toolbar: subscription picker + actions ─────────────────────── */}
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <label
            htmlFor="rule-preview-sub"
            className="text-[var(--font-size-xs)] font-medium text-[var(--color-text-tertiary)]"
          >
            {t("rule:preview.subscription_label")}
          </label>
          <select
            id="rule-preview-sub"
            className={cn(
              "h-9 w-full rounded-[var(--radius-md)] border border-[var(--color-border-strong)]",
              "bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)]",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]",
            )}
            value={subscriptionId ?? ""}
            onChange={(e) => setSubscriptionId(e.target.value || null)}
            data-testid="rule-preview-sub-select"
          >
            <option value="">—</option>
            {subs.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </div>
        <div className="flex items-center gap-1">
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={() => previewQuery.refetch()}
            disabled={!subscriptionId || previewQuery.isFetching}
            aria-label={t("common:actions.retry")}
          >
            <RefreshCw className={cn("h-3.5 w-3.5", previewQuery.isFetching && "animate-spin")} />
          </Button>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={handleCopy}
            disabled={!previewQuery.data?.final_yaml}
          >
            <Copy className="h-3.5 w-3.5" />
            {copied ? t("rule:preview.copied") : t("rule:preview.copy")}
          </Button>
        </div>
      </div>

      {/* ── Body ───────────────────────────────────────────────────────── */}
      <div className="flex min-h-0 flex-1 flex-col">
        {!subscriptionId ? (
          <div className="flex flex-1 items-center justify-center">
            <EmptyState
              title={t("rule:preview.section_title")}
              description={t("rule:preview.no_subscription")}
            />
          </div>
        ) : previewQuery.isLoading ? (
          <div className="flex flex-1 flex-col gap-2" aria-hidden>
            <Skeleton className="h-6 w-32" />
            <Skeleton className="h-full w-full" />
          </div>
        ) : previewQuery.isError ? (
          <ErrorState
            message={
              previewQuery.error?.message ?? t("rule:error.preview_failed")
            }
            onRetry={() => void previewQuery.refetch()}
            retryLabel={t("common:actions.retry")}
          />
        ) : previewQuery.data ? (
          <div className="flex min-h-0 flex-1 flex-col gap-3">
            <Tabs defaultValue="after" className="flex min-h-0 flex-1 flex-col">
              <TabsList>
                <TabsTrigger value="after">
                  {t("rule:preview.after_label")}
                </TabsTrigger>
                <TabsTrigger value="before">
                  {t("rule:preview.before_label")}
                </TabsTrigger>
                <TabsTrigger value="diff">
                  {t("rule:preview.diff_label")}
                </TabsTrigger>
              </TabsList>
              <TabsContent value="after" className="min-h-0 flex-1 overflow-hidden rounded-[var(--radius-md)] border border-[var(--color-border)]">
                <YamlMonaco
                  value={previewQuery.data.final_yaml}
                  theme={resolvedTheme === "dark" ? "vs-dark" : "vs"}
                />
              </TabsContent>
              <TabsContent value="before" className="min-h-0 flex-1 overflow-hidden rounded-[var(--radius-md)] border border-[var(--color-border)]">
                <YamlMonaco
                  value={previewQuery.data.base_yaml}
                  theme={resolvedTheme === "dark" ? "vs-dark" : "vs"}
                />
              </TabsContent>
              <TabsContent value="diff" className="min-h-0 flex-1 overflow-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
                <YamlDiffViewer
                  left={previewQuery.data.base_yaml}
                  right={previewQuery.data.final_yaml}
                />
              </TabsContent>
            </Tabs>
            <footer className="flex items-center justify-between text-[var(--font-size-xs)] text-[var(--color-text-tertiary)] tabular-nums">
              <span>
                {t("rule:preview.rule_count", {
                  count: previewQuery.data.rule_count,
                })}
              </span>
              <span>
                {previewQuery.data.final_yaml.split("\n").length} lines · {previewQuery.data.final_yaml.length} chars
              </span>
            </footer>
          </div>
        ) : null}
      </div>
    </div>
  );
}

interface YamlMonacoProps {
  value: string;
  theme: "vs" | "vs-dark";
}

function YamlMonaco({ value, theme }: YamlMonacoProps) {
  const handleMount: OnMount = (editor) => {
    editor.updateOptions({
      readOnly: true,
      minimap: { enabled: false },
      lineNumbers: "on",
      scrollBeyondLastLine: false,
      fontSize: 12,
      tabSize: 2,
      wordWrap: "on",
      automaticLayout: true,
    });
  };
  return (
    <Editor
      height="100%"
      language="yaml"
      value={value}
      theme={theme}
      onMount={handleMount}
      options={{ readOnly: true, automaticLayout: true }}
    />
  );
}

function useResolvedTheme(pref: "light" | "dark" | "system"): "light" | "dark" {
  const [resolved, setResolved] = React.useState<"light" | "dark">(() =>
    readDocTheme(),
  );
  React.useEffect(() => {
    if (pref !== "system") {
      setResolved(pref);
      return;
    }
    setResolved(readDocTheme());
    const observer = new MutationObserver(() => setResolved(readDocTheme()));
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    return () => observer.disconnect();
  }, [pref]);
  return resolved;
}

function readDocTheme(): "light" | "dark" {
  if (typeof document === "undefined") return "dark";
  return document.documentElement.getAttribute("data-theme") === "light"
    ? "light"
    : "dark";
}
