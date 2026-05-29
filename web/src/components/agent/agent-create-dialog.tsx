import * as React from "react";
import { useTranslation } from "react-i18next";
import { Copy, AlertTriangle, CheckCircle2 } from "lucide-react";
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
import { toast } from "@/components/ui/toast";
import { cn } from "@/lib/cn";
import { useApiError } from "@/hooks/use-api-error";
import { useCreateAgentMutation } from "@/api/agent";
import type {
  AgentCreateResponse,
  AgentKind,
  CreateAgentRequest,
} from "@/types/api";

type WizardStep = 1 | 2 | 3;
type OsChoice = "linux" | "darwin" | "windows";
type ArchChoice = "amd64" | "arm64";

interface AgentCreateDialogProps {
  open: boolean;
  onClose: () => void;
  /** Optional callback invoked once the dialog is dismissed *after* a
   *  successful create. The list refetch is already wired via TanStack. */
  onCreated?: (agent: AgentCreateResponse) => void;
}

/**
 * 3-step agent onboarding wizard.
 *
 *   1. Capture name + OS / arch + kind (native | nezha_compat).
 *   2. POST /api/agents and reveal the one-shot plaintext token plus the
 *      install command (or, for nezha_compat, the migration hint).
 *   3. Confirmation — guides the user back to the list view.
 *
 * The token in step 2 is shown exactly once: closing the dialog or hitting
 * "Finish" wipes it from in-memory state to match the backend contract
 * (`install_hint_i18n_key` documented in internal/types/api.go).
 */
