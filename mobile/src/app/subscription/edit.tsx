import { useState, useEffect, useMemo } from "react";
import { useTranslation } from "react-i18next";
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
  ActivityIndicator,
  Switch,
} from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useSubscriptionDetail, useUpdateSubscription } from "../../api/subscription";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";
import type { UpdateSubscriptionRequest } from "../../types/api";

export default function EditSubscriptionScreen() {
  const { t } = useTranslation(["subscription", "common"]);
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const { id } = useLocalSearchParams<{ id: string }>();
  const { data, isLoading } = useSubscriptionDetail(id ?? "");
  const updateMutation = useUpdateSubscription();

  const [name, setName] = useState("");
  const [sourceUrl, setSourceUrl] = useState("");
  const [remark, setRemark] = useState("");
  const [allowInsecure, setAllowInsecure] = useState(false);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    if (data && !loaded) {
      setName(data.name ?? "");
      setSourceUrl(data.source_url ?? "");
      setRemark(data.remark ?? "");
      setAllowInsecure(data.allow_insecure ?? false);
      setLoaded(true);
    }
  }, [data, loaded]);

  const handleSave = () => {
    if (!name.trim()) {
      Alert.alert(t("common:tip"), t("required_name"));
      return;
    }
    const req: UpdateSubscriptionRequest = {
      name: name.trim(),
    };
    if (sourceUrl.trim()) {
      req.source_url = sourceUrl.trim();
    }
    req.remark = remark.trim() || undefined;
    req.allow_insecure = allowInsecure;
    updateMutation.mutate(
      { id: id!, data: req },
      {
        onSuccess: () => {
          Alert.alert(t("save_success"), t("saved_message"), [
            { text: t("common:ok"), onPress: () => router.back() },
          ]);
        },
        onError: (err: any) => Alert.alert(t("save_failed"), err.message),
      },
    );
  };

  if (isLoading) {
    return (
      <View style={styles.loadingContainer}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <ScrollView contentContainerStyle={styles.content}>
        {/* Name */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.primarySoft }]}>
              <Ionicons name="book-outline" size={16} color={colors.primary} />
            </View>
            <Text style={styles.cardTitle}>{t("basic_info")}</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("name")} <Text style={styles.required}>*</Text></Text>
            <TextInput
              style={styles.input}
              value={name}
              onChangeText={setName}
              placeholder={t("name_placeholder")}
              placeholderTextColor={colors.textDisabled}
            />
          </View>
        </View>

        {/* Source URL */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.infoBg }]}>
              <Ionicons name="link-outline" size={16} color={colors.info} />
            </View>
            <Text style={styles.cardTitle}>{t("source")}</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("url")}</Text>
            <TextInput
              style={styles.input}
              value={sourceUrl}
              onChangeText={setSourceUrl}
              placeholder={t("url_placeholder")}
              placeholderTextColor={colors.textDisabled}
              autoCapitalize="none"
              autoCorrect={false}
              keyboardType="url"
            />
          </View>
        </View>

        {/* Remark */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: "rgba(255,255,255,0.04)" }]}>
              <Ionicons name="chatbubble-outline" size={16} color={colors.textTertiary} />
            </View>
            <Text style={styles.cardTitle}>{t("remark")}</Text>
          </View>
          <View style={styles.field}>
            <TextInput
              style={[styles.input, styles.textArea]}
              value={remark}
              onChangeText={setRemark}
              placeholder={t("remark_placeholder")}
              placeholderTextColor={colors.textDisabled}
              multiline
              numberOfLines={3}
              textAlignVertical="top"
            />
          </View>
        </View>

        {/* Allow insecure TLS (url subscriptions only) */}
        {data?.type === "url" && (
          <View style={styles.card}>
            <View style={styles.cardHeader}>
              <View style={[styles.cardIcon, { backgroundColor: colors.warningBg }]}>
                <Ionicons name="shield-outline" size={16} color={colors.warning} />
              </View>
              <Text style={styles.cardTitle}>{t("allow_insecure_title")}</Text>
            </View>
            <View style={styles.insecureRow}>
              <Text style={styles.insecureHint}>
                {t("allow_insecure_hint")}
              </Text>
              <Switch
                value={allowInsecure}
                onValueChange={setAllowInsecure}
                trackColor={{ false: colors.border, true: colors.primarySoft }}
                thumbColor={allowInsecure ? colors.primary : colors.textDisabled}
              />
            </View>
          </View>
        )}

        {/* Submit */}
        <TouchableOpacity
          style={[styles.submitBtn, updateMutation.isPending && styles.submitBtnDisabled]}
          onPress={handleSave}
          disabled={updateMutation.isPending}
          activeOpacity={0.8}
        >
          <Text style={styles.submitText}>
            {updateMutation.isPending ? t("common:saving") : t("save_changes")}
          </Text>
        </TouchableOpacity>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  loadingContainer: { flex: 1, justifyContent: "center", alignItems: "center", backgroundColor: colors.bg },
  content: { padding: spacing.xl, paddingBottom: 40 },
  insecureRow: { flexDirection: "row", alignItems: "center", gap: spacing.md },
  insecureHint: { flex: 1, fontSize: fontSize.sm, color: colors.textTertiary, lineHeight: 18 },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.xl,
    marginBottom: spacing.lg,
  },
  cardHeader: { flexDirection: "row", alignItems: "center", gap: spacing.sm, marginBottom: spacing.lg },
  cardIcon: { width: 28, height: 28, borderRadius: radius.md, justifyContent: "center", alignItems: "center" },
  cardTitle: { fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
  field: { gap: spacing.xs },
  label: { fontSize: fontSize.sm, fontWeight: "600", color: colors.textSecondary },
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
  textArea: { height: 80, paddingTop: spacing.md },
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
});
