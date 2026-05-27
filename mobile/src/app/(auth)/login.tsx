import { useState, useEffect } from "react";
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  StyleSheet,
  Alert,
  KeyboardAvoidingView,
  Platform,
  ActivityIndicator,
} from "react-native";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import * as SecureStore from "expo-secure-store";
import { useLoginMutation } from "../../api/auth";
import { useAuthStore } from "../../stores/auth-store";
import { STORAGE_KEYS } from "../../lib/constants";
import { colors, spacing, radius, fontSize } from "../../lib/theme";

export default function LoginScreen() {
  const [serverUrl, setServerUrl] = useState(useAuthStore.getState().serverUrl);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [showServer, setShowServer] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(true);
  const [autoLogging, setAutoLogging] = useState(true);
  const loginMutation = useLoginMutation();
  const setServerUrlStore = useAuthStore((s) => s.setServerUrl);

  // Load saved credentials on mount + auto-login
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

      // Auto-login if credentials exist
      if (savedUser && savedPass) {
        try {
          if (savedServer) {
            await useAuthStore.getState().setServerUrl(savedServer);
          }
          await loginMutation.mutateAsync({ username: savedUser, password: savedPass });
          router.replace("/(tabs)");
          return;
        } catch {
          // Auto-login failed, show form
        }
      }
      setAutoLogging(false);
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

      // Save credentials
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

  // Show loading while auto-login
  if (autoLogging) {
    return (
      <View style={styles.autoLogin}>
        <Text style={styles.logo}>拾光VPS</Text>
        <ActivityIndicator size="large" color={colors.primary} style={{ marginTop: 24 }} />
        <Text style={styles.autoLoginText}>自动登录中...</Text>
      </View>
    );
  }

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <View style={styles.inner}>
        <View style={styles.header}>
          <Text style={styles.logo}>拾光VPS</Text>
          <Text style={styles.subtitle}>自托管代理订阅管理平台</Text>
        </View>

        <View style={styles.form}>
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

          {/* Remember me */}
          <TouchableOpacity
            style={styles.rememberRow}
            onPress={() => setRememberMe(!rememberMe)}
            activeOpacity={0.6}
          >
            <Ionicons
              name={rememberMe ? "checkbox-outline" : "square-outline"}
              size={20}
              color={rememberMe ? colors.primary : colors.textTertiary}
            />
            <Text style={styles.rememberText}>记住账号密码</Text>
          </TouchableOpacity>

          <TouchableOpacity
            style={[styles.button, loginMutation.isPending && styles.buttonDisabled]}
            onPress={handleLogin}
            disabled={loginMutation.isPending}
            activeOpacity={0.8}
          >
            <Text style={styles.buttonText}>
              {loginMutation.isPending ? "登录中..." : "登录"}
            </Text>
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
      </View>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  inner: { flex: 1, justifyContent: "center", paddingHorizontal: spacing.xxxl },
  header: { alignItems: "center", marginBottom: 48 },
  logo: { fontSize: 32, fontWeight: "800", color: colors.textPrimary, letterSpacing: -1 },
  subtitle: { fontSize: fontSize.sm, color: colors.textTertiary, marginTop: spacing.sm },
  form: { gap: spacing.lg },
  field: { gap: spacing.xs },
  label: { fontSize: fontSize.sm, fontWeight: "600", color: colors.textSecondary },
  input: {
    height: 48,
    borderRadius: radius.lg,
    borderWidth: 1,
    borderColor: colors.borderStrong,
    backgroundColor: colors.surface,
    paddingHorizontal: spacing.lg,
    fontSize: fontSize.base,
    color: colors.textPrimary,
  },
  button: {
    height: 48,
    borderRadius: radius.lg,
    backgroundColor: colors.primary,
    justifyContent: "center",
    alignItems: "center",
    marginTop: spacing.sm,
  },
  buttonDisabled: { opacity: 0.5 },
  buttonText: { fontSize: fontSize.base, fontWeight: "700", color: "#fff" },
  passwordWrap: {
    flexDirection: "row",
    alignItems: "center",
    borderRadius: radius.lg,
    borderWidth: 1,
    borderColor: colors.borderStrong,
    backgroundColor: colors.surface,
  },
  passwordInput: {
    flex: 1,
    height: 48,
    paddingHorizontal: spacing.lg,
    fontSize: fontSize.base,
    color: colors.textPrimary,
  },
  eyeBtn: {
    paddingHorizontal: spacing.md,
    height: 48,
    justifyContent: "center",
  },
  rememberRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
  },
  rememberText: { fontSize: fontSize.sm, color: colors.textSecondary },
  serverToggle: { fontSize: fontSize.sm, color: colors.textTertiary, textAlign: "center", marginTop: spacing.sm },
  autoLogin: {
    flex: 1,
    backgroundColor: colors.bg,
    justifyContent: "center",
    alignItems: "center",
  },
  autoLoginText: { fontSize: fontSize.sm, color: colors.textTertiary, marginTop: spacing.md },
});
