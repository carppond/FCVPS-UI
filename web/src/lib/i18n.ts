import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";

// ─────────────────────────────────────────────────────────────────────────────
// All 16 namespaces are loaded eagerly.
//
// Rationale (P0 bug fix 2026-05): the previous design relied on per-route
// `i18next.addResourceBundle()` calls to lazy-load page-level namespaces.
// In practice only a handful of pages implemented that, and even those that
// did registered the bundle inside `useEffect`, so the first render rendered
// raw t() keys (or English residue) when the user had switched to zh-CN.
//
// Static-importing every namespace adds ~40-50 KB gzip to the first-screen
// bundle, which is fully acceptable for a self-hosted admin panel.
// ─────────────────────────────────────────────────────────────────────────────

// ── zh-CN ────────────────────────────────────────────────────────────────────
import zhCNCommon from "@/locales/zh-CN/common.json";
import zhCNErrors from "@/locales/zh-CN/errors.json";
import zhCNAuth from "@/locales/zh-CN/auth.json";
import zhCNPipeline from "@/locales/zh-CN/pipeline.json";
import zhCNAgent from "@/locales/zh-CN/agent.json";
import zhCNAudit from "@/locales/zh-CN/audit.json";
import zhCNCmdk from "@/locales/zh-CN/cmdk.json";
import zhCNDashboard from "@/locales/zh-CN/dashboard.json";
import zhCNNode from "@/locales/zh-CN/node.json";
import zhCNNotify from "@/locales/zh-CN/notify.json";
import zhCNRule from "@/locales/zh-CN/rule.json";
import zhCNRuleSet from "@/locales/zh-CN/rule-set.json";
import zhCNProxyGroup from "@/locales/zh-CN/proxy-group.json";
import zhCNScript from "@/locales/zh-CN/script.json";
import zhCNSettings from "@/locales/zh-CN/settings.json";
import zhCNShortlink from "@/locales/zh-CN/shortlink.json";
import zhCNSubscription from "@/locales/zh-CN/subscription.json";
import zhCNTraffic from "@/locales/zh-CN/traffic.json";
import zhCNVpsAsset from "@/locales/zh-CN/vps-asset.json";

// ── en ───────────────────────────────────────────────────────────────────────
import enCommon from "@/locales/en/common.json";
import enErrors from "@/locales/en/errors.json";
import enAuth from "@/locales/en/auth.json";
import enPipeline from "@/locales/en/pipeline.json";
import enAgent from "@/locales/en/agent.json";
import enAudit from "@/locales/en/audit.json";
import enCmdk from "@/locales/en/cmdk.json";
import enDashboard from "@/locales/en/dashboard.json";
import enNode from "@/locales/en/node.json";
import enNotify from "@/locales/en/notify.json";
import enRule from "@/locales/en/rule.json";
import enRuleSet from "@/locales/en/rule-set.json";
import enProxyGroup from "@/locales/en/proxy-group.json";
import enScript from "@/locales/en/script.json";
import enSettings from "@/locales/en/settings.json";
import enShortlink from "@/locales/en/shortlink.json";
import enSubscription from "@/locales/en/subscription.json";
import enTraffic from "@/locales/en/traffic.json";
import enVpsAsset from "@/locales/en/vps-asset.json";

// ── ja ───────────────────────────────────────────────────────────────────────
import jaCommon from "@/locales/ja/common.json";
import jaErrors from "@/locales/ja/errors.json";
import jaAuth from "@/locales/ja/auth.json";
import jaPipeline from "@/locales/ja/pipeline.json";
import jaAgent from "@/locales/ja/agent.json";
import jaAudit from "@/locales/ja/audit.json";
import jaCmdk from "@/locales/ja/cmdk.json";
import jaDashboard from "@/locales/ja/dashboard.json";
import jaNode from "@/locales/ja/node.json";
import jaNotify from "@/locales/ja/notify.json";
import jaRule from "@/locales/ja/rule.json";
import jaRuleSet from "@/locales/ja/rule-set.json";
import jaProxyGroup from "@/locales/ja/proxy-group.json";
import jaScript from "@/locales/ja/script.json";
import jaSettings from "@/locales/ja/settings.json";
import jaShortlink from "@/locales/ja/shortlink.json";
import jaSubscription from "@/locales/ja/subscription.json";
import jaTraffic from "@/locales/ja/traffic.json";
import jaVpsAsset from "@/locales/ja/vps-asset.json";

