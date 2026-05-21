import { useTranslation } from "react-i18next";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { EmptyState } from "@/components/ui/empty-state";
import { useRuleTemplatesQuery } from "@/api/rule";
import type { RuleTemplate } from "@/types/api";

interface RuleTemplatesDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /**
   * Invoked when the user clicks "use" on a template card. Parent should
   * close the dialog and seed the form with the supplied template.
   */
  onTemplateSelect: (template: RuleTemplate) => void;
}

/**
 * Built-in template picker. Renders a dialog with 3 preset cards
 * (国内直连 / 全局代理 / 广告屏蔽) returned by GET /api/rules/templates.
 *
 * The dialog stays a thin shell — the parent route owns "what to do" with the
 * chosen template (typically: populate the form's content field and switch
 * type to rules / rule-providers as appropriate).
 */
export function RuleTemplatesDialog({
  open,
  onOpenChange,
  onTemplateSelect,
}: RuleTemplatesDialogProps) {
  const { t } = useTranslation(["rule", "common"]);
  const { data, isLoading, isError, error, refetch } = useRuleTemplatesQuery();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("rule:templates.dialog_title")}</DialogTitle>
          <DialogDescription>
            {t("rule:templates.dialog_description")}
          </DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <TemplateListSkeleton />
        ) : isError ? (
          <ErrorState
            message={error?.message ?? t("rule:error.load_failed")}
            onRetry={() => void refetch()}
            retryLabel={t("common:actions.retry")}
          />
        ) : !data || data.length === 0 ? (
          <EmptyState
            title={t("rule:list.empty_title")}
            description={t("rule:list.empty_description")}
          />
        ) : (
          <ul className="flex flex-col gap-3" data-testid="rule-templates-list">
            {data.map((tpl) => (
              <li key={tpl.id}>
                <TemplateCard
                  template={tpl}
                  onSelect={() => onTemplateSelect(tpl)}
                />
              </li>
            ))}
          </ul>
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
      className="flex items-start justify-between gap-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4"
    >
      <div className="flex-1">
        <h3 className="text-[var(--font-size-base)] font-medium text-[var(--color-text-primary)]">
          {template.name}
        </h3>
        <p className="mt-1 text-[var(--font-size-xs)] leading-relaxed text-[var(--color-text-tertiary)]">
          {template.description}
        </p>
      </div>
      <Button size="sm" variant="default" onClick={onSelect}>
        {t("rule:templates.select")}
      </Button>
    </article>
  );
}

function TemplateListSkeleton() {
  return (
    <ul className="flex flex-col gap-3" aria-hidden>
      {Array.from({ length: 3 }).map((_, i) => (
        <li
          key={i}
          className="flex items-start justify-between gap-4 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
        >
          <div className="flex-1 space-y-2">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-3 w-64" />
          </div>
          <Skeleton className="h-8 w-16" />
        </li>
      ))}
    </ul>
  );
}
