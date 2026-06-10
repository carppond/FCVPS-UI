import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import * as Localization from "expo-localization";

// 命名空间 JSON 同步打包进 bundle(移动端不做按需加载,体积可控)。
// 新增命名空间时:两种语言都要加,并在下面 resources 里注册。
import zhCommon from "../locales/zh-CN/common.json";
import enCommon from "../locales/en/common.json";
import zhNav from "../locales/zh-CN/nav.json";
import enNav from "../locales/en/nav.json";
import zhAuth from "../locales/zh-CN/auth.json";
import enAuth from "../locales/en/auth.json";
import zhDashboard from "../locales/zh-CN/dashboard.json";
import enDashboard from "../locales/en/dashboard.json";
import zhSubscription from "../locales/zh-CN/subscription.json";
import enSubscription from "../locales/en/subscription.json";
import zhNodes from "../locales/zh-CN/nodes.json";
import enNodes from "../locales/en/nodes.json";
import zhAgents from "../locales/zh-CN/agents.json";
import enAgents from "../locales/en/agents.json";
import zhVps from "../locales/zh-CN/vps.json";
import enVps from "../locales/en/vps.json";
import zhTraffic from "../locales/zh-CN/traffic.json";
import enTraffic from "../locales/en/traffic.json";
import zhRules from "../locales/zh-CN/rules.json";
import enRules from "../locales/en/rules.json";
import zhNotify from "../locales/zh-CN/notify.json";
import enNotify from "../locales/en/notify.json";
import zhSettings from "../locales/zh-CN/settings.json";
import enSettings from "../locales/en/settings.json";
import zhAlert from "../locales/zh-CN/alert.json";
import enAlert from "../locales/en/alert.json";
import zhErrors from "../locales/zh-CN/errors.json";
import enErrors from "../locales/en/errors.json";

export type AppLanguage = "zh-CN" | "en";
export type LanguagePreference = AppLanguage | "system";

/** 把系统语言解析成受支持的语言(中文 → zh-CN,其余 → en)。 */
export function resolveSystemLanguage(): AppLanguage {
  const code = Localization.getLocales()[0]?.languageCode ?? "en";
  return code === "zh" ? "zh-CN" : "en";
}

export function resolveLanguage(pref: LanguagePreference): AppLanguage {
  return pref === "system" ? resolveSystemLanguage() : pref;
}

void i18n.use(initReactI18next).init({
  resources: {
    "zh-CN": {
      common: zhCommon,
      nav: zhNav,
      auth: zhAuth,
      dashboard: zhDashboard,
      subscription: zhSubscription,
      nodes: zhNodes,
      agents: zhAgents,
      vps: zhVps,
      traffic: zhTraffic,
      rules: zhRules,
      notify: zhNotify,
      settings: zhSettings,
      alert: zhAlert,
      errors: zhErrors,
    },
    en: {
      common: enCommon,
      nav: enNav,
      auth: enAuth,
      dashboard: enDashboard,
      subscription: enSubscription,
      nodes: enNodes,
      agents: enAgents,
      vps: enVps,
      traffic: enTraffic,
      rules: enRules,
      notify: enNotify,
      settings: enSettings,
      alert: enAlert,
      errors: enErrors,
    },
  },
  // 默认中文(主要用户群);英文/跟随系统在「设置 → 语言」里手动选
  lng: "zh-CN",
  fallbackLng: "zh-CN",
  defaultNS: "common",
  interpolation: { escapeValue: false }, // RN 无 XSS 面,关闭转义
  returnNull: false,
});

export default i18n;
