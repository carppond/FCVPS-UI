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
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import * as Clipboard from "expo-clipboard";
import { useCreateAgent } from "../../api/agent";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";
import type { AgentKind, AgentCreateResponse } from "../../types/api";

const KINDS: { key: AgentKind; label: string; desc: string }[] = [
  { key: "native", label: "原生 Agent", desc: "使用拾光VPS 自带的探针二进制" },
  { key: "nezha_compat", label: "哪吒兼容", desc: "兼容已有的哪吒 v2 Agent" },
];

export default function CreateAgentScreen() {
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const createMutation = useCreateAgent();
  const [name, setName] = useState("");
  const [kind, setKind] = useState<AgentKind>("native");
  const [result, setResult] = useState<AgentCreateResponse | null>(null);

  const handleCreate = async () => {
    if (!name.trim()) {
      Alert.alert("提示", "请输入探针名称");
      return;
    }
    try {
      const res = await createMutation.mutateAsync({ name: name.trim(), kind });
      setResult(res);
    } catch (err: any) {
      Alert.alert("创建失败", err.message);
    }
  };

  const copyText = async (text: string, label: string) => {
    await Clipboard.setStringAsync(text);
    Alert.alert("已复制", `${label} 已复制到剪贴板`);
  };

  // Show result after creation
  if (result) {
    return (
      <ScrollView style={styles.container} contentContainerStyle={styles.content}>
        <View style={styles.successCard}>
          <Ionicons name="checkmark-circle" size={48} color={colors.success} />
          <Text style={styles.successTitle}>探针已创建</Text>
          <Text style={styles.successName}>{result.name}</Text>
        </View>

        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.warningBg }]}>
              <Ionicons name="key-outline" size={16} color={colors.warning} />
            </View>
            <Text style={styles.cardTitle}>Token（仅显示一次）</Text>
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
            <Text style={styles.copyBtnText}>复制 Token</Text>
          </TouchableOpacity>
        </View>

        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.infoBg }]}>
              <Ionicons name="terminal-outline" size={16} color={colors.info} />
            </View>
            <Text style={styles.cardTitle}>安装命令</Text>
          </View>
          <View style={styles.tokenBox}>
            <Text style={styles.commandText} selectable>{result.install_command}</Text>
          </View>
          <TouchableOpacity
            style={styles.copyBtn}
            onPress={() => copyText(result.install_command, "安装命令")}
            activeOpacity={0.7}
          >
            <Ionicons name="copy-outline" size={14} color={colors.primary} />
            <Text style={styles.copyBtnText}>复制命令</Text>
          </TouchableOpacity>
        </View>

        <TouchableOpacity
          style={styles.doneBtn}
          onPress={() => router.back()}
          activeOpacity={0.8}
        >
          <Text style={styles.doneBtnText}>完成</Text>
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
          <Text style={styles.cardTitle}>探针信息</Text>
        </View>
        <View style={styles.field}>
          <Text style={styles.label}>名称 <Text style={styles.required}>*</Text></Text>
          <TextInput
            style={styles.input}
            value={name}
            onChangeText={setName}
            placeholder="如：hk-vps-01"
            placeholderTextColor={colors.textDisabled}
          />
        </View>
      </View>

      <View style={styles.card}>
        <View style={styles.cardHeader}>
          <View style={[styles.cardIcon, { backgroundColor: colors.infoBg }]}>
            <Ionicons name="options-outline" size={16} color={colors.info} />
          </View>
          <Text style={styles.cardTitle}>类型</Text>
        </View>
        {KINDS.map((k) => (
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
          {createMutation.isPending ? "创建中..." : "创建探针"}
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
