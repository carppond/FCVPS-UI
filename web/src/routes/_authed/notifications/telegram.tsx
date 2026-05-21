import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { TGBotSettings } from "@/components/notify/tg-bot-settings";
import i18n from "@/lib/i18n";
import notifyZh from "@/locales/zh-CN/notify.json";
import notifyEn from "@/locales/en/notify.json";
import notifyJa from "@/locales/ja/notify.json";
import notifyKo from "@/locales/ko/notify.json";

// Lazy-register the "notify" namespace before mount. The Telegram tab is a
// standalone page (not embedded inside /notifications) so this route owns the
// namespace seeding for the bot-configuration UI specifically — keeping the
// first-screen bundle slim per tech-lead-plan §2.3.
function ensureNotifyNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "notify")) {
    i18n.addResourceBundle("zh-CN", "notify", notifyZh, true, true);
    i18n.addResourceBundle("en", "notify", notifyEn, true, true);
    i18n.addResourceBundle("ja", "notify", notifyJa, true, true);
    i18n.addResourceBundle("ko", "notify", notifyKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/notifications/telegram")({
  beforeLoad: () => {
    ensureNotifyNamespace();
  },
  component: TelegramBotPage,
});

function TelegramBotPage() {
  const { t } = useTranslation(["notify"]);
  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-[var(--spacing-6)]">
      <header>
        <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
          {t("notify:telegram.page_title")}
        </h1>
        <p className="mt-[var(--spacing-1)] text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("notify:telegram.page_description")}
        </p>
      </header>
      <TGBotSettings />
    </div>
  );
}
