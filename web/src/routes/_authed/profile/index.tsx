import * as React from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  User,
  Lock,
  ShieldCheck,
  Save,
  KeyRound,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { useMeQuery } from "@/api/user";
import { ProfileBasicForm } from "@/components/auth/profile-basic-form";
import { ProfilePasswordForm } from "@/components/auth/profile-password-form";
import { cn } from "@/lib/cn";

export const Route = createFileRoute("/_authed/profile/")({
  component: ProfilePage,
});

type SectionId = "basic" | "password" | "2fa";

interface NavDef {
  id: SectionId;
  icon: React.ReactNode;
  labelKey: string;
  iconBg: string;
  iconColor: string;
}

const NAV: NavDef[] = [
  { id: "basic", icon: <User className="h-[15px] w-[15px]" />, labelKey: "auth:profile.tab_basic", iconBg: "rgba(96,165,250,.08)", iconColor: "var(--color-info)" },
  { id: "password", icon: <Lock className="h-[15px] w-[15px]" />, labelKey: "auth:profile.change_password.title", iconBg: "rgba(251,191,36,.08)", iconColor: "var(--color-warning)" },
  { id: "2fa", icon: <ShieldCheck className="h-[15px] w-[15px]" />, labelKey: "auth:profile.tab_2fa", iconBg: "rgba(52,211,153,.08)", iconColor: "var(--color-success)" },
];

