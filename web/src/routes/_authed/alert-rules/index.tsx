import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Plus, Pencil, Trash2, BellRing } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import {
  useAlertRulesQuery,
  useCreateAlertRuleMutation,
  useUpdateAlertRuleMutation,
  useDeleteAlertRuleMutation,
} from "@/api/alert-rule";
import { useAgentsQuery } from "@/api/agent";
import { useApiError } from "@/hooks/use-api-error";
import i18n from "@/lib/i18n";
import arZh from "@/locales/zh-CN/alert-rule.json";
import arEn from "@/locales/en/alert-rule.json";
import arJa from "@/locales/ja/alert-rule.json";
import arKo from "@/locales/ko/alert-rule.json";
import type { AlertMetric, AlertRule } from "@/types/api";

function ensureNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "alert-rule")) {
    i18n.addResourceBundle("zh-CN", "alert-rule", arZh, true, true);
    i18n.addResourceBundle("en", "alert-rule", arEn, true, true);
    i18n.addResourceBundle("ja", "alert-rule", arJa, true, true);
    i18n.addResourceBundle("ko", "alert-rule", arKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/alert-rules/")({
  beforeLoad: () => ensureNamespace(),
  component: AlertRulesPage,
});

const METRICS: AlertMetric[] = ["cpu", "mem", "disk", "offline"];

interface FormState {
  name: string;
  agentId: string; // "" = all agents
  metric: AlertMetric;
  threshold: string;
  durationSec: string;
  cooldownSec: string;
  enabled: boolean;
}

function emptyForm(): FormState {
  return {
    name: "",
    agentId: "",
    metric: "cpu",
    threshold: "80",
    durationSec: "0",
    cooldownSec: "3600",
    enabled: true,
  };
}

