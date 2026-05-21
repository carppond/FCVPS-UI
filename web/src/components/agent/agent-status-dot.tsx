import { useTranslation } from "react-i18next";
import { cn } from "@/lib/cn";
import type { AgentStatus } from "@/types/api";

interface AgentStatusDotProps {
  status: AgentStatus;
  /** When true, render the localized status text alongside the dot. */
  withLabel?: boolean;
  className?: string;
}

const TONE: Record<AgentStatus, string> = {
  online: "bg-[var(--color-status-online)] text-[var(--color-status-online)]",
  offline: "bg-[var(--color-status-offline)] text-[var(--color-status-offline)]",
  degraded:
    "bg-[var(--color-status-degraded)] text-[var(--color-status-degraded)]",
};

/**
 * Renders the agent's connectivity state as a small coloured dot, optionally
 * accompanied by the localized label. The status palette comes from the
 * `--color-status-*` design tokens (see _dev-cheatsheet.md) so the colour is
 * consistent with the node table.
 */
export function AgentStatusDot({
  status,
  withLabel,
  className,
}: AgentStatusDotProps) {
  const { t } = useTranslation("agent");
  const tone = TONE[status] ?? TONE.offline;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-2 text-[var(--font-size-sm)]",
        className,
      )}
    >
      <span
        aria-hidden
        className={cn("h-2 w-2 shrink-0 rounded-full", tone.split(" ")[0])}
      />
      {withLabel && (
        <span className={cn("font-medium", tone.split(" ")[1])}>
          {t(`status.${status}`)}
        </span>
      )}
    </span>
  );
}
