import { useState, useEffect, useCallback } from "react";
import { View, Text, ScrollView, StyleSheet, TouchableOpacity, Alert, Switch, Platform } from "react-native";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useAuthStore } from "../../stores/auth-store";
import { useThemeStore } from "../../stores/theme-store";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import { mintWidgetToken, revokeWidgetToken, getWidgetTokenStatus } from "../../api/widget";
import { isWidgetSupported, saveWidgetData, clearWidgetData, reloadWidget } from "../../lib/widget-bridge";

export default function SettingsScreen() {
  const { user, serverUrl, clearSession } = useAuthStore();
  const themeMode = useThemeStore((s) => s.mode);
  const toggleTheme = useThemeStore((s) => s.toggle);

  // Home-screen traffic widget (iOS, custom dev/prod build only). The toggle
  // mints a read-only token and writes {serverUrl, token} into the App Group
  // so the native widget can fetch traffic; turning it off revokes the token.
  const widgetSupported = Platform.OS === "ios" && isWidgetSupported();
  const [widgetOn, setWidgetOn] = useState(false);
  const [widgetBusy, setWidgetBusy] = useState(false);

  useEffect(() => {
    if (!widgetSupported) return;
    getWidgetTokenStatus()
      .then((s) => setWidgetOn(s.enabled))
      .catch(() => {});
  }, [widgetSupported]);

  const toggleWidget = useCallback(
    async (next: boolean) => {
      if (widgetBusy) return;
      setWidgetBusy(true);
      try {
        if (next) {
          const { token } = await mintWidgetToken();
          saveWidgetData(serverUrl, token);
          reloadWidget();
          setWidgetOn(true);
        } else {
          await revokeWidgetToken();
          clearWidgetData();
          reloadWidget();
          setWidgetOn(false);
        }
      } catch {
        Alert.alert("操作失败", "请检查网络后重试");
      } finally {
        setWidgetBusy(false);
      }
    },
    [widgetBusy, serverUrl],
  );

  const handleLogout = () => {
    Alert.alert("退出登录", "确定要退出吗？", [
      { text: "取消", style: "cancel" },
      {
        text: "退出",
        style: "destructive",
        onPress: async () => {
          await clearSession();
          router.replace("/(auth)/login");
        },
      },
    ]);
  };

  const initials = user?.username?.slice(0, 2).toUpperCase() ?? "??";

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.scrollContent}>
      {/* Profile card */}
      <TouchableOpacity
        style={styles.profileCard}
        onPress={() => router.push("/profile")}
        activeOpacity={0.7}
      >
        <View style={styles.avatar}>
          <Text style={styles.avatarText}>{initials}</Text>
        </View>
        <View style={{ flex: 1 }}>
          <Text style={styles.username}>{user?.username}</Text>
          <Text style={styles.email}>{user?.email || "未设置邮箱"}</Text>
          <Text style={styles.roleBadge}>{user?.role?.toUpperCase()}</Text>
        </View>
        <Ionicons name="chevron-forward" size={18} color={colors.textDisabled} />
      </TouchableOpacity>

      {/* Server info */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>服务器</Text>
        <View style={styles.row}>
          <Ionicons name="globe-outline" size={18} color={colors.textTertiary} />
          <Text style={styles.rowText} numberOfLines={1}>{serverUrl}</Text>
        </View>
      </View>

      {/* Appearance */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>外观</Text>
        <View style={styles.switchRow}>
          <Ionicons name="moon-outline" size={18} color={colors.textTertiary} />
          <Text style={styles.rowText}>深色模式</Text>
          <Switch
            value={themeMode === "dark"}
            onValueChange={toggleTheme}
            trackColor={{ false: colors.border, true: colors.primarySoft }}
            thumbColor={themeMode === "dark" ? colors.primary : colors.textDisabled}
          />
        </View>
      </View>

      {/* Home-screen widget (iOS only) */}
      {widgetSupported && (
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>桌面小组件</Text>
          <View style={styles.switchRow}>
            <Ionicons name="speedometer-outline" size={18} color={colors.textTertiary} />
            <Text style={styles.rowText}>流量小组件</Text>
            <Switch
              value={widgetOn}
              onValueChange={toggleWidget}
              disabled={widgetBusy}
              trackColor={{ false: colors.border, true: colors.primarySoft }}
              thumbColor={widgetOn ? colors.primary : colors.textDisabled}
            />
          </View>
          <Text style={styles.hint}>
            开启后可在桌面长按添加"拾光VPS"小组件，约 15 分钟自动刷新，打开 App 时立即刷新。
          </Text>
        </View>
      )}

      {/* About */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>关于</Text>
        <View style={styles.row}>
          <Ionicons name="information-circle-outline" size={18} color={colors.textTertiary} />
          <Text style={styles.rowText}>拾光VPS v1.0.0</Text>
        </View>
      </View>

      {/* Logout */}
      <TouchableOpacity style={styles.logoutBtn} onPress={handleLogout} activeOpacity={0.7}>
        <Ionicons name="log-out-outline" size={18} color={colors.error} />
        <Text style={styles.logoutText}>退出登录</Text>
      </TouchableOpacity>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  scrollContent: { padding: spacing.xl, paddingBottom: 40 },
  profileCard: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.lg,
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.xl,
    marginBottom: spacing.xl,
  },
  avatar: {
    width: 56,
    height: 56,
    borderRadius: 28,
    backgroundColor: colors.primarySoft,
    justifyContent: "center",
    alignItems: "center",
  },
  avatarText: { fontSize: fontSize.xl, fontWeight: "800", color: colors.primary },
  username: { fontSize: fontSize.lg, fontWeight: "700", color: colors.textPrimary },
  email: { fontSize: fontSize.sm, color: colors.textTertiary, marginTop: 2 },
  roleBadge: {
    fontSize: fontSize.xs,
    fontWeight: "700",
    color: colors.primary,
    backgroundColor: colors.primarySoft,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: radius.sm,
    alignSelf: "flex-start",
    marginTop: spacing.xs,
  },
  section: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.md,
  },
  sectionTitle: {
    fontSize: fontSize.xs,
    fontWeight: "700",
    color: colors.textDisabled,
    letterSpacing: 1,
    marginBottom: spacing.md,
  },
  row: { flexDirection: "row", alignItems: "center", gap: spacing.md },
  switchRow: { flexDirection: "row", alignItems: "center", gap: spacing.md },
  rowText: { fontSize: fontSize.base, color: colors.textSecondary, flex: 1 },
  hint: { fontSize: fontSize.xs, color: colors.textDisabled, marginTop: spacing.sm, lineHeight: 16 },
  logoutBtn: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.sm,
    backgroundColor: colors.errorBg,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: "rgba(248,113,113,0.15)",
    padding: spacing.lg,
    marginTop: spacing.xl,
  },
  logoutText: { fontSize: fontSize.base, fontWeight: "700", color: colors.error },
});
