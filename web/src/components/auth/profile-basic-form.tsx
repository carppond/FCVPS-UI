import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useUpdateProfileMutation } from "@/api/user";

export type AppLocale = "zh-CN" | "en" | "ja" | "ko";

const SUPPORTED_LOCALES: AppLocale[] = ["zh-CN", "en", "ja", "ko"];

function normalizeLocale(input: string): AppLocale {
  return (SUPPORTED_LOCALES as string[]).includes(input)
    ? (input as AppLocale)
    : "zh-CN";
}

interface ProfileFormValues {
  username: string;
  email: string;
  locale: AppLocale;
}

interface ProfileBasicFormProps {
  initialUsername: string;
  initialEmail: string;
  initialLocale: string;
}

export function ProfileBasicForm({
  initialUsername,
  initialEmail,
  initialLocale,
}: ProfileBasicFormProps) {
  const { t } = useTranslation(["auth"]);
  const { handle: handleError } = useApiError();
  const updateMutation = useUpdateProfileMutation();

  const schema = React.useMemo(
    () =>
      z.object({
        username: z
          .string()
          .min(3, t("auth:login.error.username_length"))
          .max(32, t("auth:login.error.username_length")),
        email: z.string().refine((v) => v === "" || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v), {
          message: "invalid email",
        }),
        locale: z.enum(["zh-CN", "en", "ja", "ko"]),
      }),
    [t],
  );

  const form = useForm<ProfileFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      username: initialUsername,
      email: initialEmail,
      locale: normalizeLocale(initialLocale),
    },
  });

  const onSubmit = form.handleSubmit(async (values) => {
    try {
      await updateMutation.mutateAsync({
        username: values.username,
        email: values.email || undefined,
        locale: values.locale,
      });
      toast.success(t("auth:profile.save_success"));
    } catch (err) {
      handleError(err);
    }
  });

  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
      <div className="flex flex-col gap-2">
        <Label htmlFor="profile-username">{t("auth:profile.username_label")}</Label>
        <Input id="profile-username" {...form.register("username")} />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="profile-email">{t("auth:profile.email_label")}</Label>
        <Input
          id="profile-email"
          type="email"
          placeholder={t("auth:profile.email_placeholder")}
          {...form.register("email")}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="profile-locale">{t("auth:profile.locale_label")}</Label>
        <select
          id="profile-locale"
          className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
          {...form.register("locale")}
        >
          <option value="zh-CN">中文</option>
          <option value="en">English</option>
          <option value="ja">日本語</option>
          <option value="ko">한국어</option>
        </select>
      </div>
      <div>
        <Button type="submit" disabled={updateMutation.isPending}>
          {t("auth:profile.save")}
        </Button>
      </div>
    </form>
  );
}
