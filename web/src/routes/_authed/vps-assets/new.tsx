import * as React from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  ArrowLeft,
  Monitor,
  Clock,
  HardDrive,
  Lock,
} from "lucide-react";
import { Link } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { DatePicker } from "@/components/ui/date-picker";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useCreateVpsAssetMutation } from "@/api/vps-asset";
import { cn } from "@/lib/cn";
import type { BillingCycle, CreateVpsAssetRequest } from "@/types/api";

export const Route = createFileRoute("/_authed/vps-assets/new")({
  component: NewVpsAssetPage,
});

const BILLING_CYCLES: BillingCycle[] = [
  "monthly",
  "quarterly",
  "semi_annual",
  "annual",
  "biennial",
  "triennial",
];

const CURRENCIES = ["USD", "CNY", "EUR", "GBP", "JPY", "KRW"];

function NewVpsAssetPage() {
  const { t } = useTranslation(["vps-asset", "common"]);
  const { handle: handleError } = useApiError();
  const navigate = useNavigate();
  const createMutation = useCreateVpsAssetMutation();

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

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const req: CreateVpsAssetRequest = {
        name,
        provider,
        price: Number(price),
        billing_cycle: billingCycle,
        expire_at: expireAt,
        currency,
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
      await createMutation.mutateAsync(req);
      toast.success(t("vps-asset:toast.created"));
      navigate({ to: "/vps-assets" });
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="mx-auto max-w-[640px] py-2">
      <Link
        to="/vps-assets"
        className="mb-4 inline-flex items-center gap-1.5 text-xs text-[var(--color-text-tertiary)] transition-colors hover:text-[var(--color-text-primary)]"
      >
        <ArrowLeft className="h-3.5 w-3.5" />
        {t("common:actions.back")} VPS {t("vps-asset:title")}
      </Link>

      <h1 className="text-[28px] font-extrabold tracking-tight text-[var(--color-text-primary)]">
        {t("vps-asset:form.title_create")}
      </h1>
      <p className="mt-1 mb-8 text-sm text-[var(--color-text-tertiary)]">
        {t("vps-asset:subtitle")}
      </p>

      <form onSubmit={onSubmit} className="flex flex-col gap-4">
        {/* Section: Basic Info */}
        <FormCard
          icon={<Monitor className="h-3.5 w-3.5" />}
          title={t("vps-asset:form.name")}
          titleSuffix={`& ${t("vps-asset:form.provider")}`}
        >
          <div className="grid grid-cols-2 gap-3.5">
            <FormField label={t("vps-asset:form.name")} required>
              <Input
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t("vps-asset:form.name_placeholder")}
                className="h-11"
              />
            </FormField>
            <FormField label={t("vps-asset:form.provider")} required>
              <Input
                required
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
                placeholder={t("vps-asset:form.provider_placeholder")}
                className="h-11"
              />
            </FormField>
          </div>
          <div className="mt-3.5 grid grid-cols-2 gap-3.5">
            <FormField label={t("vps-asset:form.ip")} optional>
              <Input
                value={ip}
                onChange={(e) => setIp(e.target.value)}
                placeholder={t("vps-asset:form.ip_placeholder")}
                className="h-11"
              />
            </FormField>
            <FormField label={t("vps-asset:form.location")} optional>
              <Input
                value={location}
                onChange={(e) => setLocation(e.target.value)}
                placeholder={t("vps-asset:form.location_placeholder")}
                className="h-11"
              />
            </FormField>
          </div>
        </FormCard>

        {/* Section: Billing */}
        <FormCard
          icon={<Clock className="h-3.5 w-3.5" />}
          title={t("vps-asset:form.price")}
          titleSuffix={`& ${t("vps-asset:form.expire_at")}`}
        >
          <div className="grid grid-cols-[2fr_1fr_1.5fr] gap-3.5">
            <FormField label={t("vps-asset:form.price")} required>
              <Input
                required
                type="number"
                step="0.01"
                min="0"
                value={price}
                onChange={(e) => setPrice(e.target.value)}
                placeholder="49.99"
                className="h-11"
              />
            </FormField>
            <FormField label={t("vps-asset:form.currency")}>
              <select
                value={currency}
                onChange={(e) => setCurrency(e.target.value)}
                className={cn(
                  "h-11 w-full rounded-[var(--radius-md)] border border-[var(--color-border)]",
                  "bg-[var(--color-surface)] px-3 text-sm text-[var(--color-text-primary)]",
                  "transition focus:border-[var(--color-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-primary)]",
                )}
              >
                {CURRENCIES.map((c) => (
                  <option key={c} value={c}>{c}</option>
                ))}
              </select>
            </FormField>
            <FormField label={t("vps-asset:form.billing_cycle")} required>
              <select
                value={billingCycle}
                onChange={(e) => setBillingCycle(e.target.value as BillingCycle)}
                className={cn(
                  "h-11 w-full rounded-[var(--radius-md)] border border-[var(--color-border)]",
                  "bg-[var(--color-surface)] px-3 text-sm text-[var(--color-text-primary)]",
                  "transition focus:border-[var(--color-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-primary)]",
                )}
              >
                {BILLING_CYCLES.map((c) => (
                  <option key={c} value={c}>
                    {t(`vps-asset:billing_cycle.${c}`)}
                  </option>
                ))}
              </select>
            </FormField>
          </div>
          <div className="mt-3.5">
            <FormField label={t("vps-asset:form.expire_at")} required>
              <DatePicker
                value={expireAt}
                onChange={setExpireAt}
              />
            </FormField>
          </div>
        </FormCard>

        {/* Section: Hardware */}
        <FormCard
          icon={<HardDrive className="h-3.5 w-3.5" />}
          title={t("vps-asset:form.cpu")}
          titleSuffix={`/ ${t("vps-asset:form.memory")} / ${t("vps-asset:form.disk")}`}
        >
          <div className="grid grid-cols-3 gap-3.5">
            <FormField label={t("vps-asset:form.cpu")}>
              <Input
                value={cpu}
                onChange={(e) => setCpu(e.target.value)}
                placeholder={t("vps-asset:form.cpu_placeholder")}
                className="h-11"
              />
            </FormField>
            <FormField label={t("vps-asset:form.memory")}>
              <Input
                value={memory}
                onChange={(e) => setMemory(e.target.value)}
                placeholder={t("vps-asset:form.memory_placeholder")}
                className="h-11"
              />
            </FormField>
            <FormField label={t("vps-asset:form.disk")}>
              <Input
                value={disk}
                onChange={(e) => setDisk(e.target.value)}
                placeholder={t("vps-asset:form.disk_placeholder")}
                className="h-11"
              />
            </FormField>
          </div>
          <div className="mt-3.5 grid grid-cols-2 gap-3.5">
            <FormField label={t("vps-asset:form.bandwidth")}>
              <Input
                value={bandwidth}
                onChange={(e) => setBandwidth(e.target.value)}
                placeholder={t("vps-asset:form.bandwidth_placeholder")}
                className="h-11"
              />
            </FormField>
            <FormField label={t("vps-asset:form.monthly_traffic")}>
              <Input
                type="number"
                min="0"
                value={monthlyTraffic}
                onChange={(e) => setMonthlyTraffic(e.target.value)}
                placeholder={t("vps-asset:form.monthly_traffic_placeholder")}
                className="h-11"
              />
            </FormField>
          </div>
        </FormCard>

        {/* Section: SSH & Notes */}
        <FormCard
          icon={<Lock className="h-3.5 w-3.5" />}
          title="SSH"
          titleSuffix={`& ${t("vps-asset:form.notes")}`}
        >
          <div className="grid grid-cols-3 gap-3.5">
            <FormField label={t("vps-asset:form.ssh_port")}>
              <Input
                type="number"
                value={sshPort}
                onChange={(e) => setSshPort(e.target.value)}
                className="h-11"
              />
            </FormField>
            <FormField label={t("vps-asset:form.ssh_user")}>
              <Input
                value={sshUser}
                onChange={(e) => setSshUser(e.target.value)}
                placeholder="root"
                className="h-11"
              />
            </FormField>
            <FormField label={t("vps-asset:form.os")}>
              <Input
                value={os}
                onChange={(e) => setOs(e.target.value)}
                placeholder={t("vps-asset:form.os_placeholder")}
                className="h-11"
              />
            </FormField>
          </div>
          <div className="mt-3.5">
            <FormField label={t("vps-asset:form.notes")}>
              <textarea
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                placeholder={t("vps-asset:form.notes_placeholder")}
                rows={3}
                className={cn(
                  "w-full rounded-[var(--radius-md)] border border-[var(--color-border)]",
                  "bg-[var(--color-surface)] px-4 py-3 text-sm leading-relaxed",
                  "text-[var(--color-text-primary)] placeholder:text-[var(--color-text-disabled)]",
                  "transition focus:border-[var(--color-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-primary)]",
                  "resize-y",
                )}
              />
            </FormField>
          </div>
        </FormCard>

        {/* Actions */}
        <div className="flex justify-end gap-2.5 pb-10 pt-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => navigate({ to: "/vps-assets" })}
            disabled={createMutation.isPending}
            className="h-11 px-7"
          >
            {t("common:actions.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={createMutation.isPending}
            className="h-11 px-7"
          >
            {t("vps-asset:actions.create")}
          </Button>
        </div>
      </form>
    </div>
  );
}

function FormCard({
  icon,
  title,
  titleSuffix,
  children,
}: {
  icon: React.ReactNode;
  title: string;
  titleSuffix?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-7">
      <div className="mb-5 flex items-center gap-2.5 text-[15px] font-semibold text-[var(--color-text-primary)]">
        <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-[var(--color-primary-soft)] text-[var(--color-primary)]">
          {icon}
        </span>
        {title}
        {titleSuffix && (
          <span className="font-normal text-[var(--color-text-tertiary)]">
            {titleSuffix}
          </span>
        )}
      </div>
      {children}
    </div>
  );
}

function FormField({
  label,
  required,
  optional,
  children,
}: {
  label: string;
  required?: boolean;
  optional?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label className="flex items-center gap-1 text-[13px]">
        {label}
        {required && (
          <span className="text-[10px] text-[var(--color-primary)]">*</span>
        )}
        {optional && (
          <span className="ml-auto text-[11px] font-normal text-[var(--color-text-disabled)]">
            optional
          </span>
        )}
      </Label>
      {children}
    </div>
  );
}
