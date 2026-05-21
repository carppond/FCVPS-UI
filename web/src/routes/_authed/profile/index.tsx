import { createFileRoute, Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { ProfileBasicForm } from "@/components/auth/profile-basic-form";
import { ProfilePasswordForm } from "@/components/auth/profile-password-form";
import { useMeQuery } from "@/api/user";

export const Route = createFileRoute("/_authed/profile/")({
  component: ProfilePage,
});

function ProfilePage() {
  const { t } = useTranslation(["auth", "common", "errors"]);
  const { data: me, isLoading, isError, error, refetch } = useMeQuery();

  if (isLoading) return <ProfileSkeleton />;
  if (isError || !me) {
    return (
      <ErrorState
        message={
          (error as Error)?.message ?? t("errors:INTERNAL_UNKNOWN")
        }
        onRetry={() => refetch()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <div>
        <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
          {t("auth:profile.title")}
        </h1>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("auth:profile.tab_basic")}</CardTitle>
        </CardHeader>
        <CardContent>
          <ProfileBasicForm
            initialUsername={me.username}
            initialEmail={me.email ?? ""}
            initialLocale={me.locale}
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("auth:profile.change_password.title")}</CardTitle>
        </CardHeader>
        <CardContent>
          <ProfilePasswordForm />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("auth:profile.tab_2fa")}</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-between gap-4">
          <p className="text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
            {me.totp_enabled
              ? t("auth:profile.two_factor.status_enabled")
              : t("auth:profile.two_factor.status_disabled")}
          </p>
          <Button asChild>
            <Link to="/profile/2fa">
              {me.totp_enabled
                ? t("auth:profile.two_factor.disable")
                : t("auth:profile.two_factor.enable")}
            </Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

function ProfileSkeleton() {
  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <Skeleton className="h-8 w-48" />
      <Skeleton className="h-40 w-full" />
      <Skeleton className="h-40 w-full" />
    </div>
  );
}