function AlertRulesPage() {
  const { t } = useTranslation(["alert-rule", "common"]);
  const { handle: handleError } = useApiError();

  const rulesQ = useAlertRulesQuery();
  const agentsQ = useAgentsQuery({ pageSize: 1000 });
  const createMutation = useCreateAlertRuleMutation();
  const updateMutation = useUpdateAlertRuleMutation();
  const deleteMutation = useDeleteAlertRuleMutation();

  const rules = React.useMemo(() => rulesQ.data?.items ?? [], [rulesQ.data]);
  const agents = React.useMemo(() => agentsQ.data?.items ?? [], [agentsQ.data]);
  const agentName = React.useCallback(
    (id: string) => agents.find((a) => a.id === id)?.name ?? id,
    [agents],
  );

  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<AlertRule | null>(null);
  const [form, setForm] = React.useState<FormState>(emptyForm);
  const [deleteTarget, setDeleteTarget] = React.useState<AlertRule | null>(null);

  const metricLabel = React.useCallback(
    (m: AlertMetric) => t(`metric_${m}`),
    [t],
  );

  function openCreate() {
    setEditing(null);
    setForm(emptyForm());
    setDialogOpen(true);
  }

  function openEdit(rule: AlertRule) {
    setEditing(rule);
    setForm({
      name: rule.name,
      agentId: rule.agent_id ?? "",
      metric: rule.metric,
      threshold: String(rule.threshold),
      durationSec: String(rule.duration_sec),
      cooldownSec: String(rule.cooldown_sec),
      enabled: rule.enabled,
    });
    setDialogOpen(true);
  }

  async function handleSave() {
    const payload = {
      name: form.name.trim(),
      agent_id: form.agentId || undefined,
      metric: form.metric,
      threshold: form.metric === "offline" ? 0 : Number(form.threshold),
      duration_sec: Number(form.durationSec) || 0,
      cooldown_sec: Number(form.cooldownSec) || 3600,
      enabled: form.enabled,
    };
    try {
      if (editing) {
        await updateMutation.mutateAsync({
          id: editing.id,
          data: { ...payload, agent_id: form.agentId || null },
        });
      } else {
        await createMutation.mutateAsync(payload);
      }
      toast.success(t("saved"));
      setDialogOpen(false);
    } catch (err) {
      handleError(err);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    try {
      await deleteMutation.mutateAsync(deleteTarget.id);
      toast.success(t("deleted"));
      setDeleteTarget(null);
    } catch (err) {
      handleError(err);
    }
  }

  function conditionText(rule: AlertRule): string {
    const dur =
      rule.duration_sec > 0 ? `${Math.round(rule.duration_sec / 60)}m` : "";
    if (rule.metric === "offline") {
      return t("cond_offline", { duration: dur || "—" });
    }
    let s = t("cond_metric", {
      metric: metricLabel(rule.metric),
      threshold: rule.threshold,
    });
    if (dur) s += t("cond_sustained", { duration: dur });
    return s;
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-[var(--font-size-xl)] font-semibold text-[var(--color-text-primary)]">
            {t("title")}
          </h1>
          <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("subtitle")}
          </p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="mr-1 h-4 w-4" />
          {t("add")}
        </Button>
      </div>

      {rulesQ.isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      ) : rulesQ.isError ? (
        <ErrorState message={t("load_failed")} onRetry={() => rulesQ.refetch()} />
      ) : rules.length === 0 ? (
        <EmptyState
          icon={<BellRing className="h-8 w-8" />}
          title={t("empty_title")}
          description={t("empty_desc")}
          ctaLabel={t("add")}
          onCta={openCreate}
        />
      ) : (
        <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)]">
          <table className="w-full text-[var(--font-size-sm)]">
            <thead className="bg-[var(--color-surface)] text-[var(--color-text-tertiary)]">
              <tr>
                <th className="px-4 py-2 text-left font-medium">{t("form_name")}</th>
                <th className="px-4 py-2 text-left font-medium">{t("col_target")}</th>
                <th className="px-4 py-2 text-left font-medium">{t("col_condition")}</th>
                <th className="px-4 py-2 text-left font-medium">{t("col_status")}</th>
                <th className="px-4 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {rules.map((rule) => (
                <tr
                  key={rule.id}
                  className="border-t border-[var(--color-border)] text-[var(--color-text-secondary)]"
                >
                  <td className="px-4 py-3 text-[var(--color-text-primary)]">{rule.name}</td>
                  <td className="px-4 py-3">
                    {rule.agent_id ? agentName(rule.agent_id) : t("form_agent_all")}
                  </td>
                  <td className="px-4 py-3">{conditionText(rule)}</td>
                  <td className="px-4 py-3">
                    <Badge variant={rule.enabled ? "default" : "secondary"}>
                      {rule.enabled ? t("status_on") : t("status_off")}
                    </Badge>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex justify-end gap-1">
                      <Button variant="ghost" size="icon" onClick={() => openEdit(rule)}>
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="icon" onClick={() => setDeleteTarget(rule)}>
                        <Trash2 className="h-4 w-4 text-[var(--color-danger)]" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Create / edit dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editing ? t("edit") : t("create")}</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="ar-name">{t("form_name")}</Label>
              <Input
                id="ar-name"
                value={form.name}
                placeholder={t("form_name_ph")}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="ar-agent">{t("form_agent")}</Label>
              <select
                id="ar-agent"
                className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-bg)] px-2 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
                value={form.agentId}
                onChange={(e) => setForm({ ...form, agentId: e.target.value })}
              >
                <option value="">{t("form_agent_all")}</option>
                {agents.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="ar-metric">{t("form_metric")}</Label>
              <select
                id="ar-metric"
                className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-bg)] px-2 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
                value={form.metric}
                onChange={(e) => setForm({ ...form, metric: e.target.value as AlertMetric })}
              >
                {METRICS.map((m) => (
                  <option key={m} value={m}>
                    {metricLabel(m)}
                  </option>
                ))}
              </select>
            </div>
            {form.metric !== "offline" && (
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ar-threshold">{t("form_threshold")}</Label>
                <Input
                  id="ar-threshold"
                  type="number"
                  min={1}
                  max={100}
                  value={form.threshold}
                  onChange={(e) => setForm({ ...form, threshold: e.target.value })}
                />
              </div>
            )}
            <div className="grid grid-cols-2 gap-3">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ar-duration">{t("form_duration")}</Label>
                <Input
                  id="ar-duration"
                  type="number"
                  min={0}
                  value={form.durationSec}
                  onChange={(e) => setForm({ ...form, durationSec: e.target.value })}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ar-cooldown">{t("form_cooldown")}</Label>
                <Input
                  id="ar-cooldown"
                  type="number"
                  min={0}
                  value={form.cooldownSec}
                  onChange={(e) => setForm({ ...form, cooldownSec: e.target.value })}
                />
              </div>
            </div>
            <label className="flex items-center gap-2">
              <Checkbox
                checked={form.enabled}
                onCheckedChange={(c) => setForm({ ...form, enabled: c === true })}
              />
              <span className="text-[var(--font-size-sm)]">{t("form_enabled")}</span>
            </label>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDialogOpen(false)}>
              {t("cancel")}
            </Button>
            <Button
              onClick={handleSave}
              disabled={!form.name.trim() || createMutation.isPending || updateMutation.isPending}
            >
              {t("save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirm */}
      <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("delete_confirm_title")}</DialogTitle>
            <DialogDescription>{t("delete_confirm_desc")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDeleteTarget(null)}>
              {t("cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteMutation.isPending}
            >
              {t("common:delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
