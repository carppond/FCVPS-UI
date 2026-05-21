import * as React from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/cn";

export interface YamlDiffViewerProps {
  left: string;
  right: string;
  className?: string;
}

type DiffOp = "context" | "add" | "remove";
interface DiffLine {
  op: DiffOp;
  text: string;
  leftNo: number | null;
  rightNo: number | null;
}

/**
 * Simple unified-diff viewer for YAML / text payloads. Used by the rule
 * preview screen (T-12) to show "before vs after" snippets; reused here so
 * the editor / rule modules share a single visual idiom.
 *
 * We use a line-by-line greedy LCS — sufficient for short YAML files and
 * avoids pulling in a dedicated diff dependency. For larger inputs the
 * caller should prefer a streaming side-by-side viewer (out of scope for v1).
 */
export function YamlDiffViewer({ left, right, className }: YamlDiffViewerProps) {
  const { t } = useTranslation(["pipeline"]);
  const lines = React.useMemo(() => computeDiff(left, right), [left, right]);

  const isEmpty = lines.every((l) => l.op === "context");
  if (isEmpty && left === right) {
    return (
      <div
        data-testid="yaml-diff-viewer-empty"
        className={cn(
          "flex items-center justify-center rounded-[var(--radius-md)] border border-[var(--color-border)] p-6",
          "text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]",
          className,
        )}
      >
        {t("pipeline:diff.empty")}
      </div>
    );
  }

  return (
    <pre
      data-testid="yaml-diff-viewer"
      className={cn(
        "max-h-[var(--spacing-96,32rem)] overflow-auto rounded-[var(--radius-md)] border border-[var(--color-border)]",
        "bg-[var(--color-bg-elevated)] p-3 font-mono text-[var(--font-size-xs)] leading-relaxed",
        className,
      )}
    >
      {lines.map((line, idx) => (
        <DiffRow key={idx} line={line} />
      ))}
    </pre>
  );
}

function DiffRow({ line }: { line: DiffLine }) {
  const { t } = useTranslation(["pipeline"]);
  const tone =
    line.op === "add"
      ? "bg-[var(--color-success-bg)] text-[var(--color-success)]"
      : line.op === "remove"
        ? "bg-[var(--color-error-bg)] text-[var(--color-error)]"
        : "text-[var(--color-text-secondary)]";

  const prefix =
    line.op === "add"
      ? t("pipeline:diff.added_prefix")
      : line.op === "remove"
        ? t("pipeline:diff.removed_prefix")
        : " ";

  return (
    <div
      data-testid={`diff-row-${line.op}`}
      className={cn("flex gap-2 px-1", tone)}
    >
      <span
        aria-hidden
        className="w-8 select-none text-right text-[var(--color-text-tertiary)] tabular-nums"
      >
        {line.leftNo ?? ""}
      </span>
      <span
        aria-hidden
        className="w-8 select-none text-right text-[var(--color-text-tertiary)] tabular-nums"
      >
        {line.rightNo ?? ""}
      </span>
      <span className="w-3 select-none">{prefix}</span>
      <span className="whitespace-pre-wrap break-all">{line.text}</span>
    </div>
  );
}

/**
 * Minimal LCS-based line diff. Returns a unified-style ordered list:
 *   context lines have both line numbers; adds have only `rightNo`; removes
 *   only `leftNo`. Implementation is O(n×m) which is fine for YAML payloads.
 */
export function computeDiff(left: string, right: string): DiffLine[] {
  const a = left.split("\n");
  const b = right.split("\n");
  const n = a.length;
  const m = b.length;

  // dp[i][j] = LCS length of a[i..] and b[j..]. Walking from the end keeps
  // the back-trace step trivial.
  const dp: number[][] = Array.from({ length: n + 1 }, () =>
    new Array<number>(m + 1).fill(0),
  );
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      if (a[i] === b[j]) dp[i][j] = dp[i + 1][j + 1] + 1;
      else dp[i][j] = Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }

  const out: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      out.push({
        op: "context",
        text: a[i],
        leftNo: i + 1,
        rightNo: j + 1,
      });
      i++;
      j++;
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      out.push({ op: "remove", text: a[i], leftNo: i + 1, rightNo: null });
      i++;
    } else {
      out.push({ op: "add", text: b[j], leftNo: null, rightNo: j + 1 });
      j++;
    }
  }
  while (i < n) {
    out.push({ op: "remove", text: a[i], leftNo: i + 1, rightNo: null });
    i++;
  }
  while (j < m) {
    out.push({ op: "add", text: b[j], leftNo: null, rightNo: j + 1 });
    j++;
  }
  return out;
}