export function AgentCreateDialog({
  open,
  onClose,
  onCreated,
}: AgentCreateDialogProps) {
  const { t } = useTranslation(["agent", "common"]);
  const { handle: handleError } = useApiError();
  const create = useCreateAgentMutation();

  const [step, setStep] = React.useState<WizardStep>(1);
  const [name, setName] = React.useState("");
  const [kind, setKind] = React.useState<AgentKind>("native");
  const [os, setOs] = React.useState<OsChoice>("linux");
  const [arch, setArch] = React.useState<ArchChoice>("amd64");
  const [result, setResult] = React.useState<AgentCreateResponse | null>(null);

  // Reset wizard state every time the dialog re-opens so the token isn't
  // carried across sessions. Run on close too — the secret should not linger.
  React.useEffect(() => {
    if (open) {
      setStep(1);
      setName("");
      setKind("native");
      setOs("linux");
      setArch("amd64");
      setResult(null);
    } else {
      // Wipe the token from memory once the dialog is dismissed.
      setResult(null);
    }
  }, [open]);

  const canSubmit = name.trim().length > 0;

  const onSubmit = async () => {
    if (!canSubmit) return;
    const payload: CreateAgentRequest = { name: name.trim(), kind };
    try {
      const created = await create.mutateAsync(payload);
      setResult(created);
      setStep(2);
      onCreated?.(created);
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("agent:wizard.title")}</DialogTitle>
          <DialogDescription>
            {step === 1 && t("agent:wizard.step_1_title")}
            {step === 2 && t("agent:wizard.step_2_title")}
            {step === 3 && t("agent:wizard.step_3_title")}
          </DialogDescription>
        </DialogHeader>

        <StepIndicator step={step} />

        {step === 1 && (
          <Step1
            name={name}
            setName={setName}
            kind={kind}
            setKind={setKind}
            os={os}
            setOs={setOs}
            arch={arch}
            setArch={setArch}
          />
        )}
        {step === 2 && result && (
          <Step2 result={result} os={os} arch={arch} />
        )}
        {step === 3 && <Step3 />}

        <DialogFooter className="mt-4">
          {step === 1 && (
            <>
              <Button variant="outline" onClick={onClose}>
                {t("common:actions.cancel")}
              </Button>
              <Button
                onClick={onSubmit}
                disabled={!canSubmit || create.isPending}
                data-testid="wizard-next-1"
              >
                {create.isPending
                  ? t("common:loading")
                  : t("agent:wizard.submit")}
              </Button>
            </>
          )}
          {step === 2 && (
            <Button
              onClick={() => setStep(3)}
              data-testid="wizard-next-2"
            >
              {t("agent:wizard.next")}
            </Button>
          )}
          {step === 3 && (
            <Button onClick={onClose} data-testid="wizard-finish">
              {t("agent:wizard.finish")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ── Step 1 ───────────────────────────────────────────────────────────────────

function Step1({
  name,
  setName,
  kind,
  setKind,
  os,
  setOs,
  arch,
  setArch,
}: {
  name: string;
  setName: (v: string) => void;
  kind: AgentKind;
  setKind: (v: AgentKind) => void;
  os: OsChoice;
  setOs: (v: OsChoice) => void;
  arch: ArchChoice;
  setArch: (v: ArchChoice) => void;
}) {
  const { t } = useTranslation("agent");
  return (
    <div className="flex flex-col gap-4" data-testid="wizard-step-1">
      <div className="flex flex-col gap-2">
        <Label htmlFor="agent-name">{t("wizard.name_label")}</Label>
        <Input
          id="agent-name"
          value={name}
          placeholder={t("wizard.name_placeholder")}
          onChange={(e) => setName(e.target.value)}
          autoFocus
        />
        <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("wizard.name_hint")}
        </p>
      </div>

      <div className="flex flex-col gap-2">
        <Label>{t("wizard.kind_label")}</Label>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <KindChoice
            label={t("kind.native")}
            description={t("wizard.kind_native_desc")}
            active={kind === "native"}
            onClick={() => setKind("native")}
            testId="wizard-kind-native"
          />
          <KindChoice
            label={t("kind.nezha_compat")}
            description={t("wizard.kind_nezha_desc")}
            active={kind === "nezha_compat"}
            onClick={() => setKind("nezha_compat")}
            testId="wizard-kind-nezha"
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="flex flex-col gap-2">
          <Label htmlFor="agent-os">{t("wizard.os_label")}</Label>
          <select
            id="agent-os"
            value={os}
            onChange={(e) => setOs(e.target.value as OsChoice)}
            className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
          >
            <option value="linux">{t("wizard.os_linux")}</option>
            <option value="darwin">{t("wizard.os_darwin")}</option>
            <option value="windows">{t("wizard.os_windows")}</option>
          </select>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="agent-arch">{t("wizard.arch_label")}</Label>
          <select
            id="agent-arch"
            value={arch}
            onChange={(e) => setArch(e.target.value as ArchChoice)}
            className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
          >
            <option value="amd64">{t("wizard.arch_amd64")}</option>
            <option value="arm64">{t("wizard.arch_arm64")}</option>
          </select>
        </div>
      </div>
    </div>
  );
}

function KindChoice({
  label,
  description,
  active,
  onClick,
  testId,
}: {
  label: string;
  description: string;
  active: boolean;
  onClick: () => void;
  testId: string;
}) {
  return (
    <button
      type="button"
      data-testid={testId}
      onClick={onClick}
      className={cn(
        "flex flex-col gap-1 rounded-[var(--radius-md)] border p-3 text-left transition-colors duration-[var(--duration-fast)]",
        active
          ? "border-[var(--color-primary)] bg-[var(--color-info-bg)]"
          : "border-[var(--color-border)] bg-[var(--color-surface)] hover:border-[var(--color-border-strong)]",
      )}
    >
      <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
        {label}
      </span>
      <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
        {description}
      </span>
    </button>
  );
}

// ── Step 2 ───────────────────────────────────────────────────────────────────

function Step2({
  result,
  os,
  arch,
}: {
  result: AgentCreateResponse;
  os: OsChoice;
  arch: ArchChoice;
}) {
  const { t } = useTranslation("agent");
  // The native installer already encodes os/arch via the curl URL; for
  // nezha_compat the install_command is the migration hint string only.
  const command = React.useMemo(() => {
    // The backend emits a literal "<hub>" placeholder (it has no request
    // context when building the string); substitute the address the operator
    // is actually viewing the panel at. origin includes the http/https scheme
    // so the resulting curl URL is complete.
    const hub =
      typeof window !== "undefined" ? window.location.origin : "<hub>";
    const installCmd = result.install_command.replaceAll("<hub>", hub);
    if (result.kind === "nezha_compat") return installCmd;
    // Append optional os/arch hints so users on macOS / arm64 can still pick
    // the correct binary if the install script honours those env vars.
    return `${installCmd} --os=${os} --arch=${arch}`;
  }, [result, os, arch]);

  const copy = async (text: string, label: string) => {
    try {
      if (typeof navigator !== "undefined" && navigator.clipboard) {
        await navigator.clipboard.writeText(text);
      }
      toast.success(t("wizard.copied"));
    } catch {
      toast.error(label);
    }
  };

  return (
    <div className="flex flex-col gap-4" data-testid="wizard-step-2">
      <div className="flex items-start gap-2 rounded-[var(--radius-md)] bg-[var(--color-warning-bg)] p-3 text-[var(--font-size-sm)] text-[var(--color-warning)]">
        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
        <span>{t("wizard.token_warning")}</span>
      </div>

      <div className="flex flex-col gap-2">
        <Label>{t("wizard.token_label")}</Label>
        <div className="flex items-stretch gap-2">
          <code
            data-testid="wizard-token"
            className="flex-1 overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-2 font-mono text-[var(--font-size-xs)] text-[var(--color-text-primary)]"
          >
            {result.token}
          </code>
          <Button
            variant="outline"
            size="icon"
            onClick={() => copy(result.token, t("error.create_failed"))}
            aria-label={t("actions.copy_token")}
            data-testid="wizard-copy-token"
          >
            <Copy className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {result.kind === "nezha_compat" ? (
        <div className="flex flex-col gap-2">
          <Label>{t("wizard.nezha_hint_title")}</Label>
          <pre
            data-testid="wizard-nezha-hint"
            className="whitespace-pre-wrap rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-3 font-mono text-[var(--font-size-xs)] text-[var(--color-text-primary)]"
          >
            {t("wizard.nezha_hint_body", {
              origin:
                typeof window !== "undefined"
                  ? window.location.origin
                  : "<hub>",
              token: result.token,
            })}
          </pre>
        </div>
      ) : (
        <div className="flex flex-col gap-2">
          <Label>{t("wizard.install_label")}</Label>
          <div className="flex items-stretch gap-2">
            <code
              data-testid="wizard-install-command"
              className="flex-1 overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-2 font-mono text-[var(--font-size-xs)] text-[var(--color-text-primary)]"
            >
              {command}
            </code>
            <Button
              variant="outline"
              size="icon"
              onClick={() => copy(command, t("error.create_failed"))}
              aria-label={t("actions.copy_command")}
              data-testid="wizard-copy-command"
            >
              <Copy className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Step 3 ───────────────────────────────────────────────────────────────────

function Step3() {
  const { t } = useTranslation("agent");
  return (
    <div
      data-testid="wizard-step-3"
      className="flex flex-col items-center gap-3 py-6 text-center"
    >
      <CheckCircle2 className="h-12 w-12 text-[var(--color-status-online)]" />
      <p className="text-[var(--font-size-base)] font-medium text-[var(--color-text-primary)]">
        {t("wizard.step_3_title")}
      </p>
      <p className="max-w-sm text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
        {t("detail.no_metrics")}
      </p>
    </div>
  );
}

// ── Step indicator ──────────────────────────────────────────────────────────

function StepIndicator({ step }: { step: WizardStep }) {
  return (
    <div className="flex items-center gap-2 py-1" aria-hidden>
      {[1, 2, 3].map((s) => (
        <span
          key={s}
          className={cn(
            "h-1 flex-1 rounded-full transition-colors",
            s <= step
              ? "bg-[var(--color-primary)]"
              : "bg-[var(--color-surface-hover)]",
          )}
        />
      ))}
    </div>
  );
}
