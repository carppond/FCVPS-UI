import { useTranslation } from "react-i18next";
import { Tabs, router } from "expo-router";
import { TouchableOpacity } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { fontSize, spacing } from "../../lib/theme";
import { useColors } from "../../lib/useColors";

export default function TabsLayout() {
  const { t } = useTranslation("nav");
  const colors = useColors();
  return (
    <Tabs
      screenOptions={{
        tabBarStyle: {
          backgroundColor: colors.surface,
          borderTopColor: colors.border,
          borderTopWidth: 1,
          height: 85,
          paddingBottom: 24,
          paddingTop: 8,
        },
        tabBarActiveTintColor: colors.primary,
        tabBarInactiveTintColor: colors.textTertiary,
        tabBarLabelStyle: { fontSize: fontSize.xs, fontWeight: "600" },
        headerStyle: { backgroundColor: colors.surface },
        headerTintColor: colors.textPrimary,
        headerTitleStyle: { fontWeight: "700", fontSize: fontSize.lg },
        headerShadowVisible: false,
      }}
    >
      <Tabs.Screen
        name="index"
        options={{
          title: t("tab_home"),
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="grid-outline" size={size} color={color} />
          ),
        }}
      />
      <Tabs.Screen
        name="subscriptions"
        options={{
          title: t("tab_subs"),
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="book-outline" size={size} color={color} />
          ),
          headerRight: () => (
            <TouchableOpacity
              onPress={() => router.push("/subscription/create")}
              style={{ marginRight: spacing.lg }}
              activeOpacity={0.6}
            >
              <Ionicons name="add-circle-outline" size={24} color={colors.primary} />
            </TouchableOpacity>
          ),
        }}
      />
      <Tabs.Screen
        name="nodes"
        options={{
          title: t("tab_nodes"),
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="server-outline" size={size} color={color} />
          ),
        }}
      />
      <Tabs.Screen
        name="vps-assets"
        options={{
          title: "VPS",
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="hardware-chip-outline" size={size} color={color} />
          ),
          headerRight: () => (
            <TouchableOpacity
              onPress={() => router.push("/vps-asset/create")}
              style={{ marginRight: spacing.lg }}
              activeOpacity={0.6}
            >
              <Ionicons name="add-circle-outline" size={24} color={colors.primary} />
            </TouchableOpacity>
          ),
        }}
      />
      <Tabs.Screen
        name="more"
        options={{
          title: t("tab_more"),
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="menu-outline" size={size} color={color} />
          ),
        }}
      />
      {/* Hidden tabs — accessible as screens but not in tab bar */}
      <Tabs.Screen name="agents" options={{ href: null, title: t("agents") }} />
      <Tabs.Screen name="rules" options={{ href: null, title: t("rules") }} />
      <Tabs.Screen name="traffic" options={{ href: null, title: t("traffic") }} />
      <Tabs.Screen name="settings" options={{ href: null, title: t("settings") }} />
    </Tabs>
  );
}
