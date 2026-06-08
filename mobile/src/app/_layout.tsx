import { useEffect, useMemo } from "react";
import { AppState } from "react-native";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import "../lib/i18n"; // side-effect: 初始化 i18next(必须在任何 useTranslation 之前)
import { useAuthStore } from "../stores/auth-store";
import { useThemeStore } from "../stores/theme-store";
import { useLocaleStore } from "../stores/locale-store";
import { type AppColors } from "../lib/theme";
import { useColors } from "../lib/useColors";
import { trafficSummaryQuery } from "../api/traffic";
// Side-effect import: loading widget-sync runs createWidget() so the home-screen
// widget's layout is registered with native at app startup (not only when the
// Traffic tab mounts). No-ops safely when the native module is absent (Expo Go).
import { pushTrafficToWidget, isTrafficWidgetAvailable } from "../lib/widget-sync";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, retry: 1 },
  },
});

export default function RootLayout() {
  const { t } = useTranslation("nav");
  const colors = useColors();
  const loadFromStorage = useAuthStore((s) => s.loadFromStorage);
  const loadTheme = useThemeStore((s) => s.loadFromStorage);
  const loadLocale = useLocaleStore((s) => s.loadFromStorage);

  useEffect(() => {
    loadFromStorage();
    loadTheme();
    loadLocale();
  }, []);

  // Refresh the home-screen widget whenever the app returns to the foreground:
  // re-fetch traffic (respecting staleTime) and push it. Keeps the widget fresh
  // on reopen without relying on background tasks or the Traffic tab being open.
  // Skipped when logged out; failures are swallowed (widget is best-effort).
  useEffect(() => {
    const sub = AppState.addEventListener("change", (state) => {
      if (state !== "active" || !useAuthStore.getState().token) return;
      if (!isTrafficWidgetAvailable()) return; // no widget runtime → don't fetch
      queryClient
        .fetchQuery(trafficSummaryQuery)
        .then(pushTrafficToWidget)
        .catch(() => {});
    });
    return () => sub.remove();
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      <StatusBar style="dark" />
      <Stack
        screenOptions={{
          headerStyle: { backgroundColor: colors.surface },
          headerTintColor: colors.textPrimary,
          headerTitleStyle: { fontWeight: "700" },
          headerBackTitle: t("back"),
          headerBackButtonMenuEnabled: false,
          contentStyle: { backgroundColor: colors.bg },
          headerShadowVisible: false,
        }}
      >
        <Stack.Screen name="(auth)" options={{ headerShown: false }} />
        <Stack.Screen name="(tabs)" options={{ headerShown: false, title: "" }} />
        <Stack.Screen name="subscription/[id]" options={{ title: t("sub_detail") }} />
        <Stack.Screen name="subscription/create" options={{ title: t("sub_create"), presentation: "modal" }} />
        <Stack.Screen name="shortlinks" options={{ title: t("shortlinks") }} />
        <Stack.Screen name="notifications" options={{ title: t("notifications") }} />
        <Stack.Screen name="profile" options={{ title: t("profile") }} />
        <Stack.Screen name="rule-sets" options={{ title: t("rule_sets") }} />
        <Stack.Screen name="vps-asset/create" options={{ title: t("vps_create"), presentation: "modal" }} />
        <Stack.Screen name="vps-asset/ssh" options={{ headerShown: false, orientation: "landscape" }} />
        <Stack.Screen name="subscription/edit" options={{ title: t("sub_edit") }} />
        <Stack.Screen name="vps-asset/edit" options={{ title: t("vps_edit") }} />
        <Stack.Screen name="admin/users" options={{ title: t("admin_users") }} />
        <Stack.Screen name="admin/audit" options={{ title: t("admin_audit") }} />
        <Stack.Screen name="admin/settings" options={{ title: t("admin_settings") }} />
        <Stack.Screen name="admin/ota" options={{ title: t("admin_ota") }} />
        <Stack.Screen name="proxy-groups" options={{ title: t("proxy_groups") }} />
        <Stack.Screen name="rule/create" options={{ title: t("rule_create"), presentation: "modal" }} />
        <Stack.Screen name="notification/create" options={{ title: t("notify_create"), presentation: "modal" }} />
        <Stack.Screen name="pipelines" options={{ title: t("pipelines") }} />
        <Stack.Screen name="scripts" options={{ title: t("scripts") }} />
        <Stack.Screen name="agents-page" options={{ title: t("agents") }} />
        <Stack.Screen name="alert-rules" options={{ title: t("alert_rules") }} />
        <Stack.Screen name="agent/[id]" options={{ title: t("agent_detail") }} />
        <Stack.Screen name="agent/create" options={{ title: t("agent_create"), presentation: "modal" }} />
        <Stack.Screen name="agent/edit" options={{ title: t("agent_edit") }} />
        <Stack.Screen name="traffic-page" options={{ title: t("traffic") }} />
        <Stack.Screen name="rules-page" options={{ title: t("rules") }} />
        <Stack.Screen name="settings-page" options={{ title: t("settings") }} />
      </Stack>
    </QueryClientProvider>
  );
}
