import { View, Text, StyleSheet, TouchableOpacity, Alert } from "react-native";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useAuthStore } from "../../stores/auth-store";
import { colors, spacing, radius, fontSize } from "../../lib/theme";

export default function SettingsScreen() {
  const { user, serverUrl, clearSession } = useAuthStore();

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
    <View style={styles.container}>
      {/* Profile card */}
      <View style={styles.profileCard}>
        <View style={styles.avatar}>
          <Text style={styles.avatarText}>{initials}</Text>
        </View>
        <View>
          <Text style={styles.username}>{user?.username}</Text>
          <Text style={styles.email}>{user?.email || "未设置邮箱"}</Text>
          <Text style={styles.roleBadge}>{user?.role?.toUpperCase()}</Text>
        </View>
      </View>

      {/* Server info */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>服务器</Text>
        <View style={styles.row}>
          <Ionicons name="globe-outline" size={18} color={colors.textTertiary} />
          <Text style={styles.rowText} numberOfLines={1}>{serverUrl}</Text>
        </View>
      </View>

      {/* Features */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>功能</Text>
        <TouchableOpacity style={styles.navRow} onPress={() => router.push("/rule-sets")} activeOpacity={0.6}>
          <Ionicons name="layers-outline" size={18} color={colors.textTertiary} />
          <Text style={styles.rowText}>规则集</Text>
          <Ionicons name="chevron-forward" size={16} color={colors.textDisabled} />
        </TouchableOpacity>
        <TouchableOpacity style={styles.navRow} onPress={() => router.push("/shortlinks")} activeOpacity={0.6}>
          <Ionicons name="link-outline" size={18} color={colors.textTertiary} />
          <Text style={styles.rowText}>短链</Text>
          <Ionicons name="chevron-forward" size={16} color={colors.textDisabled} />
        </TouchableOpacity>
        <TouchableOpacity style={styles.navRow} onPress={() => router.push("/notifications")} activeOpacity={0.6}>
          <Ionicons name="notifications-outline" size={18} color={colors.textTertiary} />
          <Text style={styles.rowText}>通知</Text>
          <Ionicons name="chevron-forward" size={16} color={colors.textDisabled} />
        </TouchableOpacity>
        <TouchableOpacity style={styles.navRow} onPress={() => router.push("/profile")} activeOpacity={0.6}>
          <Ionicons name="person-outline" size={18} color={colors.textTertiary} />
          <Text style={styles.rowText}>个人资料</Text>
          <Ionicons name="chevron-forward" size={16} color={colors.textDisabled} />
        </TouchableOpacity>
      </View>

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
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg, padding: spacing.xl },
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
  navRow: { flexDirection: "row", alignItems: "center", gap: spacing.md, paddingVertical: spacing.sm },
  rowText: { fontSize: fontSize.base, color: colors.textSecondary, flex: 1 },
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
