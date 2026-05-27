import { useEffect } from "react";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAuthStore } from "../stores/auth-store";
import { colors } from "../lib/theme";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, retry: 1 },
  },
});

export default function RootLayout() {
  const loadFromStorage = useAuthStore((s) => s.loadFromStorage);

  useEffect(() => {
    loadFromStorage();
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
      </Stack>
    </QueryClientProvider>
  );
}