// ── ko ───────────────────────────────────────────────────────────────────────
import koCommon from "@/locales/ko/common.json";
import koErrors from "@/locales/ko/errors.json";
import koAuth from "@/locales/ko/auth.json";
import koPipeline from "@/locales/ko/pipeline.json";
import koAgent from "@/locales/ko/agent.json";
import koAudit from "@/locales/ko/audit.json";
import koCmdk from "@/locales/ko/cmdk.json";
import koDashboard from "@/locales/ko/dashboard.json";
import koNode from "@/locales/ko/node.json";
import koNotify from "@/locales/ko/notify.json";
import koRule from "@/locales/ko/rule.json";
import koRuleSet from "@/locales/ko/rule-set.json";
import koProxyGroup from "@/locales/ko/proxy-group.json";
import koScript from "@/locales/ko/script.json";
import koSettings from "@/locales/ko/settings.json";
import koShortlink from "@/locales/ko/shortlink.json";
import koSubscription from "@/locales/ko/subscription.json";
import koTraffic from "@/locales/ko/traffic.json";
import koVpsAsset from "@/locales/ko/vps-asset.json";

const EAGER_NS = [
  "common",
  "errors",
  "auth",
  "pipeline",
  "agent",
  "audit",
  "cmdk",
  "dashboard",
  "node",
  "notify",
  "rule",
  "rule-set",
  "proxy-group",
  "script",
  "settings",
  "shortlink",
  "subscription",
  "traffic",
  "vps-asset",
] as const;

void i18next
  .use(initReactI18next)
  .use(LanguageDetector)
  .init({
    fallbackLng: "zh-CN",
    supportedLngs: ["zh-CN", "en", "ja", "ko"],
    defaultNS: "common",
    ns: [...EAGER_NS],
    resources: {
      "zh-CN": {
        common: zhCNCommon,
        errors: zhCNErrors,
        auth: zhCNAuth,
        pipeline: zhCNPipeline,
        agent: zhCNAgent,
        audit: zhCNAudit,
        cmdk: zhCNCmdk,
        dashboard: zhCNDashboard,
        node: zhCNNode,
        notify: zhCNNotify,
        rule: zhCNRule,
        "rule-set": zhCNRuleSet,
        "proxy-group": zhCNProxyGroup,
        script: zhCNScript,
        settings: zhCNSettings,
        shortlink: zhCNShortlink,
        subscription: zhCNSubscription,
        traffic: zhCNTraffic,
        "vps-asset": zhCNVpsAsset,
      },
      en: {
        common: enCommon,
        errors: enErrors,
        auth: enAuth,
        pipeline: enPipeline,
        agent: enAgent,
        audit: enAudit,
        cmdk: enCmdk,
        dashboard: enDashboard,
        node: enNode,
        notify: enNotify,
        rule: enRule,
        "rule-set": enRuleSet,
        "proxy-group": enProxyGroup,
        script: enScript,
        settings: enSettings,
        shortlink: enShortlink,
        subscription: enSubscription,
        traffic: enTraffic,
        "vps-asset": enVpsAsset,
      },
      ja: {
        common: jaCommon,
        errors: jaErrors,
        auth: jaAuth,
        pipeline: jaPipeline,
        agent: jaAgent,
        audit: jaAudit,
        cmdk: jaCmdk,
        dashboard: jaDashboard,
        node: jaNode,
        notify: jaNotify,
        rule: jaRule,
        "rule-set": jaRuleSet,
        "proxy-group": jaProxyGroup,
        script: jaScript,
        settings: jaSettings,
        shortlink: jaShortlink,
        subscription: jaSubscription,
        traffic: jaTraffic,
        "vps-asset": jaVpsAsset,
      },
      ko: {
        common: koCommon,
        errors: koErrors,
        auth: koAuth,
        pipeline: koPipeline,
        agent: koAgent,
        audit: koAudit,
        cmdk: koCmdk,
        dashboard: koDashboard,
        node: koNode,
        notify: koNotify,
        rule: koRule,
        "rule-set": koRuleSet,
        "proxy-group": koProxyGroup,
        script: koScript,
        settings: koSettings,
        shortlink: koShortlink,
        subscription: koSubscription,
        traffic: koTraffic,
        "vps-asset": koVpsAsset,
      },
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
