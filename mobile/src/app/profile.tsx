import { useState, useMemo } from "react";
import {
  View,
  Text,
  TextInput,
  ScrollView,
  StyleSheet,
  TouchableOpacity,
  Alert,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useProfileQuery, useUpdateProfile, useChangePassword } from "../api/user";
import { useAuthStore } from "../stores/auth-store";
import { spacing, radius, fontSize, type AppColors } from "../lib/theme";
import { useColors } from "../lib/useColors";

export default function ProfileScreen() {
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const { data: profile, refetch } = useProfileQuery();
  const updateMutation = useUpdateProfile();
  const passwordMutation = useChangePassword();
  const { clearSession } = useAuthStore();

  const [editUsername, setEditUsername] = useState("");
  const [editEmail, setEditEmail] = useState("");
  const [editLocale, setEditLocale] = useState("");
  const [profileDirty, setProfileDirty] = useState(false);

  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const LOCALE_OPTIONS = [
    { value: "zh-CN", label: "中文" },
    { value: "en", label: "English" },
    { value: "ja", label: "日本語" },
    { value: "ko", label: "한국어" },
  ];

  // Sync profile data into edit fields when loaded
  if (profile && !profileDirty && editUsername === "" && editEmail === "" && editLocale === "") {
    setEditUsername(profile.username ?? "");
    setEditEmail(profile.email ?? "");
    setEditLocale(profile.locale ?? "zh-CN");
  }

  const handleProfileChange = (field: "username" | "email" | "locale", value: string) => {
    setProfileDirty(true);
    if (field === "username") setEditUsername(value);
    else if (field === "email") setEditEmail(value);
    else setEditLocale(value);
  };

  const handleSaveProfile = () => {
    const payload: { username?: string; email?: string; locale?: string } = {};
    if (editUsername.trim() && editUsername !== profile?.username) {
      payload.username = editUsername.trim();
    }
    if (editEmail !== (profile?.email ?? "")) {
      payload.email = editEmail.trim();
    }
    if (editLocale !== profile?.locale) {
      payload.locale = editLocale;
    }
    if (Object.keys(payload).length === 0) {
      Alert.alert("提示", "没有需要保存的修改");
      return;
    }
    updateMutation.mutate(payload, {
      onSuccess: () => {
        Alert.alert("成功", "个人信息已更新");
        setProfileDirty(false);
        refetch();
      },
      onError: (err: any) => Alert.alert("更新失败", err.message),
    });
  };

  const initials = profile?.username?.slice(0, 2).toUpperCase() ?? "??";

  const handleChangePassword = () => {
    if (!oldPassword.trim()) {
      Alert.alert("提示", "请输入旧密码");
      return;
    }
    if (!newPassword.trim()) {
      Alert.alert("提示", "请输入新密码");
      return;
    }
    if (newPassword !== confirmPassword) {
      Alert.alert("提示", "两次输入的新密码不一致");
      return;
    }
    passwordMutation.mutate(
      { old_password: oldPassword, new_password: newPassword },
      {
        onSuccess: () => {
          Alert.alert("成功", "密码已修改");
          setOldPassword("");
          setNewPassword("");
          setConfirmPassword("");
        },
        onError: (err: any) => Alert.alert("修改失败", err.message),
      },
    );
  };

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

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <ScrollView contentContainerStyle={styles.content}>
        {/* Avatar card */}
        <View style={styles.profileCard}>
          <View style={styles.avatar}>
            <Text style={styles.avatarText}>{initials}</Text>
          </View>
          <View style={styles.profileInfo}>
            <Text style={styles.username}>{profile?.username ?? "--"}</Text>
            <Text style={styles.email}>
              {profile?.email || "未设置邮箱"}
            </Text>
            <View style={styles.badgeRow}>
              <Text style={styles.roleBadge}>
                {profile?.role?.toUpperCase() ?? "--"}
              </Text>
              <Text style={styles.localeBadge}>
                {profile?.locale ?? "--"}
              </Text>
            </View>
          </View>
        </View>

        {/* Profile info card - editable */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View
              style={[styles.cardIcon, { backgroundColor: colors.infoBg }]}
            >
              <Ionicons name="person-outline" size={16} color={colors.info} />
            </View>
            <Text style={styles.cardTitle}>账号信息</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>用户名</Text>
            <TextInput
              style={styles.input}
              value={editUsername}
              onChangeText={(v) => handleProfileChange("username", v)}
              placeholder="请输入用户名"
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>邮箱</Text>
            <TextInput
              style={styles.input}
              value={editEmail}
              onChangeText={(v) => handleProfileChange("email", v)}
              placeholder="请输入邮箱"
              placeholderTextColor={colors.textDisabled}
              keyboardType="email-address"
              autoCapitalize="none"
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>语言</Text>
            <View style={styles.localeRow}>
              {LOCALE_OPTIONS.map((opt) => (
                <TouchableOpacity
                  key={opt.value}
                  style={[styles.localeChip, editLocale === opt.value && styles.localeChipActive]}
                  onPress={() => handleProfileChange("locale", opt.value)}
                  activeOpacity={0.7}
                >
                  <Text style={[styles.localeChipText, editLocale === opt.value && styles.localeChipTextActive]}>
                    {opt.label}
                  </Text>
                </TouchableOpacity>
              ))}
            </View>
          </View>
          <View style={styles.infoRow}>
            <Text style={styles.infoLabel}>角色</Text>
            <Text style={styles.infoValue}>
              {profile?.role ?? "--"}
            </Text>
          </View>
          <View style={[styles.infoRow, { borderBottomWidth: 0 }]}>
            <Text style={styles.infoLabel}>两步验证</Text>
            <Text
              style={[
                styles.infoValue,
                {
                  color: profile?.totp_enabled
                    ? colors.success
                    : colors.textTertiary,
                },
              ]}
            >
              {profile?.totp_enabled ? "已开启" : "未开启"}
            </Text>
          </View>
          <TouchableOpacity
            style={[styles.saveBtn, updateMutation.isPending && styles.submitBtnDisabled]}
            onPress={handleSaveProfile}
            disabled={updateMutation.isPending}
            activeOpacity={0.8}
          >
            <Text style={styles.saveBtnText}>
              {updateMutation.isPending ? "保存中..." : "保存修改"}
            </Text>
          </TouchableOpacity>
        </View>

        {/* Change password */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View
              style={[
                styles.cardIcon,
                { backgroundColor: colors.warningBg },
              ]}
            >
              <Ionicons
                name="lock-closed-outline"
                size={16}
                color={colors.warning}
              />
            </View>
            <Text style={styles.cardTitle}>修改密码</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>
              旧密码 <Text style={styles.required}>*</Text>
            </Text>
            <TextInput
              style={styles.input}
              value={oldPassword}
              onChangeText={setOldPassword}
              placeholder="请输入旧密码"
              placeholderTextColor={colors.textDisabled}
              secureTextEntry
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>
              新密码 <Text style={styles.required}>*</Text>
            </Text>
            <TextInput
              style={styles.input}
              value={newPassword}
              onChangeText={setNewPassword}
              placeholder="请输入新密码"
              placeholderTextColor={colors.textDisabled}
              secureTextEntry
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>
              确认新密码 <Text style={styles.required}>*</Text>
            </Text>
            <TextInput
              style={styles.input}
              value={confirmPassword}
              onChangeText={setConfirmPassword}
              placeholder="再次输入新密码"
              placeholderTextColor={colors.textDisabled}
              secureTextEntry
            />
          </View>
          <TouchableOpacity
            style={[
              styles.submitBtn,
              passwordMutation.isPending && styles.submitBtnDisabled,
            ]}
            onPress={handleChangePassword}
            disabled={passwordMutation.isPending}
            activeOpacity={0.8}
          >
            <Text style={styles.submitText}>
              {passwordMutation.isPending ? "修改中..." : "修改密码"}
            </Text>
          </TouchableOpacity>
        </View>

        {/* Logout */}
        <TouchableOpacity
          style={styles.logoutBtn}
          onPress={handleLogout}
          activeOpacity={0.7}
        >
          <Ionicons name="log-out-outline" size={18} color={colors.error} />
          <Text style={styles.logoutText}>退出登录</Text>
        </TouchableOpacity>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.xl, paddingBottom: 40 },
  profileCard: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.lg,
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.xl,
    marginBottom: spacing.lg,
  },
  avatar: {
    width: 56,
    height: 56,
    borderRadius: 28,
    backgroundColor: colors.primarySoft,
    justifyContent: "center",
    alignItems: "center",
  },
  avatarText: {
    fontSize: fontSize.xl,
    fontWeight: "800",
    color: colors.primary,
  },
  profileInfo: { flex: 1 },
  username: {
    fontSize: fontSize.lg,
    fontWeight: "700",
    color: colors.textPrimary,
  },
  email: {
    fontSize: fontSize.sm,
    color: colors.textTertiary,
    marginTop: 2,
  },
  badgeRow: {
    flexDirection: "row",
    gap: spacing.sm,
    marginTop: spacing.xs,
  },
  roleBadge: {
    fontSize: fontSize.xs,
    fontWeight: "700",
    color: colors.primary,
    backgroundColor: colors.primarySoft,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: radius.sm,
  },
  localeBadge: {
    fontSize: fontSize.xs,
    fontWeight: "700",
    color: colors.info,
    backgroundColor: colors.infoBg,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: radius.sm,
  },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.xl,
    marginBottom: spacing.lg,
  },
  cardHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
    marginBottom: spacing.lg,
  },
  cardIcon: {
    width: 28,
    height: 28,
    borderRadius: radius.md,
    justifyContent: "center",
    alignItems: "center",
  },
  cardTitle: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
  },
  infoRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    paddingVertical: spacing.md,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  infoLabel: {
    fontSize: fontSize.sm,
    color: colors.textTertiary,
  },
  infoValue: {
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.textSecondary,
  },
  field: { gap: spacing.xs, marginBottom: spacing.md },
  label: {
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.textSecondary,
  },
  required: { color: colors.primary, fontSize: fontSize.xs },
  input: {
    height: 48,
    borderRadius: radius.lg,
    borderWidth: 1,
    borderColor: colors.borderStrong,
    backgroundColor: colors.elevated,
    paddingHorizontal: spacing.lg,
    fontSize: fontSize.base,
    color: colors.textPrimary,
  },
  submitBtn: {
    height: 50,
    borderRadius: radius.lg,
    backgroundColor: colors.primary,
    justifyContent: "center",
    alignItems: "center",
    marginTop: spacing.sm,
  },
  submitBtnDisabled: { opacity: 0.5 },
  submitText: { fontSize: fontSize.lg, fontWeight: "700", color: "#fff" },
  localeRow: {
    flexDirection: "row", flexWrap: "wrap", gap: spacing.sm,
  },
  localeChip: {
    paddingHorizontal: spacing.md, paddingVertical: spacing.sm,
    borderRadius: radius.lg, backgroundColor: colors.elevated,
    borderWidth: 1, borderColor: colors.borderStrong,
  },
  localeChipActive: {
    backgroundColor: colors.primary, borderColor: colors.primary,
  },
  localeChipText: {
    fontSize: fontSize.sm, fontWeight: "600", color: colors.textTertiary,
  },
  localeChipTextActive: {
    color: "#fff",
  },
  saveBtn: {
    height: 50,
    borderRadius: radius.lg,
    backgroundColor: colors.info,
    justifyContent: "center",
    alignItems: "center",
    marginTop: spacing.md,
  },
  saveBtnText: { fontSize: fontSize.lg, fontWeight: "700", color: "#fff" },
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
    marginTop: spacing.sm,
  },
  logoutText: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.error,
  },
});
