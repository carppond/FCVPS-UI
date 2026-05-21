import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/cn";
import type { NodeProtocol } from "@/types/api";

/**
 * Visual badge for one of the 12 supported node protocols (M-NODE.1 §6.1).
 *
 * The component is intentionally a thin wrapper around the design-system
 * `Badge`; we keep colour mapping in a static lookup that uses the surface /
 * border tokens from globals.css so the badge respects the active theme
 * (dark-first + light). NO raw hex literals — see _dev-cheatsheet.md §编码硬约束.
 */
const PROTOCOL_VARIANTS: Record<NodeProtocol, string> = {
  vmess: "bg-[var(--color-info-bg)] text-[var(--color-info)]",
  vless: "bg-[var(--color-info-bg)] text-[var(--color-info)]",
  ss: "bg-[var(--color-success-bg)] text-[var(--color-success)]",
  ssr: "bg-[var(--color-success-bg)] text-[var(--color-success)]",
  trojan: "bg-[var(--color-warning-bg)] text-[var(--color-warning)]",
  hysteria: "bg-[var(--color-warning-bg)] text-[var(--color-warning)]",
  hysteria2: "bg-[var(--color-warning-bg)] text-[var(--color-warning)]",
  tuic: "bg-[var(--color-warning-bg)] text-[var(--color-warning)]",
  wireguard: "bg-[var(--color-error-bg)] text-[var(--color-error)]",
  anytls: "bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)]",
  socks5: "bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)]",
  naive: "bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)]",
};

interface ProtocolBadgeProps {
  protocol: NodeProtocol;
  className?: string;
}

export function ProtocolBadge({ protocol, className }: ProtocolBadgeProps) {
  const variant = PROTOCOL_VARIANTS[protocol] ?? PROTOCOL_VARIANTS.anytls;
  return (
    <Badge
      variant="outline"
      className={cn(
        "uppercase tracking-wide font-mono text-[var(--font-size-xs)]",
        variant,
        className,
      )}
    >
      {protocol}
    </Badge>
  );
}
