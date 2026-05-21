import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/toast";
import { TotpInput, TOTP_CODE_LENGTH } from "@/components/auth/totp-input";
import { RecoveryCodesDialog } from "@/components/auth/recovery-codes-dialog";
import { useApiError } from "@/hooks/use-api-error";
import {
  useConfirmTotpMutation,
  useDisableTotpMutation,
  useMeQuery,
  useRegenRecoveryMutation,
  useTotpSetupQuery,
} from "@/api/user";

export const Route = createFileRoute("/_authed/profile/2fa")({
  component: TwoFactorPage,
});

function TwoFactorPage() {
  const { t } = useTranslation(["auth"]);
  const { data: me, isLoading } = useMeQuery();

  if (isLoading) {
    return (
      <div className="mx-auto max-w-xl">
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (!me) return null;
  return (
    <div className="mx-auto flex max-w-xl flex-col gap-6">
      <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
        {t("auth:profile.tab_2fa")}
      </h1>
      {me.totp_enabled ? <DisableSection /> : <EnableSection />}
    </div>
  );
}

function EnableSection() {
  const { t } = useTranslation(["auth"]);
  const { handle: handleError } = useApiError();
  const setupQuery = useTotpSetupQuery(true);
  const confirmMutation = useConfirmTotpMutation();

  const [code, setCode] = React.useState("");
  const [hasError, setHasError] = React.useState(false);
  const [codes, setCodes] = React.useState<string[] | null>(null);

  const submit = async () => {
    if (code.length !== TOTP_CODE_LENGTH) {
      setHasError(true);
      toast.error(t("auth:totp.error.incomplete"));
      return;
    }
    try {
      const data = await confirmMutation.mutateAsync({ code });
      setCodes(data.backup_codes);
      toast.success(t("auth:totp.setup.success"));
    } catch (err) {
      setHasError(true);
      setCode("");
      handleError(err);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("auth:totp.setup.title")}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {setupQuery.isLoading && <Skeleton className="h-40 w-full" />}
        {setupQuery.data && (
          <div className="flex flex-col gap-4">
            <p className="text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
              {t("auth:totp.setup.scan_qr")}
            </p>
            {setupQuery.data.qr_code_url && (
              <div className="flex justify-center">
                <img
                  src={setupQuery.data.qr_code_url}
                  alt="TOTP QR code"
                  className="h-44 w-44 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-white p-2"
                />
              </div>
            )}
            <div>
              <Label className="mb-1 block">
                {t("auth:totp.setup.manual_key")}
              </Label>
              <code className="block break-all rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 font-mono text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
                {setupQuery.data.secret}
              </code>
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("auth:totp.setup.verify_label")}</Label>
              <TotpInput
                value={code}
                onChange={(v) => {
                  setCode(v);
                  if (hasError) setHasError(false);
                }}
                onComplete={() => void submit()}
                hasError={hasError}
                disabled={confirmMutation.isPending}
              />
            </div>
            <Button onClick={() => void submit()} disabled={confirmMutation.isPending}>
              {t("auth:totp.setup.confirm")}
            </Button>
          </div>
        )}
      </CardContent>

      <RecoveryCodesDialog
        open={!!codes}
        codes={codes ?? []}
        onConfirmed={() => {
          setCodes(null);
          window.location.assign("/profile");
        }}
      />
    </Card>
  );
}

interface DisableValues {
  password: string;
  code: string;
}

function DisableSection() {
  const { t } = useTranslation(["auth"]);
  const { handle: handleError } = useApiError();
  const disableMutation = useDisableTotpMutation();
  const regenMutation = useRegenRecoveryMutation();
  const [codes, setCodes] = React.useState<string[] | null>(null);

  const schema = z.object({
    password: z.string().min(1),
    code: z.string().regex(/^\d{6}$/, t("auth:totp.error.invalid")),
  });

  const form = useForm<DisableValues>({
    resolver: zodResolver(schema),
    defaultValues: { password: "", code: "" },
  });

  const onSubmit = form.handleSubmit(async (values) => {
    try {
      await disableMutation.mutateAsync({
        code: values.code,
        password: values.password,
      });
      toast.success(t("auth:totp.setup.disable_success"));
      window.location.assign("/profile");
    } catch (err) {
      handleError(err);
    }
  });

  const regenerate = async () => {
    const password = window.prompt(
      t("auth:totp.setup.disable_password_label"),
    );
    if (!password) return;
    try {
      const data = await regenMutation.mutateAsync(password);
      setCodes(data.backup_codes);
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t("auth:totp.setup.disable_title")}</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
            <p className="text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
              {t("auth:totp.setup.disable_description")}
            </p>
            <div className="flex flex-col gap-2">
              <Label htmlFor="disable-password">
                {t("auth:totp.setup.disable_password_label")}
              </Label>
              <Input
                id="disable-password"
                type="password"
                autoComplete="current-password"
                {...form.register("password")}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="disable-code">{t("auth:totp.title")}</Label>
              <Input
                id="disable-code"
                maxLength={TOTP_CODE_LENGTH}
                className="font-mono tabular-nums"
                {...form.register("code")}
              />
              {form.formState.errors.code && (
                <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
                  {form.formState.errors.code.message}
                </p>
              )}
            </div>
            <Button
              type="submit"
              variant="destructive"
              disabled={disableMutation.isPending}
            >
              {t("auth:totp.setup.disable_submit")}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>
            {t("auth:profile.two_factor.recovery_section")}
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
            {t("auth:recovery_codes.regenerate_warning")}
          </p>
          <div>
            <Button
              variant="outline"
              onClick={() => void regenerate()}
              disabled={regenMutation.isPending}
            >
              {t("auth:recovery_codes.regenerate")}
            </Button>
          </div>
        </CardContent>
      </Card>

      <RecoveryCodesDialog
        open={!!codes}
        codes={codes ?? []}
        onConfirmed={() => setCodes(null)}
      />
    </>
  );
}
