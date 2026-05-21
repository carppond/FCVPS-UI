import * as React from "react";
import { useTranslation } from "react-i18next";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ErrorState } from "@/components/ui/error-state";
import { EmptyState } from "@/components/ui/empty-state";
import { cn } from "@/lib/cn";
import { useRuleTemplatesQuery } from "@/api/rule";
import type { RuleTemplate } from "@/types/api";

interface RuleTemplatesDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onTemplateSelect: (template: RuleTemplate) => void;
}

type TemplateCategory = "common" | "region" | "app" | "block";

const CATEGORY_ORDER: TemplateCategory[] = ["common", "region", "app", "block"];

/** Buckets a template into one of the four tabs. Unknown categories fall back
 *  to "common" so older templates still render somewhere visible. */
function categoryFor(t: RuleTemplate): TemplateCategory {
  const c = t.category;
  if (c === "region" || c === "app" || c === "block" || c === "common") {
    return c;
  }
  return "common";
}

/**
 * Categorised template picker — backend now ships 18 templates spread across
 * common/region/app/block. We split them into tabs so the dialog stays
 * scannable; the "use" button forwards the chosen template to the host.
 */
export function RuleTemplatesDialog({
  open,
  onOpenChange,
  onTemplateSelect,
}: RuleTemplatesDialogProps) {
  const { t } = useTranslation(["rule", "common"]);
  const { data, isLoading, isError, error, refetch } = useRuleTemplatesQuery();

  const [activeTab, setActiveTab] = React.useState<TemplateCategory>("common");
  React.useEffect(() => {
    if (open) setActiveTab("common");
  }, [open]);

  const byCategory = React.useMemo(() => {
    const acc: Record<TemplateCategory, RuleTemplate[]> = {
      common: [],
      region: [],
      app: [],
      block: [],
    };
    for (const tpl of data ?? []) acc[categoryFor(tpl)].push(tpl);
    return acc;
  }, [data]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>{t("rule:templates.dialog_title")}</DialogTitle>
          <DialogDescription>
            {t("rule:templates.dialog_description")}
          </DialogDescription>
        </DialogHeader>

        {isError ? (
          <ErrorState
            message={error?.message ?? t("rule:error.load_failed")}
            onRetry={() => void refetch()}
            retryLabel={t("common:actions.retry")}
          />
        ) : (
          <Tabs
            value={activeTab}
            onValueChange={(v) => setActiveTab(v as TemplateCategory)}
          >
            <TabsList className="w-full">
              {CATEGORY_ORDER.map((key) => (
                <TabsTrigger key={key} value={key} className="flex-1">
                  {t(`rule:templates.tabs.${key}`)}
                </TabsTrigger>
              ))}
            </TabsList>

            {CATEGORY_ORDER.map((key) => (
              <TabsContent
                key={key}
                value={key}
                className="max-h-[60vh] overflow-y-auto"
              >
                {isLoading ? (
                  <TemplateListSkeleton />
                ) : byCategory[key].length === 0 ? (
                  <EmptyState
                    title={t("rule:list.empty_title")}
                    description={t("rule:list.empty_description")}
                  />
                ) : (
                  <ul
                    className="grid grid-cols-1 gap-2 py-2 sm:grid-cols-2"
                    data-testid="rule-templates-list"
                  >
                    {byCategory[key].map((tpl) => (
                      <li key={tpl.id}>
                        <TemplateCard
                          template={tpl}
                          onSelect={() => onTemplateSelect(tpl)}
                        />
                      </li>
                    ))}
                  </ul>
                )}
              </TabsContent>
            ))}
          </Tabs>
        )}
      </DialogContent>
    </Dialog>
  );
}

interface TemplateCardProps {
  template: RuleTemplate;
  onSelect: () => void;
}

function TemplateCard({ template, onSelect }: TemplateCardProps) {
  const { t } = useTranslation(["rule"]);
  return (
    <article
      data-testid={`rule-template-${template.id}`}
      className={cn(
        "flex h-full flex-col gap-3 rounded-[var(--radius-md)]",
        "border border-[var(--color-border)] bg-[var(--color-surface)] p-4",
        "transition-colors duration-[var(--duration-fast)] hover:bg-[var(--color-surface-hover)]",
      )}
    >
      <div className="flex items-start gap-3">
        <span
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--radius-md)] bg-[var(--color-surface-hover)] text-base"
          aria-hidden
        >
          {template.emoji ?? "·"}
        </span>
        <div className="flex-1 min-w-0">
          <h3 className="text-[var(--font-size-base)] font-medium text-[var(--color-text-primary)]">
            {template.name}
          </h3>
          <p className="mt-1 text-[var(--font-size-xs)] leading-relaxed text-[var(--color-text-tertiary)]">
            {template.description}
          </p>
        </div>
      </div>
      {template.tags && template.tags.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {template.tags.map((tag) => (
            <Badge key={tag} variant="outline">
              {tag}
            </Badge>
          ))}
        </div>
      )}
      <div className="mt-auto flex justify-end">
        <Button size="sm" variant="default" onClick={onSelect}>
          {t("rule:templates.select")}
        </Button>
      </div>
    </article>
  );
}

function TemplateListSkeleton() {
  return (
    <div className="grid grid-cols-1 gap-2 py-2 sm:grid-cols-2" aria-hidden>
      {Array.from({ length: 6 }).map((_, i) => (
        <div
          key={i}
          className="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
        >
          <div className="flex items-start gap-3">
            <Skeleton className="h-9 w-9 rounded-[var(--radius-md)]" />
            <div className="flex flex-1 flex-col gap-2">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-3 w-56" />
            </div>
          </div>
          <Skeleton className="ml-auto h-8 w-16" />
        </div>
      ))}
    </div>
  );
}
