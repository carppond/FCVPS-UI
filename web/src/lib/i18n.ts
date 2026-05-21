import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";

// ── Eagerly loaded namespaces (首屏必须) ─────────────────────────────────────
// Per tech-lead-plan §2.3: first-screen bundle ships common + errors + auth.
import zhCNCommon from "@/locales/zh-CN/common.json";
import zhCNErrors from "@/locales/zh-CN/errors.json";
import zhCNAuth from "@/locales/zh-CN/auth.json";
import enCommon from "@/locales/en/common.json";
import enErrors from "@/locales/en/errors.json";
import enAuth from "@/locales/en/auth.json";
import jaCommon from "@/locales/ja/common.json";
import jaErrors from "@/locales/ja/errors.json";
import jaAuth from "@/locales/ja/auth.json";
import koCommon from "@/locales/ko/common.json";
import koErrors from "@/locales/ko/errors.json";
import koAuth from "@/locales/ko/auth.json";

const EAGER_NS = ["common", "errors", "auth"] as const;

// ── Namespaces that load lazily (by page / feature) ──────────────────────────
// auth, subscription, pipeline, node, rule, script, agent, traffic, notify, settings
// These are added via i18next.addResourceBundle() when the relevant page is mounted.

void i18next
  .use(initReactI18next)
  .use(LanguageDetector)
  .init({
    fallbackLng: "zh-CN",
    supportedLngs: ["zh-CN", "en", "ja", "ko"],
    defaultNS: "common",
    ns: [...EAGER_NS],
    resources: {
      "zh-CN": { common: zhCNCommon, errors: zhCNErrors, auth: zhCNAuth },
      en: { common: enCommon, errors: enErrors, auth: enAuth },
      ja: { common: jaCommon, errors: jaErrors, auth: jaAuth },
      ko: { common: koCommon, errors: koErrors, auth: koAuth },
    },
    detection: {
      order: ["localStorage", "navigator"],
      caches: ["localStorage"],
      lookupLocalStorage: "i18next_lng",
    },
    interpolation: {
      escapeValue: false, // React already escapes
    },
    react: {
      useSuspense: false,
    },
  });

export default i18next;
