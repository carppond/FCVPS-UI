import { View, Text, ScrollView, StyleSheet, TouchableOpacity, Image } from "react-native";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useAuthStore } from "../../stores/auth-store";
import { colors, spacing, radius, fontSize, glow } from "../../lib/theme";

const MASCOT = require("../../../assets/login-art.png");

interface NavItem {
  icon: keyof typeof Ionicons.glyphMap;
  iconColor: string;
  iconBg: string;
  label: string;
  route: string;
}

const TOOLS: NavItem[] = [
  { icon: "radio-outline", iconColor: colors.success, iconBg: colors.successBg, label: "探针", route: "/agents-page" },
  { icon: "bar-chart-outline", iconColor: colors.info, iconBg: colors.infoBg, label: "流量", route: "/traffic-page" },
  { icon: "shield-outline", iconColor: colors.primary, iconBg: colors.primarySoft, label: "规则", route: "/rules-page" },
  { icon: "layers-outline", iconColor: colors.warning, iconBg: colors.warningBg, label: "规则集", route: "/rule-sets" },
  { icon: "git-branch-outline", iconColor: "#a78bfa", iconBg: "rgba(167,139,250,0.08)", label: "代理组", route: "/proxy-groups" },
  { icon: "code-slash-outline", iconColor: colors.info, iconBg: colors.infoBg, label: "流水线", route: "/pipelines" },
  { icon: "terminal-outline", iconColor: colors.textSecondary, iconBg: "rgba(0,0,0,0.04)", label: "脚本", route: "/scripts" },
];

const SERVICES: NavItem[] = [
  { icon: "link-outline", iconColor: colors.primary, iconBg: colors.primarySoft, label: "短链", route: "/shortlinks" },
  { icon: "notifications-outline", iconColor: colors.warning, iconBg: colors.warningBg, label: "通知", route: "/notifications" },
];

const ACCOUNT: NavItem[] = [
  { icon: "person-outline", iconColor: colors.info, iconBg: colors.infoBg, label: "个人资料", route: "/profile" },
  { icon: "settings-outline", iconColor: colors.textSecondary, iconBg: "rgba(0,0,0,0.04)", label: "设置", route: "/settings-page" },
];

const ADMIN: NavItem[] = [
  { icon: "people-outline", iconColor: colors.primary, iconBg: colors.primarySoft, label: "用户管理", route: "/admin/users" },
  { icon: "document-text-outline", iconColor: colors.warning, iconBg: colors.warningBg, label: "审计日志", route: "/admin/audit" },
  { icon: "construct-outline", iconColor: colors.textSecondary, iconBg: "rgba(0,0,0,0.04)", label: "系统设置", route: "/admin/settings" },
  { icon: "cloud-download-outline", iconColor: colors.success, iconBg: colors.successBg, label: "OTA 升级", route: "/admin/ota" },
];

export default function MoreScreen() {
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <TouchableOpacity
        style={styles.profile}
        onPress={() => router.push("/profile" as any)}
        activeOpacity={0.7}
      >
        <View style={styles.profileAvatarWrap}>
          <Image source={MASCOT} style={styles.profileAvatarImg} resizeMode="cover" />
        </View>
        <Text style={styles.profileName}>{user?.username ?? "Admin"}</Text>
        <Text style={styles.profileMeta}>
          {isAdmin ? "管理员" : "用户"}
          {user?.totp_enabled ? " · 已启用 2FA" : ""} · Lv.拾光
        </Text>
      </TouchableOpacity>

      <Section title="工具" items={TOOLS} />
      <Section title="服务" items={SERVICES} />
      <Section title="账户" items={ACCOUNT} />
      {isAdmin && <Section title="管理" items={ADMIN} />}
    </ScrollView>
  );
}

function Section({ title, items }: { title: string; items: NavItem[] }) {
  return (
    <View style={styles.section}>
      <Text style={styles.sectionTitle}>{title}</Text>
      <View style={styles.card}>
        {items.map((item, idx) => (
          <TouchableOpacity
            key={item.route}
            style={[styles.row, idx < items.length - 1 && styles.rowBorder]}
            onPress={() => router.push(item.route as any)}
            activeOpacity={0.6}
          >
            <View style={[styles.rowIcon, { backgroundColor: item.iconBg }]}>
              <Ionicons name={item.icon} size={18} color={item.iconColor} />
            </View>
            <Text style={styles.rowLabel}>{item.label}</Text>
            <Ionicons name="chevron-forward" size={16} color={colors.textDisabled} />
          </TouchableOpacity>
        ))}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.xl, paddingBottom: 40 },
  profile: {
    alignItems: "center",
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    paddingVertical: spacing.xl,
    paddingHorizontal: spacing.lg,
    marginBottom: spacing.xl,
  },
  profileAvatarWrap: {
    width: 74,
    height: 74,
    borderRadius: 37,
    overflow: "hidden",
    borderWidth: 2,
    borderColor: colors.primary,
    marginBottom: spacing.md,
    ...glow(colors.primary, 16, 0.45),
  },
  profileAvatarImg: { width: 74, height: 100, marginTop: 2 },
  profileName: { fontSize: fontSize.lg, fontWeight: "800", color: colors.textPrimary },
  profileMeta: { fontSize: fontSize.xs, color: colors.textTertiary, marginTop: 4 },
  section: { marginBottom: spacing.xl },
  sectionTitle: {
    fontSize: fontSize.xs,
    fontWeight: "700",
    color: colors.textDisabled,
    textTransform: "uppercase",
    letterSpacing: 1,
    marginBottom: spacing.sm,
    marginLeft: spacing.xs,
  },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    overflow: "hidden",
  },
  row: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
  },
  rowBorder: {
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  rowIcon: {
    width: 32,
    height: 32,
    borderRadius: radius.md,
    justifyContent: "center",
    alignItems: "center",
  },
  rowLabel: {
    flex: 1,
    fontSize: fontSize.base,
    fontWeight: "500",
    color: colors.textPrimary,
  },
});
