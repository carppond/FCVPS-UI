import { useEffect } from "react";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAuthStore } from "../stores/auth-store";
import { useThemeStore } from "../stores/theme-store";
import { colors } from "../lib/theme";

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

  return (
    <QueryClientProvider client={queryClient}>
      <StatusBar style="dark" />
      <Stack
        screenOptions={{
          headerStyle: { backgroundColor: colors.surface },
          headerTintColor: colors.textPrimary,
          headerTitleStyle: { fontWeight: "700" },
          contentStyle: { backgroundColor: colors.bg },
          headerShadowVisible: false,
        }}
      >
        <Stack.Screen name="(auth)" options={{ headerShown: false }} />
        <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
        <Stack.Screen name="subscription/[id]" options={{ title: "订阅详情" }} />
        <Stack.Screen name="subscription/create" options={{ title: "新建订阅", presentation: "modal" }} />
        <Stack.Screen name="shortlinks" options={{ title: "短链" }} />
        <Stack.Screen name="notifications" options={{ title: "通知" }} />
        <Stack.Screen name="profile" options={{ title: "个人资料" }} />
        <Stack.Screen name="rule-sets" options={{ title: "规则集" }} />
        <Stack.Screen name="vps-asset/create" options={{ title: "新增 VPS", presentation: "modal" }} />
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
      </Stack>
    </QueryClientProvider>
  );
}
