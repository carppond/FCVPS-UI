import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";

// ── Eagerly loaded namespaces (首屏必须) ─────────────────────────────────────
import zhCNCommon from "@/locales/zh-CN/common.json";
import zhCNErrors from "@/locales/zh-CN/errors.json";
import enCommon from "@/locales/en/common.json";
import enErrors from "@/locales/en/errors.json";
import jaCommon from "@/locales/ja/common.json";
import jaErrors from "@/locales/ja/errors.json";
import koCommon from "@/locales/ko/common.json";
import koErrors from "@/locales/ko/errors.json";

const EAGER_NS = ["common", "errors"] as const;

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
      "zh-CN": { common: zhCNCommon, errors: zhCNErrors },
      en: { common: enCommon, errors: enErrors },
      ja: { common: jaCommon, errors: jaErrors },
      ko: { common: koCommon, errors: koErrors },
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
