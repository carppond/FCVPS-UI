import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { LoginForm } from "@/components/auth/login-form";

export const Route = createFileRoute("/_public/login")({
  component: LoginPage,
});

function LoginPage() {
  const { t } = useTranslation(["auth", "common"]);
  return (
    <div className="w-full max-w-sm px-4">
      <div className="mb-8 text-center">
        <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
          {t("auth:login.title")}
        </h1>
        <p className="mt-2 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("auth:login.subtitle")}
        </p>
      </div>
      <LoginForm />
    </div>
  );
}
