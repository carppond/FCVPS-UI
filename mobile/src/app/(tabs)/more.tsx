import { View, Text, ScrollView, StyleSheet, TouchableOpacity, Image } from "react-native";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
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

const buildTools = (c: AppColors, t: TFunction): NavItem[] => [
  { icon: "radio-outline", iconColor: c.success, iconBg: c.successBg, label: t("more_tools_agents"), route: "/agents-page" },
  { icon: "bar-chart-outline", iconColor: c.info, iconBg: c.infoBg, label: t("more_tools_traffic"), route: "/traffic-page" },
  { icon: "shield-outline", iconColor: c.primary, iconBg: c.primarySoft, label: t("more_tools_rules"), route: "/rules-page" },
  { icon: "layers-outline", iconColor: c.warning, iconBg: c.warningBg, label: t("more_tools_rule_sets"), route: "/rule-sets" },
  { icon: "git-branch-outline", iconColor: c.purple, iconBg: "rgba(155,107,255,0.12)", label: t("more_tools_proxy_groups"), route: "/proxy-groups" },
  { icon: "code-slash-outline", iconColor: c.info, iconBg: c.infoBg, label: t("more_tools_pipelines"), route: "/pipelines" },
  { icon: "terminal-outline", iconColor: c.textSecondary, iconBg: c.surfaceHover, label: t("more_tools_scripts"), route: "/scripts" },
];

const buildServices = (c: AppColors, t: TFunction): NavItem[] => [
  { icon: "link-outline", iconColor: c.primary, iconBg: c.primarySoft, label: t("more_services_shortlinks"), route: "/shortlinks" },
  { icon: "notifications-outline", iconColor: c.warning, iconBg: c.warningBg, label: t("more_services_notifications"), route: "/notifications" },
];

const buildAccount = (c: AppColors, t: TFunction): NavItem[] => [
  { icon: "person-outline", iconColor: c.info, iconBg: c.infoBg, label: t("more_account_profile"), route: "/profile" },
  { icon: "settings-outline", iconColor: c.textSecondary, iconBg: c.surfaceHover, label: t("more_account_settings"), route: "/settings-page" },
];

const buildAdmin = (c: AppColors, t: TFunction): NavItem[] => [
  { icon: "people-outline", iconColor: c.primary, iconBg: c.primarySoft, label: t("more_admin_users"), route: "/admin/users" },
  { icon: "document-text-outline", iconColor: c.warning, iconBg: c.warningBg, label: t("more_admin_audit"), route: "/admin/audit" },
  { icon: "construct-outline", iconColor: c.textSecondary, iconBg: c.surfaceHover, label: t("more_admin_settings"), route: "/admin/settings" },
  { icon: "cloud-download-outline", iconColor: c.success, iconBg: c.successBg, label: t("more_admin_ota"), route: "/admin/ota" },
];

export default function MoreScreen() {
  const { t } = useTranslation("settings");
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";

  const tools = useMemo(() => buildTools(colors, t), [colors, t]);
  const services = useMemo(() => buildServices(colors, t), [colors, t]);
  const account = useMemo(() => buildAccount(colors, t), [colors, t]);
  const admin = useMemo(() => buildAdmin(colors, t), [colors, t]);

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
          {isAdmin ? t("role_admin") : t("role_user")}
          {user?.totp_enabled ? t("more_meta_2fa") : ""} · {t("more_meta_level")}
        </Text>
      </TouchableOpacity>

      <Section title={t("more_section_tools")} items={tools} />
      <Section title={t("more_section_services")} items={services} />
      <Section title={t("more_section_account")} items={account} />
      {isAdmin && <Section title={t("more_section_admin")} items={admin} />}
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
