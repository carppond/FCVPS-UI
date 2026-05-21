import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/cn";
import type { AgentKind } from "@/types/api";

interface AgentKindBadgeProps {
  kind: AgentKind;
  className?: string;
}

/**
 * Localized kind badge. `native` is rendered with the accent (chart-1) so it
 * stands out as the recommended path, while `nezha_compat` uses a neutral
 * outline tone — same colour mapping as the create wizard.
 */
export function AgentKindBadge({ kind, className }: AgentKindBadgeProps) {
  const { t } = useTranslation("agent");
  const tone =
    kind === "native"
      ? "bg-[var(--color-info-bg)] text-[var(--color-info)] border-transparent"
      : "bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)] border-[var(--color-border-strong)]";
  return (
    <Badge variant="outline" className={cn(tone, className)}>
      {t(`kind.${kind}`)}
    </Badge>
  );
}
