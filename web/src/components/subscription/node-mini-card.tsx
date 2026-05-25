import { cn } from "@/lib/cn";
import type { NodeWithLatency, NodeProtocol } from "@/types/api";

// ---------------------------------------------------------------------------
// Latency colour helpers (matches preview V4 spec)
// ---------------------------------------------------------------------------

function latencyClass(ms: number): string {
  if (ms < 0) return "bg-[var(--color-surface-hover)] text-[var(--color-text-tertiary)]"; // unreachable / grey
  if (ms < 100) return "bg-[var(--color-success-bg)] text-[var(--color-success)]";
  if (ms < 300) return "bg-[var(--color-warning-bg)] text-[var(--color-warning)]";
  return "bg-[var(--color-error-bg)] text-[var(--color-error)]";
}

function latencyLabel(ms: number | undefined | null): string {
  if (ms === null || ms === undefined) return "--";
  if (ms < 0) return "timeout";
  return `${ms} ms`;
}

// ---------------------------------------------------------------------------
// Protocol badge colour (inline, smaller than the dedicated ProtocolBadge)
// ---------------------------------------------------------------------------

const PROTO_COLORS: Partial<Record<NodeProtocol, string>> = {
  vmess: "bg-[var(--color-info-bg)] text-[var(--color-info)]",
  vless: "bg-[var(--color-info-bg)] text-[var(--color-info)]",
  ss: "bg-[var(--color-success-bg)] text-[var(--color-success)]",
  ssr: "bg-[var(--color-success-bg)] text-[var(--color-success)]",
  trojan: "bg-[var(--color-warning-bg)] text-[var(--color-warning)]",
  hysteria: "bg-[var(--color-warning-bg)] text-[var(--color-warning)]",
  hysteria2: "bg-[var(--color-warning-bg)] text-[var(--color-warning)]",
  tuic: "bg-[var(--color-warning-bg)] text-[var(--color-warning)]",
  wireguard: "bg-[var(--color-error-bg)] text-[var(--color-error)]",
};

const PROTO_DEFAULT = "bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)]";

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface NodeMiniCardProps {
  node: NodeWithLatency;
  className?: string;
  onClick?: () => void;
}

/**
 * Compact node card for the V4 subscription detail page.
 *
 * Layout (matches style-preview-sub-detail.html `.nd` class):
 *  - Top row: online dot + name + protocol badge
 *  - Middle row: server:port (mono, 10px)
 *  - Bottom row: tags chips + latency badge
 */
export function NodeMiniCard({ node, className, onClick }: NodeMiniCardProps) {
  const online = node.reachable || (node.latency_ms != null && node.latency_ms >= 0);

  return (
    <div
      className={cn(
        // Base card — matches `.nd` from preview
        "flex cursor-pointer flex-col gap-1 rounded-lg border p-2 px-2.5",
        "border-[var(--color-border)] bg-[var(--color-surface-hover)]",
        "transition-all duration-100",
        "hover:border-[var(--color-border-strong)] hover:bg-[var(--color-bg-elevated)]",
        className,
      )}
      onClick={onClick}
    >
      {/* Top row: dot + name + protocol */}
      <div className="flex items-center justify-between gap-1.5">
        <div className="flex min-w-0 items-center gap-1.5">
          <span
            className={cn(
              "inline-block h-1.5 w-1.5 flex-shrink-0 rounded-full",
              online
                ? "bg-[var(--color-success)] shadow-[0_0_6px_var(--color-success)]"
                : "bg-[var(--color-text-disabled)]",
            )}
          />
          <span
            className={cn(
              "truncate text-[11.5px] font-semibold",
              online
                ? "text-[var(--color-text-primary)]"
                : "text-[var(--color-text-tertiary)]",
            )}
          >
            {node.tag || node.server}
          </span>
        </div>
        <span
          className={cn(
            "flex-shrink-0 rounded px-1.5 py-px font-mono text-[10px] font-semibold uppercase",
            PROTO_COLORS[node.protocol] ?? PROTO_DEFAULT,
          )}
        >
          {node.protocol}
        </span>
      </div>

      {/* Address row */}
      <span className="truncate font-mono text-[10px] text-[var(--color-text-tertiary)]">
        {node.server}:{node.port}
      </span>

      {/* Bottom row: tags + latency */}
      <div className="flex items-center justify-between gap-1">
        <div className="flex min-w-0 flex-wrap gap-1">
          {node.tags?.slice(0, 3).map((tag) => (
            <span
              key={tag}
              className="rounded bg-[var(--color-surface-hover)] px-1.5 py-px text-[10px] font-medium text-[var(--color-text-tertiary)]"
            >
              {tag}
            </span>
          ))}
        </div>
        {node.latency_ms != null && (
          <span
            className={cn(
              "flex-shrink-0 rounded px-1.5 py-px text-[10px] font-semibold tabular-nums",
              latencyClass(node.latency_ms),
            )}
          >
            {latencyLabel(node.latency_ms)}
          </span>
        )}
      </div>
    </div>
  );
}
