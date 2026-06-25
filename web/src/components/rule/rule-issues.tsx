import { useTranslation } from "react-i18next";
import { AlertTriangle } from "lucide-react";
import { useProxyGroups } from "@/api/proxy-group";
import type { RuleLineIssue } from "@/types/api";

/**
 * Renders the per-line validation issues returned by the rule API on save
 * (400). Each issue shows the offending line, why it is wrong, and a corrected
 * example; when proxy groups exist, their names are listed so the user can pick
 * a real policy to drop into a `<策略组>` placeholder.
 */
export function RuleIssues({ issues }: { issues: RuleLineIssue[] }) {
  const { t } = useTranslation(["rule", "common"]);
  const { data: groups } = useProxyGroups();
  if (issues.length === 0) return null;
  const groupNames = (groups ?? []).map((g) => g.name).filter(Boolean);
  return (
    <div className="flex flex-col gap-2 rounded-[var(--radius-md)] border border-[var(--color-error)] bg-[var(--color-error-bg)] p-3">
      <div className="flex items-center gap-1.5 text-[var(--font-size-sm)] font-medium text-[var(--color-error)]">
        <AlertTriangle className="size-4" />
        {t("rule:issues.title", { count: issues.length })}
      </div>
      <ul className="flex flex-col gap-2">
        {issues.map((issue) => (
          <li
            key={issue.line}
            className="text-[var(--font-size-xs)] text-[var(--color-text-secondary)]"
          >
            <span className="text-[var(--color-text-primary)]">
              {t("rule:issues.line", { line: issue.line })}
            </span>{" "}
            <code className="rounded bg-[var(--color-surface-hover)] px-1">
              {issue.text}
            </code>{" "}
            — {issue.reason}
            {issue.suggestion && (
              <div className="mt-0.5">
                {t("rule:issues.suggest")}:{" "}
                <code className="rounded bg-[var(--color-surface-hover)] px-1 text-[var(--color-text-primary)]">
                  {issue.suggestion}
                </code>
              </div>
            )}
          </li>
        ))}
      </ul>
      {groupNames.length > 0 && (
        <div className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("rule:issues.available_groups")}: {groupNames.join(" · ")}
        </div>
      )}
    </div>
  );
}