function ProfilePage() {
  const { t } = useTranslation(["auth", "common", "errors"]);
  const { data: me, isLoading, isError, error, refetch } = useMeQuery();
  const [section, setSection] = React.useState<SectionId>("basic");

  if (isLoading) {
    return (
      <div className="mx-auto flex max-w-[900px] flex-col gap-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-[400px] w-full rounded-2xl" />
      </div>
    );
  }

  if (isError || !me) {
    return (
      <ErrorState
        message={(error as Error)?.message ?? t("errors:INTERNAL_UNKNOWN")}
        onRetry={() => refetch()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }

  const initials = me.username.slice(0, 2).toUpperCase();

  return (
    <div className="mx-auto flex max-w-[900px] flex-col gap-6">
      <header>
        <h1 className="text-[26px] font-extrabold tracking-tight text-[var(--color-text-primary)]">
          {t("auth:profile.title")}
        </h1>
      </header>

      <div
        className={cn(
          "flex overflow-hidden rounded-[20px] border border-[var(--color-border)]",
          "bg-[var(--color-surface)] shadow-[0_20px_60px_rgba(0,0,0,0.4)]",
          "min-h-[480px]",
        )}
      >
        {/* Left nav */}
        <nav className="flex w-[220px] flex-shrink-0 flex-col border-r border-[var(--color-border)] bg-gradient-to-b from-white/[.02] to-transparent p-5 pr-3">
          {/* Avatar block */}
          <div className="mb-5 flex flex-col items-center gap-2 rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-4 py-5">
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-[var(--color-primary-soft)] text-lg font-bold text-[var(--color-primary)]">
              {initials}
            </div>
            <div className="text-center">
              <div className="text-[14px] font-semibold text-[var(--color-text-primary)]">{me.username}</div>
              <div className="text-[11px] text-[var(--color-text-tertiary)]">{me.email || "—"}</div>
            </div>
            <span
              className={cn(
                "mt-1 rounded-md px-2 py-0.5 text-[9px] font-bold uppercase",
                me.role === "admin"
                  ? "bg-[rgba(255,99,99,.1)] text-[var(--color-primary)]"
                  : "bg-white/[.04] text-[var(--color-text-tertiary)]",
              )}
            >
              {me.role}
            </span>
          </div>

          <div className="flex flex-col gap-0.5">
            {NAV.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => setSection(item.id)}
                className={cn(
                  "relative flex items-center gap-2.5 rounded-[10px] px-3.5 py-2.5",
                  "text-[13px] font-medium transition-all duration-150 text-left",
                  section === item.id
                    ? "bg-[var(--color-primary-soft)] text-[var(--color-text-primary)]"
                    : "text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)] hover:bg-white/[.03]",
                )}
              >
                {section === item.id && (
                  <span className="absolute left-0 top-1/2 h-[18px] w-[3px] -translate-y-1/2 rounded-r-sm bg-[var(--color-primary)]" />
                )}
                <span
                  className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-lg transition-shadow"
                  style={{
                    background: item.iconBg,
                    color: item.iconColor,
                    boxShadow: section === item.id ? `0 0 8px ${item.iconBg}` : undefined,
                  }}
                >
                  {item.icon}
                </span>
                {t(item.labelKey)}
              </button>
            ))}
          </div>
        </nav>

        {/* Right content */}
        <div className="flex flex-1 flex-col" key={section}>
          <div className="animate-in fade-in slide-in-from-bottom-1 duration-200 flex flex-1 flex-col">
            {section === "basic" && (
              <div className="flex flex-1 flex-col">
                <div className="px-8 pt-6">
                  <h2 className="flex items-center gap-2 text-[18px] font-bold tracking-tight">
                    <span className="flex h-7 w-7 items-center justify-center rounded-lg" style={{ background: "rgba(96,165,250,.1)", color: "var(--color-info)" }}>
                      <User className="h-3.5 w-3.5" />
                    </span>
                    {t("auth:profile.tab_basic")}
                  </h2>
                  <p className="mt-1 text-[12px] text-[var(--color-text-tertiary)]">
                    {t("auth:profile.username_label")} / {t("auth:profile.email_label")} / {t("auth:profile.locale_label")}
                  </p>
                </div>
                <div className="flex-1 px-8 py-5">
                  <ProfileBasicForm
                    initialUsername={me.username}
                    initialEmail={me.email ?? ""}
                    initialLocale={me.locale}
                  />
                </div>
              </div>
            )}

            {section === "password" && (
              <div className="flex flex-1 flex-col">
                <div className="px-8 pt-6">
                  <h2 className="flex items-center gap-2 text-[18px] font-bold tracking-tight">
                    <span className="flex h-7 w-7 items-center justify-center rounded-lg" style={{ background: "rgba(251,191,36,.1)", color: "var(--color-warning)" }}>
                      <Lock className="h-3.5 w-3.5" />
                    </span>
                    {t("auth:profile.change_password.title")}
                  </h2>
                  <p className="mt-1 text-[12px] text-[var(--color-text-tertiary)]">
                    {t("auth:profile.change_password.old_label")} → {t("auth:profile.change_password.new_label")}
                  </p>
                </div>
                <div className="flex-1 px-8 py-5">
                  <ProfilePasswordForm />
                </div>
              </div>
            )}

            {section === "2fa" && (
              <div className="flex flex-1 flex-col">
                <div className="px-8 pt-6">
                  <h2 className="flex items-center gap-2 text-[18px] font-bold tracking-tight">
                    <span className="flex h-7 w-7 items-center justify-center rounded-lg" style={{ background: "rgba(52,211,153,.1)", color: "var(--color-success)" }}>
                      <ShieldCheck className="h-3.5 w-3.5" />
                    </span>
                    {t("auth:profile.tab_2fa")}
                  </h2>
                </div>
                <div className="flex-1 px-8 py-5">
                  <div
                    className={cn(
                      "flex items-center justify-between gap-4 rounded-2xl border p-5",
                      me.totp_enabled
                        ? "border-[rgba(52,211,153,.15)] bg-[rgba(52,211,153,.04)]"
                        : "border-[var(--color-border)] bg-[var(--color-bg-elevated)]",
                    )}
                  >
                    <div className="flex items-center gap-4">
                      <span
                        className={cn(
                          "flex h-10 w-10 items-center justify-center rounded-xl",
                          me.totp_enabled
                            ? "bg-[rgba(52,211,153,.1)] text-[var(--color-success)]"
                            : "bg-white/[.04] text-[var(--color-text-tertiary)]",
                        )}
                      >
                        <KeyRound className="h-5 w-5" />
                      </span>
                      <div>
                        <div className="text-[14px] font-semibold text-[var(--color-text-primary)]">
                          TOTP {t("auth:profile.tab_2fa")}
                        </div>
                        <div className="mt-0.5 text-[12px] text-[var(--color-text-tertiary)]">
                          {me.totp_enabled
                            ? t("auth:profile.two_factor.status_enabled")
                            : t("auth:profile.two_factor.status_disabled")}
                        </div>
                      </div>
                    </div>
                    <Button
                      asChild
                      variant={me.totp_enabled ? "outline" : "default"}
                      className="h-10 px-5"
                    >
                      <Link to="/profile/2fa">
                        {me.totp_enabled
                          ? t("auth:profile.two_factor.disable")
                          : t("auth:profile.two_factor.enable")}
                      </Link>
                    </Button>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
