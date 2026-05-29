import { useEffect } from "react";
import { AppState } from "react-native";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAuthStore } from "../stores/auth-store";
import { useThemeStore } from "../stores/theme-store";
import { colors } from "../lib/theme";
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
  const loadFromStorage = useAuthStore((s) => s.loadFromStorage);
  const loadTheme = useThemeStore((s) => s.loadFromStorage);

  useEffect(() => {
    loadFromStorage();
    loadTheme();
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
          headerBackTitle: "返回",
          headerBackButtonMenuEnabled: false,
          contentStyle: { backgroundColor: colors.bg },
          headerShadowVisible: false,
        }}
      >
        <Stack.Screen name="(auth)" options={{ headerShown: false }} />
        <Stack.Screen name="(tabs)" options={{ headerShown: false, title: "" }} />
        <Stack.Screen name="subscription/[id]" options={{ title: "订阅详情" }} />
        <Stack.Screen name="subscription/create" options={{ title: "新建订阅", presentation: "modal" }} />
        <Stack.Screen name="shortlinks" options={{ title: "短链" }} />
        <Stack.Screen name="notifications" options={{ title: "通知" }} />
        <Stack.Screen name="profile" options={{ title: "个人资料" }} />
        <Stack.Screen name="rule-sets" options={{ title: "规则集" }} />
        <Stack.Screen name="vps-asset/create" options={{ title: "新增 VPS", presentation: "modal" }} />
        <Stack.Screen name="vps-asset/ssh" options={{ headerShown: false, orientation: "landscape" }} />
        <Stack.Screen name="subscription/edit" options={{ title: "编辑订阅" }} />
        <Stack.Screen name="vps-asset/edit" options={{ title: "编辑 VPS" }} />
        <Stack.Screen name="admin/users" options={{ title: "用户管理" }} />
        <Stack.Screen name="admin/audit" options={{ title: "审计日志" }} />
        <Stack.Screen name="admin/settings" options={{ title: "系统设置" }} />
        <Stack.Screen name="admin/ota" options={{ title: "OTA 升级" }} />
        <Stack.Screen name="proxy-groups" options={{ title: "代理组" }} />
        <Stack.Screen name="rule/create" options={{ title: "新建规则", presentation: "modal" }} />
        <Stack.Screen name="notification/create" options={{ title: "新建通知渠道", presentation: "modal" }} />
        <Stack.Screen name="pipelines" options={{ title: "流水线" }} />
        <Stack.Screen name="scripts" options={{ title: "脚本" }} />
        <Stack.Screen name="agents-page" options={{ title: "探针" }} />
        <Stack.Screen name="agent/[id]" options={{ title: "探针详情" }} />
        <Stack.Screen name="agent/create" options={{ title: "新建探针", presentation: "modal" }} />
        <Stack.Screen name="agent/edit" options={{ title: "编辑探针" }} />
        <Stack.Screen name="traffic-page" options={{ title: "流量" }} />
        <Stack.Screen name="rules-page" options={{ title: "规则" }} />
        <Stack.Screen name="settings-page" options={{ title: "设置" }} />
      </Stack>
    </QueryClientProvider>
  );
}
