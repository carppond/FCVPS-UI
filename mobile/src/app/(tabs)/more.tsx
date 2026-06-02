import { View, Text, ScrollView, StyleSheet, TouchableOpacity, Image } from "react-native";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useMemo } from "react";
import { useAuthStore } from "../../stores/auth-store";
import { spacing, radius, fontSize, glow, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";

const MASCOT = require("../../../assets/login-art.png");

interface NavItem {
  icon: keyof typeof Ionicons.glyphMap;
  iconColor: string;
  iconBg: string;
  label: string;
  route: string;
}

const buildTools = (c: AppColors): NavItem[] => [
  { icon: "radio-outline", iconColor: c.success, iconBg: c.successBg, label: "探针", route: "/agents-page" },
  { icon: "bar-chart-outline", iconColor: c.info, iconBg: c.infoBg, label: "流量", route: "/traffic-page" },
  { icon: "shield-outline", iconColor: c.primary, iconBg: c.primarySoft, label: "规则", route: "/rules-page" },
  { icon: "layers-outline", iconColor: c.warning, iconBg: c.warningBg, label: "规则集", route: "/rule-sets" },
  { icon: "git-branch-outline", iconColor: c.purple, iconBg: "rgba(155,107,255,0.12)", label: "代理组", route: "/proxy-groups" },
  { icon: "code-slash-outline", iconColor: c.info, iconBg: c.infoBg, label: "流水线", route: "/pipelines" },
  { icon: "terminal-outline", iconColor: c.textSecondary, iconBg: c.surfaceHover, label: "脚本", route: "/scripts" },
];

const buildServices = (c: AppColors): NavItem[] => [
  { icon: "link-outline", iconColor: c.primary, iconBg: c.primarySoft, label: "短链", route: "/shortlinks" },
  { icon: "notifications-outline", iconColor: c.warning, iconBg: c.warningBg, label: "通知", route: "/notifications" },
];

const buildAccount = (c: AppColors): NavItem[] => [
  { icon: "person-outline", iconColor: c.info, iconBg: c.infoBg, label: "个人资料", route: "/profile" },
  { icon: "settings-outline", iconColor: c.textSecondary, iconBg: c.surfaceHover, label: "设置", route: "/settings-page" },
];

const buildAdmin = (c: AppColors): NavItem[] => [
  { icon: "people-outline", iconColor: c.primary, iconBg: c.primarySoft, label: "用户管理", route: "/admin/users" },
  { icon: "document-text-outline", iconColor: c.warning, iconBg: c.warningBg, label: "审计日志", route: "/admin/audit" },
  { icon: "construct-outline", iconColor: c.textSecondary, iconBg: c.surfaceHover, label: "系统设置", route: "/admin/settings" },
  { icon: "cloud-download-outline", iconColor: c.success, iconBg: c.successBg, label: "OTA 升级", route: "/admin/ota" },
];

export default function MoreScreen() {
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
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

      <Section title="工具" items={buildTools(colors)} />
      <Section title="服务" items={buildServices(colors)} />
      <Section title="账户" items={buildAccount(colors)} />
      {isAdmin && <Section title="管理" items={buildAdmin(colors)} />}
    </ScrollView>
  );
}

function Section({ title, items }: { title: string; items: NavItem[] }) {
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
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

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
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
