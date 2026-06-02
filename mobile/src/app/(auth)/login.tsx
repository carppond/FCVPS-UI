import { useState, useEffect, useMemo } from "react";
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  StyleSheet,
  Alert,
  Image,
  ScrollView,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { LinearGradient } from "expo-linear-gradient";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import * as SecureStore from "expo-secure-store";
import { useLoginMutation } from "../../api/auth";
import { useAuthStore } from "../../stores/auth-store";
import { STORAGE_KEYS } from "../../lib/constants";
import { spacing, radius, fontSize, gradients, glow, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";

const MASCOT = require("../../../assets/login-art.png");

export default function LoginScreen() {
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const [serverUrl, setServerUrl] = useState(useAuthStore.getState().serverUrl);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [showServer, setShowServer] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(true);
  const loginMutation = useLoginMutation();
  const setServerUrlStore = useAuthStore((s) => s.setServerUrl);

  // Load saved credentials on mount (fill form only, no auto-login)
  useEffect(() => {
    (async () => {
      const [savedUser, savedPass, savedServer] = await Promise.all([
        SecureStore.getItemAsync(STORAGE_KEYS.SAVED_USERNAME),
        SecureStore.getItemAsync(STORAGE_KEYS.SAVED_PASSWORD),
        SecureStore.getItemAsync(STORAGE_KEYS.SERVER_URL),
      ]);
      if (savedServer) setServerUrl(savedServer);
      if (savedUser) setUsername(savedUser);
      if (savedPass) setPassword(savedPass);
    })();
  }, []);

  const handleLogin = async () => {
    if (!username.trim() || !password.trim()) {
      Alert.alert("提示", "请输入用户名和密码");
      return;
    }
    try {
      await setServerUrlStore(serverUrl);
      await loginMutation.mutateAsync({ username, password });

      if (rememberMe) {
        await SecureStore.setItemAsync(STORAGE_KEYS.SAVED_USERNAME, username);
        await SecureStore.setItemAsync(STORAGE_KEYS.SAVED_PASSWORD, password);
      } else {
        await SecureStore.deleteItemAsync(STORAGE_KEYS.SAVED_USERNAME);
        await SecureStore.deleteItemAsync(STORAGE_KEYS.SAVED_PASSWORD);
      }

      router.replace("/(tabs)");
    } catch (err: any) {
      Alert.alert("登录失败", err.message || "请检查用户名和密码");
    }
  };

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      {/* ambient glow */}
      <LinearGradient
        colors={gradients.heroGlow}
        start={{ x: 0.8, y: 0 }}
        end={{ x: 0.2, y: 0.7 }}
        style={styles.glow}
        pointerEvents="none"
      />
      <ScrollView
        contentContainerStyle={styles.scroll}
        keyboardShouldPersistTaps="handled"
        showsVerticalScrollIndicator={false}
      >
        <View style={styles.hero}>
          <Image source={MASCOT} style={styles.mascot} resizeMode="contain" />
        </View>

        <View style={styles.form}>
          <Text style={styles.title}>欢迎回来 ♡</Text>
          <Text style={styles.subtitle}>拾光娘在等你登录呢～</Text>

          <View style={styles.field}>
            <Text style={styles.label}>用户名</Text>
            <TextInput
              style={styles.input}
              value={username}
              onChangeText={setUsername}
              placeholder="admin"
              placeholderTextColor={colors.textDisabled}
              autoCapitalize="none"
              autoCorrect={false}
            />
          </View>

          <View style={styles.field}>
            <Text style={styles.label}>密码</Text>
            <View style={styles.passwordWrap}>
              <TextInput
                style={styles.passwordInput}
                value={password}
                onChangeText={setPassword}
                placeholder="••••••••"
                placeholderTextColor={colors.textDisabled}
                secureTextEntry={!showPassword}
              />
              <TouchableOpacity
                style={styles.eyeBtn}
                onPress={() => setShowPassword(!showPassword)}
                activeOpacity={0.6}
              >
                <Ionicons
                  name={showPassword ? "eye-outline" : "eye-off-outline"}
                  size={20}
                  color={colors.textTertiary}
                />
              </TouchableOpacity>
            </View>
          </View>

          <TouchableOpacity
            style={styles.rememberRow}
            onPress={() => setRememberMe(!rememberMe)}
            activeOpacity={0.6}
          >
            <Ionicons
              name={rememberMe ? "checkbox" : "square-outline"}
              size={20}
              color={rememberMe ? colors.primary : colors.textTertiary}
            />
            <Text style={styles.rememberText}>记住账号密码</Text>
          </TouchableOpacity>

          <TouchableOpacity
            onPress={handleLogin}
            disabled={loginMutation.isPending}
            activeOpacity={0.85}
            style={[glow(colors.primary), { marginTop: spacing.sm }]}
          >
            <LinearGradient
              colors={gradients.primary}
              start={{ x: 0, y: 0 }}
              end={{ x: 1, y: 1 }}
              style={[styles.button, loginMutation.isPending && styles.buttonDisabled]}
            >
              <Text style={styles.buttonText}>
                {loginMutation.isPending ? "登录中..." : "登录"}
              </Text>
            </LinearGradient>
          </TouchableOpacity>

          <TouchableOpacity onPress={() => setShowServer(!showServer)} activeOpacity={0.6}>
            <Text style={styles.serverToggle}>
              {showServer ? "收起服务器设置 ▲" : "服务器设置 ▼"}
            </Text>
          </TouchableOpacity>

          {showServer && (
            <View style={styles.field}>
              <Text style={styles.label}>服务器地址</Text>
              <TextInput
                style={styles.input}
                value={serverUrl}
                onChangeText={setServerUrl}
                placeholder="https://your-hub.example.com"
                placeholderTextColor={colors.textDisabled}
                autoCapitalize="none"
                autoCorrect={false}
                keyboardType="url"
              />
            </View>
          )}
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  glow: { position: "absolute", top: 0, left: 0, right: 0, height: 360 },
  scroll: { flexGrow: 1, justifyContent: "center", paddingBottom: spacing.xxl },
  hero: { height: 280, alignItems: "center", justifyContent: "flex-end" },
  mascot: { height: 280, width: "100%" },
  form: { paddingHorizontal: spacing.xxxl, gap: spacing.md, marginTop: -spacing.lg },
  title: { fontSize: fontSize.xxl, fontWeight: "800", color: colors.textPrimary, letterSpacing: -0.5 },
  subtitle: { fontSize: fontSize.sm, color: colors.textTertiary, marginTop: spacing.xs, marginBottom: spacing.sm },
  field: { gap: spacing.xs },
  label: { fontSize: fontSize.xs, fontWeight: "600", color: colors.textTertiary },
  input: {
    height: 46,
    borderRadius: radius.md,
    borderWidth: 1,
    borderColor: colors.borderStrong,
    backgroundColor: colors.surface,
    paddingHorizontal: spacing.lg,
    fontSize: fontSize.base,
    color: colors.textPrimary,
  },
  passwordWrap: {
    flexDirection: "row",
    alignItems: "center",
    borderRadius: radius.md,
    borderWidth: 1,
    borderColor: colors.borderStrong,
    backgroundColor: colors.surface,
  },
  passwordInput: {
    flex: 1,
    height: 46,
    paddingHorizontal: spacing.lg,
    fontSize: fontSize.base,
    color: colors.textPrimary,
  },
  eyeBtn: { paddingHorizontal: spacing.md, height: 46, justifyContent: "center" },
  rememberRow: { flexDirection: "row", alignItems: "center", gap: spacing.sm, marginTop: spacing.xs },
  rememberText: { fontSize: fontSize.sm, color: colors.textSecondary },
  button: { height: 48, borderRadius: radius.lg, justifyContent: "center", alignItems: "center" },
  buttonDisabled: { opacity: 0.6 },
  buttonText: { fontSize: fontSize.base, fontWeight: "800", color: "#fff" },
  serverToggle: { fontSize: fontSize.sm, color: colors.textTertiary, textAlign: "center", marginTop: spacing.sm },
});
