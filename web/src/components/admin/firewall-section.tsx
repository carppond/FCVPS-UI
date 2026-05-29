import * as React from "react";
import { useTranslation } from "react-i18next";
import {
  Shield,
  ShieldAlert,
  ShieldCheck,
  Lock,
  Trash2,
  Plus,
  Info,
  CircleSlash,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { toast } from "@/components/ui/toast";
import {
  useFirewallStatus,
  useAllowPort,
  useDeletePort,
} from "@/api/firewall";
import { useApiError } from "@/hooks/use-api-error";
import type { FirewallRule } from "@/types/api";

/**
 * Local-host firewall (ufw) management. Scoped to the machine the hub runs on
 * — NOT remote VPS assets. Degrades to a read-only / disabled state when ufw
 * is unavailable (container deploy, missing binary, no privilege). SSH and the
 * panel access port are shown 🔒 and cannot be deleted (self-lockout guard).
 */
export function FirewallSection() {
  const { t } = useTranslation(["settings", "common", "errors"]);
  const { data, isLoading, isError, refetch } = useFirewallStatus();
  const allow = useAllowPort();
  const del = useDeletePort();
  const { handle: handleError } = useApiError();

  const [port, setPort] = React.useState("");
  const [proto, setProto] = React.useState<"tcp" | "udp">("tcp");
  const [pendingDelete, setPendingDelete] = React.useState<FirewallRule | null>(
    null,
  );

  if (isLoading) {
    return (
      <div className="flex flex-1 flex-col gap-4 p-8">
        <Skeleton className="h-7 w-1/3" />
        <Skeleton className="h-24 w-full rounded-xl" />
        <Skeleton className="h-40 w-full rounded-xl" />
      </div>
    );
  }
  if (isError || !data) {
    return (
      <div className="flex flex-1 items-center justify-center p-8">
        <ErrorState
          message={t("settings:firewall.load_failed")}
          onRetry={() => refetch()}
          retryLabel={t("common:actions.retry")}
        />
      </div>
    );
  }

  const { status, rules, note } = data;

  const handleAllow = async () => {
    const n = Number(port);
    if (!Number.isInteger(n) || n < 1 || n > 65535) {
      toast.error(t("settings:firewall.invalid_port"));
      return;
    }
    try {
      await allow.mutateAsync({ port: n, proto });
      toast.success(t("settings:firewall.allow_success", { port: n, proto }));
      setPort("");
    } catch (err) {
      handleError(err);
    }
  };

  const confirmDelete = async () => {
    if (!pendingDelete) return;
    const r = pendingDelete;
    try {
      await del.mutateAsync({ port: r.port, proto: r.proto });
      toast.success(t("settings:firewall.delete_success", { port: r.port }));
      setPendingDelete(null);
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <section
      aria-labelledby="firewall-heading"
      className="flex flex-1 flex-col gap-6 p-8"
    >
      <header className="flex flex-col gap-1">
        <h2
          id="firewall-heading"
          className="flex items-center gap-2 text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]"
        >
          <Shield className="h-4 w-4" />
          {t("settings:firewall.title")}
        </h2>
        <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("settings:firewall.description")}
        </p>
      </header>

      <StatusBanner status={status} />

      {status.can_manage ? (
        <>
          {/* Add-port row */}
          <div className="flex flex-wrap items-end gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-3">
            <div className="flex flex-col gap-1">
              <label className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                {t("settings:firewall.port_label")}
              </label>
              <Input
                value={port}
                onChange={(e) => setPort(e.target.value.replace(/[^0-9]/g, ""))}
                inputMode="numeric"
                placeholder="8081"
                className="w-28"
                onKeyDown={(e) => e.key === "Enter" && handleAllow()}
              />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                {t("settings:firewall.proto_label")}
              </label>
              <select
                value={proto}
                onChange={(e) => setProto(e.target.value as "tcp" | "udp")}
                className="h-9 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
              >
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
              </select>
            </div>
            <Button onClick={handleAllow} disabled={allow.isPending || !port}>
              <Plus className="mr-1.5 h-4 w-4" />
              {allow.isPending
                ? t("settings:firewall.allow_pending")
                : t("settings:firewall.allow_button")}
            </Button>
          </div>

          {/* Rules table */}
          <RuleTable rules={rules} onDelete={setPendingDelete} t={t} />
        </>
      ) : null}

      {/* Cloud security-group reminder (always shown) */}
      <div className="flex items-start gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-3 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
        <Info className="mt-0.5 h-4 w-4 flex-shrink-0 text-[var(--color-info)]" />
        <span>{note}</span>
      </div>

      {/* Delete confirm dialog — surfaces the listening process */}
      <Dialog
        open={pendingDelete !== null}
        onOpenChange={(o) => !o && !del.isPending && setPendingDelete(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-[var(--color-error)]">
              <ShieldAlert className="h-5 w-5" />
              {t("settings:firewall.delete_confirm_title", {
                port: pendingDelete?.port,
                proto: pendingDelete?.proto || "tcp/udp",
              })}
            </DialogTitle>
            <DialogDescription>
              {pendingDelete?.process
                ? t("settings:firewall.delete_confirm_in_use", {
                    process: pendingDelete.process,
                    pid: pendingDelete.pid ?? "?",
                  })
                : t("settings:firewall.delete_confirm_idle")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setPendingDelete(null)}
              disabled={del.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={confirmDelete}
              disabled={del.isPending}
            >
              {del.isPending
                ? t("settings:firewall.delete_pending")
                : t("settings:firewall.delete_confirm_submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}

function StatusBanner({
  status,
}: {
  status: import("@/types/api").FirewallStatus;
}) {
  const { t } = useTranslation("settings");
  if (!status.can_manage) {
    return (
      <div className="flex items-start gap-2 rounded-[var(--radius-md)] border border-[var(--color-warning)] bg-[var(--color-warning-bg,rgba(251,191,36,.08))] p-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
        <CircleSlash className="mt-0.5 h-4 w-4 flex-shrink-0 text-[var(--color-warning)]" />
        <div className="flex flex-col gap-0.5">
          <span className="font-medium">{t("firewall.unavailable")}</span>
          {status.reason && (
            <span className="text-[var(--color-text-tertiary)]">
              {status.reason}
            </span>
          )}
        </div>
      </div>
    );
  }
  const active = status.active;
  return (
    <div
      className="flex items-center gap-2 rounded-[var(--radius-md)] border p-3 text-[var(--font-size-sm)]"
      style={{
        borderColor: active ? "var(--color-success)" : "var(--color-border)",
      }}
    >
      {active ? (
        <ShieldCheck className="h-4 w-4 text-[var(--color-success)]" />
      ) : (
        <Shield className="h-4 w-4 text-[var(--color-text-tertiary)]" />
      )}
      <span className="text-[var(--color-text-primary)]">
        {active ? t("firewall.state_active") : t("firewall.state_inactive")}
      </span>
      {!active && (
        <span className="text-[var(--color-text-tertiary)]">
          {t("firewall.state_inactive_hint")}
        </span>
      )}
    </div>
  );
}

function RuleTable({
  rules,
  onDelete,
  t,
}: {
  rules: FirewallRule[];
  onDelete: (r: FirewallRule) => void;
  t: (k: string, o?: Record<string, unknown>) => string;
}) {
  if (rules.length === 0) {
    return (
      <p className="px-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
        {t("settings:firewall.empty")}
      </p>
    );
  }
  return (
    <div className="overflow-hidden rounded-[var(--radius-md)] border border-[var(--color-border)]">
      <table className="w-full text-[var(--font-size-sm)]">
        <thead>
          <tr className="border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-left text-[var(--color-text-tertiary)]">
            <th className="px-3 py-2 font-medium">
              {t("settings:firewall.col_port")}
            </th>
            <th className="px-3 py-2 font-medium">
              {t("settings:firewall.col_proto")}
            </th>
            <th className="px-3 py-2 font-medium">
              {t("settings:firewall.col_process")}
            </th>
            <th className="px-3 py-2 text-right font-medium">
              {t("settings:firewall.col_action")}
            </th>
          </tr>
        </thead>
        <tbody>
          {rules.map((r) => (
            <tr
              key={r.spec}
              className="border-b border-[var(--color-border)] last:border-0 text-[var(--color-text-primary)]"
            >
              <td className="px-3 py-2 font-mono">{r.port || r.spec}</td>
              <td className="px-3 py-2 uppercase text-[var(--color-text-secondary)]">
                {r.proto || "tcp/udp"}
              </td>
              <td className="px-3 py-2 text-[var(--color-text-secondary)]">
                {r.process
                  ? `${r.process}${r.pid ? ` (pid ${r.pid})` : ""}`
                  : t("settings:firewall.no_listener")}
              </td>
              <td className="px-3 py-2 text-right">
                {r.protected ? (
                  <span className="inline-flex items-center gap-1 text-[var(--color-text-tertiary)]">
                    <Lock className="h-3.5 w-3.5" />
                    {t("settings:firewall.protected")}
                  </span>
                ) : r.port > 0 ? (
                  <button
                    type="button"
                    onClick={() => onDelete(r)}
                    className="inline-flex items-center gap-1 text-[var(--color-error)] hover:underline"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                    {t("common:actions.delete")}
                  </button>
                ) : (
                  <span className="text-[var(--color-text-disabled)]">—</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
