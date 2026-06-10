import { useState, useMemo } from "react";
import {
  View,
  Text,
  TextInput,
  ScrollView,
  StyleSheet,
  TouchableOpacity,
  Alert,
} from "react-native";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import * as Clipboard from "expo-clipboard";
import { useCreateAgent } from "../../api/agent";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";
import type { AgentKind, AgentCreateResponse } from "../../types/api";
import { formatApiError } from "../../lib/format-api-error";

const buildKinds = (t: TFunction): { key: AgentKind; label: string; desc: string }[] => [
  { key: "native", label: t("kind_native"), desc: t("kind_native_desc") },
  { key: "nezha_compat", label: t("kind_nezha"), desc: t("kind_nezha_desc") },
];

export default function CreateAgentScreen() {
  const { t } = useTranslation(["agents", "common"]);
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const kinds = useMemo(() => buildKinds(t), [t]);
  const createMutation = useCreateAgent();
  const [name, setName] = useState("");
  const [kind, setKind] = useState<AgentKind>("native");
  const [result, setResult] = useState<AgentCreateResponse | null>(null);

  const handleCreate = async () => {
    if (!name.trim()) {
      Alert.alert(t("common:tip"), t("input_name_required"));
      return;
    }
    try {
      const res = await createMutation.mutateAsync({ name: name.trim(), kind });
      setResult(res);
    } catch (err: any) {
      Alert.alert(t("common:create_failed"), formatApiError(err, t));
    }
  };

  const copyText = async (text: string, label: string) => {
    await Clipboard.setStringAsync(text);
    Alert.alert(t("common:copied"), t("command_copied", { label }));
  };

  // Show result after creation
  if (result) {
    return (
      <ScrollView style={styles.container} contentContainerStyle={styles.content}>
        <View style={styles.successCard}>
          <Ionicons name="checkmark-circle" size={48} color={colors.success} />
          <Text style={styles.successTitle}>{t("created")}</Text>
          <Text style={styles.successName}>{result.name}</Text>
        </View>

        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.warningBg }]}>
              <Ionicons name="key-outline" size={16} color={colors.warning} />
            </View>
            <Text style={styles.cardTitle}>{t("token_once")}</Text>
          </View>
          <View style={styles.tokenBox}>
            <Text style={styles.tokenText} selectable>{result.token}</Text>
          </View>
          <TouchableOpacity
            style={styles.copyBtn}
            onPress={() => copyText(result.token, "Token")}
            activeOpacity={0.7}
          >
            <Ionicons name="copy-outline" size={14} color={colors.primary} />
            <Text style={styles.copyBtnText}>{t("copy_token")}</Text>
          </TouchableOpacity>
        </View>

        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.infoBg }]}>
              <Ionicons name="terminal-outline" size={16} color={colors.info} />
            </View>
            <Text style={styles.cardTitle}>{t("install_command")}</Text>
          </View>
          <View style={styles.tokenBox}>
            <Text style={styles.commandText} selectable>{result.install_command}</Text>
          </View>
          <TouchableOpacity
            style={styles.copyBtn}
            onPress={() => copyText(result.install_command, t("install_command_label"))}
            activeOpacity={0.7}
          >
            <Ionicons name="copy-outline" size={14} color={colors.primary} />
            <Text style={styles.copyBtnText}>{t("copy_command")}</Text>
          </TouchableOpacity>
        </View>

        <TouchableOpacity
          style={styles.doneBtn}
          onPress={() => router.back()}
          activeOpacity={0.8}
        >
          <Text style={styles.doneBtnText}>{t("done")}</Text>
        </TouchableOpacity>
      </ScrollView>
    );
  }

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <View style={styles.card}>
        <View style={styles.cardHeader}>
          <View style={[styles.cardIcon, { backgroundColor: colors.primarySoft }]}>
            <Ionicons name="radio-outline" size={16} color={colors.primary} />
          </View>
          <Text style={styles.cardTitle}>{t("info_section")}</Text>
        </View>
        <View style={styles.field}>
          <Text style={styles.label}>{t("name_label")} <Text style={styles.required}>*</Text></Text>
          <TextInput
            style={styles.input}
            value={name}
            onChangeText={setName}
            placeholder={t("name_placeholder_example")}
            placeholderTextColor={colors.textDisabled}
          />
        </View>
      </View>

      <View style={styles.card}>
        <View style={styles.cardHeader}>
          <View style={[styles.cardIcon, { backgroundColor: colors.infoBg }]}>
            <Ionicons name="options-outline" size={16} color={colors.info} />
          </View>
          <Text style={styles.cardTitle}>{t("section_kind")}</Text>
        </View>
        {kinds.map((k) => (
          <TouchableOpacity
            key={k.key}
            style={[styles.kindOption, kind === k.key && styles.kindOptionActive]}
            onPress={() => setKind(k.key)}
            activeOpacity={0.7}
          >
            <Ionicons
              name={kind === k.key ? "radio-button-on" : "radio-button-off"}
              size={20}
              color={kind === k.key ? colors.primary : colors.textDisabled}
            />
            <View style={styles.kindInfo}>
              <Text style={[styles.kindLabel, kind === k.key && styles.kindLabelActive]}>{k.label}</Text>
              <Text style={styles.kindDesc}>{k.desc}</Text>
            </View>
          </TouchableOpacity>
        ))}
      </View>

      <TouchableOpacity
        style={[styles.submitBtn, createMutation.isPending && styles.submitBtnDisabled]}
        onPress={handleCreate}
        disabled={createMutation.isPending}
        activeOpacity={0.8}
      >
        <Text style={styles.submitText}>
          {createMutation.isPending ? t("common:creating") : t("submit_create")}
        </Text>
      </TouchableOpacity>
    </ScrollView>
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.xl, paddingBottom: 40 },
  card: {
    backgroundColor: colors.surface, borderRadius: radius.xl,
    borderWidth: 1, borderColor: colors.border, padding: spacing.xl, marginBottom: spacing.lg,
  },
  cardHeader: { flexDirection: "row", alignItems: "center", gap: spacing.sm, marginBottom: spacing.lg },
  cardIcon: { width: 28, height: 28, borderRadius: radius.md, justifyContent: "center", alignItems: "center" },
  cardTitle: { fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
  field: { gap: spacing.xs },
  label: { fontSize: fontSize.sm, fontWeight: "600", color: colors.textSecondary },
  required: { color: colors.primary, fontSize: fontSize.xs },
  input: {
    height: 48, borderRadius: radius.lg, borderWidth: 1,
    borderColor: colors.borderStrong, backgroundColor: colors.elevated,
    paddingHorizontal: spacing.lg, fontSize: fontSize.base, color: colors.textPrimary,
  },
  kindOption: {
    flexDirection: "row", alignItems: "flex-start", gap: spacing.md,
    padding: spacing.md, borderRadius: radius.lg, marginBottom: spacing.sm,
    borderWidth: 1, borderColor: colors.border,
  },
  kindOptionActive: { borderColor: colors.primary, backgroundColor: colors.primarySoft },
  kindInfo: { flex: 1, gap: 2 },
  kindLabel: { fontSize: fontSize.base, fontWeight: "600", color: colors.textPrimary },
  kindLabelActive: { color: colors.primary },
  kindDesc: { fontSize: fontSize.xs, color: colors.textTertiary },
  submitBtn: {
    height: 50, borderRadius: radius.lg, backgroundColor: colors.primary,
    justifyContent: "center", alignItems: "center",
  },
  submitBtnDisabled: { opacity: 0.5 },
  submitText: { fontSize: fontSize.lg, fontWeight: "700", color: "#fff" },
  // Success state
  successCard: {
    alignItems: "center", gap: spacing.sm,
    backgroundColor: colors.surface, borderRadius: radius.xl,
    borderWidth: 1, borderColor: colors.border, padding: spacing.xxxl, marginBottom: spacing.lg,
  },
  successTitle: { fontSize: fontSize.xl, fontWeight: "800", color: colors.textPrimary },
  successName: { fontSize: fontSize.sm, color: colors.textTertiary },
  tokenBox: {
    backgroundColor: colors.elevated, borderRadius: radius.md,
    padding: spacing.md, marginBottom: spacing.sm,
  },
  tokenText: { fontSize: fontSize.sm, fontFamily: "monospace", color: colors.textPrimary, lineHeight: 20 },
  commandText: { fontSize: fontSize.xs, fontFamily: "monospace", color: colors.textSecondary, lineHeight: 18 },
  copyBtn: {
    flexDirection: "row", alignItems: "center", justifyContent: "center", gap: spacing.xs,
    backgroundColor: colors.primarySoft, borderRadius: radius.md,
    paddingVertical: spacing.sm,
  },
  copyBtnText: { fontSize: fontSize.sm, fontWeight: "600", color: colors.primary },
  doneBtn: {
    height: 50, borderRadius: radius.lg, backgroundColor: colors.primary,
    justifyContent: "center", alignItems: "center", marginTop: spacing.sm,
  },
  doneBtnText: { fontSize: fontSize.lg, fontWeight: "700", color: "#fff" },
});
