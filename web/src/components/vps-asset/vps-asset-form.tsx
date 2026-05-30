import * as React from "react";
import { useTranslation } from "react-i18next";
import {
  Monitor,
  Clock,
  HardDrive,
  Lock,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { DatePicker } from "@/components/ui/date-picker";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";
import {
  useCreateVpsAssetMutation,
  useUpdateVpsAssetMutation,
} from "@/api/vps-asset";
import { useAgentsQuery } from "@/api/agent";
import type {
  BillingCycle,
  CreateVpsAssetRequest,
  VpsAsset,
} from "@/types/api";

interface Props {
  open: boolean;
  vps?: VpsAsset | null;
  onClose: () => void;
}

const BILLING_CYCLES: BillingCycle[] = [
  "monthly",
  "quarterly",
  "semi_annual",
  "annual",
  "biennial",
  "triennial",
];

const CURRENCIES = ["USD", "CNY", "EUR", "GBP", "JPY", "KRW"];

export function VpsAssetFormDialog({ open, vps, onClose }: Props) {
  const { t } = useTranslation(["vps-asset", "common"]);
  const { handle: handleError } = useApiError();
  const isEdit = !!vps;

  const createMutation = useCreateVpsAssetMutation();
  const updateMutation = useUpdateVpsAssetMutation();
  const { data: agentsPage } = useAgentsQuery();
  const agents = agentsPage?.items ?? [];

  const [name, setName] = React.useState("");
  const [provider, setProvider] = React.useState("");
  const [ip, setIp] = React.useState("");
  const [location, setLocation] = React.useState("");
  const [price, setPrice] = React.useState("");
  const [currency, setCurrency] = React.useState("USD");
  const [billingCycle, setBillingCycle] = React.useState<BillingCycle>("annual");
  const [expireAt, setExpireAt] = React.useState("");
  const [cpu, setCpu] = React.useState("");
  const [memory, setMemory] = React.useState("");
  const [disk, setDisk] = React.useState("");
  const [bandwidth, setBandwidth] = React.useState("");
  const [monthlyTraffic, setMonthlyTraffic] = React.useState("0");
  const [sshPort, setSshPort] = React.useState("22");
  const [sshUser, setSshUser] = React.useState("");
  const [os, setOs] = React.useState("");
  const [notes, setNotes] = React.useState("");
  const [agentId, setAgentId] = React.useState("");

  React.useEffect(() => {
    if (!open) return;
    if (vps) {
      setName(vps.name);
      setProvider(vps.provider);
      setIp(vps.ip ?? "");
      setLocation(vps.location ?? "");
      setPrice(String(vps.price));
      setCurrency(vps.currency);
      setBillingCycle(vps.billing_cycle);
      setExpireAt(vps.expire_at.split("T")[0]);
      setCpu(vps.cpu ?? "");
      setMemory(vps.memory ?? "");
      setDisk(vps.disk ?? "");
      setBandwidth(vps.bandwidth ?? "");
      setMonthlyTraffic(String(vps.monthly_traffic));
      setSshPort(String(vps.ssh_port));
      setSshUser(vps.ssh_user ?? "");
      setOs(vps.os ?? "");
      setNotes(vps.notes ?? "");
      setAgentId(vps.agent_id ?? "");
    } else {
      setName(""); setProvider(""); setIp(""); setLocation("");
      setPrice(""); setCurrency("USD"); setBillingCycle("annual");
      setExpireAt(""); setCpu(""); setMemory(""); setDisk("");
      setBandwidth(""); setMonthlyTraffic("0"); setSshPort("22");
      setSshUser(""); setOs(""); setNotes(""); setAgentId("");
    }
  }, [open, vps]);

  const isPending = createMutation.isPending || updateMutation.isPending;

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (isEdit && vps) {
        await updateMutation.mutateAsync({
          id: vps.id,
          data: {
            name, provider,
            ip: ip || null, location: location || null,
            price: Number(price), currency, billing_cycle: billingCycle,
            expire_at: expireAt,
            ssh_port: Number(sshPort), ssh_user: sshUser || null, os: os || null,
            cpu: cpu || null, memory: memory || null, disk: disk || null,
            bandwidth: bandwidth || null, monthly_traffic: Number(monthlyTraffic),
            notes: notes || null, agent_id: agentId || null,
          },
        });
        toast.success(t("vps-asset:toast.updated"));
      } else {
        const req: CreateVpsAssetRequest = {
          name, provider,
          price: Number(price), billing_cycle: billingCycle,
          expire_at: expireAt, currency,
        };
        if (ip) req.ip = ip;
        if (location) req.location = location;
        if (sshPort !== "22") req.ssh_port = Number(sshPort);
        if (sshUser) req.ssh_user = sshUser;
        if (os) req.os = os;
        if (cpu) req.cpu = cpu;
        if (memory) req.memory = memory;
        if (disk) req.disk = disk;
        if (bandwidth) req.bandwidth = bandwidth;
        if (monthlyTraffic !== "0") req.monthly_traffic = Number(monthlyTraffic);
        if (notes) req.notes = notes;
        if (agentId) req.agent_id = agentId;
        await createMutation.mutateAsync(req);
        toast.success(t("vps-asset:toast.created"));
      }
      onClose();
    } catch (err) {
      handleError(err);
    }
  };

  const selectClass = cn(
    "h-11 w-full rounded-[var(--radius-md)] border border-[var(--color-border)]",
    "bg-[var(--color-surface)] px-3 text-sm text-[var(--color-text-primary)]",
    "transition focus:border-[var(--color-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-primary)]",
  );

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-[620px] gap-0 overflow-hidden p-0">
        {/* Header */}
        <div className="px-8 pt-7 pb-1">
          <h2 className="text-xl font-bold tracking-tight text-[var(--color-text-primary)]">
            {isEdit ? t("vps-asset:form.title_edit") : t("vps-asset:form.title_create")}
          </h2>
          <p className="mt-1 text-[13px] text-[var(--color-text-tertiary)]">
            {t("vps-asset:subtitle")}
          </p>
        </div>

        {/* Scrollable body */}
        <form onSubmit={onSubmit}>
          <div className="max-h-[62vh] overflow-y-auto px-8 py-5 [scrollbar-width:thin] [scrollbar-color:rgba(255,255,255,.1)_transparent] [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-white/10">
            <div className="flex flex-col gap-4">
              {/* Card: Basic Info */}
              <FormCard icon={<Monitor className="h-3.5 w-3.5" />} title={t("vps-asset:form.name")} suffix={`& ${t("vps-asset:form.provider")}`}>
                <div className="grid grid-cols-2 gap-3">
                  <Field label={t("vps-asset:form.name")} req>
                    <Input required value={name} onChange={(e) => setName(e.target.value)} placeholder={t("vps-asset:form.name_placeholder")} className="h-11" />
                  </Field>
                  <Field label={t("vps-asset:form.provider")} req>
                    <Input required value={provider} onChange={(e) => setProvider(e.target.value)} placeholder={t("vps-asset:form.provider_placeholder")} className="h-11" />
                  </Field>
                </div>
                <div className="mt-3 grid grid-cols-2 gap-3">
                  <Field label={t("vps-asset:form.ip")}>
                    <Input value={ip} onChange={(e) => setIp(e.target.value)} placeholder={t("vps-asset:form.ip_placeholder")} className="h-11" />
                  </Field>
                  <Field label={t("vps-asset:form.location")}>
                    <Input value={location} onChange={(e) => setLocation(e.target.value)} placeholder={t("vps-asset:form.location_placeholder")} className="h-11" />
                  </Field>
                </div>
              </FormCard>

              {/* Card: Billing */}
              <FormCard icon={<Clock className="h-3.5 w-3.5" />} title={t("vps-asset:form.price")} suffix={`& ${t("vps-asset:form.expire_at")}`}>
                <div className="grid grid-cols-[2fr_1fr_1.5fr] gap-3">
                  <Field label={t("vps-asset:form.price")} req>
                    <Input required type="number" step="0.01" min="0" value={price} onChange={(e) => setPrice(e.target.value)} placeholder="49.99" className="h-11" />
                  </Field>
                  <Field label={t("vps-asset:form.currency")}>
                    <select value={currency} onChange={(e) => setCurrency(e.target.value)} className={selectClass}>
                      {CURRENCIES.map((c) => <option key={c} value={c}>{c}</option>)}
                    </select>
                  </Field>
                  <Field label={t("vps-asset:form.billing_cycle")} req>
                    <select value={billingCycle} onChange={(e) => setBillingCycle(e.target.value as BillingCycle)} className={selectClass}>
                      {BILLING_CYCLES.map((c) => <option key={c} value={c}>{t(`vps-asset:billing_cycle.${c}`)}</option>)}
                    </select>
                  </Field>
                </div>
                <div className="mt-3">
                  <Field label={t("vps-asset:form.expire_at")} req>
                    <DatePicker value={expireAt} onChange={setExpireAt} />
                  </Field>
                </div>
              </FormCard>

              {/* Card: Hardware */}
              <FormCard icon={<HardDrive className="h-3.5 w-3.5" />} title={t("vps-asset:form.cpu")} suffix={`/ ${t("vps-asset:form.memory")} / ${t("vps-asset:form.disk")}`}>
                <div className="grid grid-cols-3 gap-3">
                  <Field label={t("vps-asset:form.cpu")}>
                    <Input value={cpu} onChange={(e) => setCpu(e.target.value)} placeholder={t("vps-asset:form.cpu_placeholder")} className="h-11" />
                  </Field>
                  <Field label={t("vps-asset:form.memory")}>
                    <Input value={memory} onChange={(e) => setMemory(e.target.value)} placeholder={t("vps-asset:form.memory_placeholder")} className="h-11" />
                  </Field>
                  <Field label={t("vps-asset:form.disk")}>
                    <Input value={disk} onChange={(e) => setDisk(e.target.value)} placeholder={t("vps-asset:form.disk_placeholder")} className="h-11" />
                  </Field>
                </div>
                <div className="mt-3 grid grid-cols-2 gap-3">
                  <Field label={t("vps-asset:form.bandwidth")}>
                    <Input value={bandwidth} onChange={(e) => setBandwidth(e.target.value)} placeholder={t("vps-asset:form.bandwidth_placeholder")} className="h-11" />
                  </Field>
                  <Field label={t("vps-asset:form.monthly_traffic")}>
                    <Input type="number" min="0" value={monthlyTraffic} onChange={(e) => setMonthlyTraffic(e.target.value)} placeholder={t("vps-asset:form.monthly_traffic_placeholder")} className="h-11" />
                  </Field>
                </div>
              </FormCard>

              {/* Card: SSH & Notes */}
              <FormCard icon={<Lock className="h-3.5 w-3.5" />} title="SSH" suffix={`& ${t("vps-asset:form.notes")}`}>
                <div className="grid grid-cols-3 gap-3">
                  <Field label={t("vps-asset:form.ssh_port")}>
                    <Input type="number" value={sshPort} onChange={(e) => setSshPort(e.target.value)} className="h-11" />
                  </Field>
                  <Field label={t("vps-asset:form.ssh_user")}>
                    <Input value={sshUser} onChange={(e) => setSshUser(e.target.value)} placeholder="root" className="h-11" />
                  </Field>
                  <Field label={t("vps-asset:form.os")}>
                    <Input value={os} onChange={(e) => setOs(e.target.value)} placeholder={t("vps-asset:form.os_placeholder")} className="h-11" />
                  </Field>
                </div>
                <div className="mt-3">
                  <Field label={t("vps-asset:form.agent_label")}>
                    <select value={agentId} onChange={(e) => setAgentId(e.target.value)} className={selectClass}>
                      <option value="">{t("vps-asset:form.agent_none")}</option>
                      {agents.map((a) => (
                        <option key={a.id} value={a.id}>{a.name}</option>
                      ))}
                    </select>
                  </Field>
                </div>
                <div className="mt-3">
                  <Field label={t("vps-asset:form.notes")}>
                    <textarea
                      value={notes}
                      onChange={(e) => setNotes(e.target.value)}
                      placeholder={t("vps-asset:form.notes_placeholder")}
                      rows={2}
                      className={cn(
                        "w-full rounded-[var(--radius-md)] border border-[var(--color-border)]",
                        "bg-[var(--color-surface)] px-4 py-3 text-sm leading-relaxed",
                        "text-[var(--color-text-primary)] placeholder:text-[var(--color-text-disabled)]",
                        "transition focus:border-[var(--color-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-primary)]",
                        "resize-y",
                      )}
                    />
                  </Field>
                </div>
              </FormCard>
            </div>
          </div>

          {/* Footer */}
          <div className="flex justify-end gap-2.5 border-t border-[var(--color-border)] px-8 py-5">
            <Button type="button" variant="outline" onClick={onClose} disabled={isPending} className="h-11 px-7">
              {t("common:actions.cancel")}
            </Button>
            <Button type="submit" disabled={isPending} className="h-11 px-7">
              {isEdit ? t("common:actions.save") : t("vps-asset:actions.create")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function FormCard({
  icon, title, suffix, children,
}: {
  icon: React.ReactNode; title: string; suffix?: string; children: React.ReactNode;
}) {
  return (
    <div className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-5">
      <div className="mb-4 flex items-center gap-2 text-[14px] font-semibold text-[var(--color-text-primary)]">
        <span className="flex h-6 w-6 items-center justify-center rounded-md bg-[var(--color-primary-soft)] text-[var(--color-primary)]">
          {icon}
        </span>
        {title}
        {suffix && <span className="font-normal text-[var(--color-text-tertiary)]">{suffix}</span>}
      </div>
      {children}
    </div>
  );
}

function Field({
  label, req, children,
}: {
  label: string; req?: boolean; children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label className="flex items-center gap-1 text-[13px]">
        {label}
        {req && <span className="text-[10px] text-[var(--color-primary)]">*</span>}
      </Label>
      {children}
    </div>
  );
}
