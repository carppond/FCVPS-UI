import { useState } from "react";
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
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../../lib/api-client";
import { useRuleTemplates, useCreateRule } from "../../api/rule";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { CreateSubscriptionRequest, Subscription, RuleTemplate } from "../../types/api";

const TEMPLATE_CATEGORIES = [
  { key: "region", label: "地区" },
  { key: "app", label: "应用" },
  { key: "block", label: "拦截" },
  { key: "common", label: "通用" },
] as const;

export default function CreateSubscriptionScreen() {
  const queryClient = useQueryClient();
  const [step, setStep] = useState(1);

  // Step 1: basic info
  const [name, setName] = useState("");
  const [sourceUrl, setSourceUrl] = useState("");
  const [remark, setRemark] = useState("");

  // Step 2: rule templates
  const [activeCategory, setActiveCategory] = useState("region");
  const [selectedTemplates, setSelectedTemplates] = useState<Set<string>>(new Set());
  const templatesQuery = useRuleTemplates();
  const createRuleMutation = useCreateRule();

  const templates = templatesQuery.data ?? [];
  const filteredTemplates = templates.filter(
    (t) => (t.category ?? "common") === activeCategory,
  );

  const toggleTemplate = (id: string) => {
    setSelectedTemplates((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const createMutation = useMutation({
    mutationFn: (data: CreateSubscriptionRequest) =>
      apiFetch<Subscription>("/api/subscriptions", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["subscription"] });
    },
  });

  const handleNext = () => {
    if (!name.trim()) {
      Alert.alert("提示", "请输入订阅名称");
      return;
    }
    setStep(2);
  };

  const handleCreate = async () => {
    try {
      // 1. Create subscription
      const req: CreateSubscriptionRequest = {
        name: name.trim(),
        type: sourceUrl.trim() ? "url" : "manual",
      };
      if (sourceUrl.trim()) req.source_url = sourceUrl.trim();
      if (remark.trim()) req.remark = remark.trim();
      await createMutation.mutateAsync(req);

      // 2. Create selected rule templates
      const selected = templates.filter((t) => selectedTemplates.has(t.id));
      for (const tpl of selected) {
        try {
          await createRuleMutation.mutateAsync({
            name: tpl.name,
            type: tpl.rule_type ?? "rules",
            mode: tpl.mode ?? "prepend",
            content: tpl.content,
            enabled: true,
          });
        } catch {
          // skip failed rules
        }
      }

      queryClient.invalidateQueries({ queryKey: ["rule"] });
      Alert.alert(
        "创建成功",
        selected.length > 0
          ? `订阅已添加，同时创建了 ${selected.length} 条规则`
          : "订阅已添加",
        [{ text: "好", onPress: () => router.back() }],
      );
    } catch (err: any) {
      Alert.alert("创建失败", err.message);
    }
  };

  const isPending = createMutation.isPending || createRuleMutation.isPending;

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      {/* Step indicator */}
      <View style={styles.stepBar}>
        <View style={styles.stepRow}>
          <View style={[styles.stepDot, step >= 1 && styles.stepDotActive]}>
            <Text style={[styles.stepNum, step >= 1 && styles.stepNumActive]}>1</Text>
          </View>
          <View style={[styles.stepLine, step >= 2 && styles.stepLineActive]} />
          <View style={[styles.stepDot, step >= 2 && styles.stepDotActive]}>
            <Text style={[styles.stepNum, step >= 2 && styles.stepNumActive]}>2</Text>
          </View>
        </View>
        <View style={styles.stepLabels}>
          <Text style={[styles.stepLabel, step === 1 && styles.stepLabelActive]}>基本信息</Text>
          <Text style={[styles.stepLabel, step === 2 && styles.stepLabelActive]}>选择规则</Text>
        </View>
      </View>

      {step === 1 ? (
        <ScrollView contentContainerStyle={styles.content}>
          {/* Name */}
          <View style={styles.card}>
            <View style={styles.cardHeader}>
              <View style={[styles.cardIcon, { backgroundColor: colors.primarySoft }]}>
                <Ionicons name="book-outline" size={16} color={colors.primary} />
              </View>
              <Text style={styles.cardTitle}>基本信息</Text>
            </View>
            <View style={styles.field}>
              <Text style={styles.label}>名称 <Text style={styles.required}>*</Text></Text>
              <TextInput
                style={styles.input}
                value={name}
                onChangeText={setName}
                placeholder="如：我的订阅"
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
              <Text style={styles.cardTitle}>订阅源</Text>
            </View>
            <View style={styles.field}>
              <Text style={styles.label}>URL</Text>
              <TextInput
                style={styles.input}
                value={sourceUrl}
                onChangeText={setSourceUrl}
                placeholder="https://example.com/subscribe?token=xxx"
                placeholderTextColor={colors.textDisabled}
                autoCapitalize="none"
                autoCorrect={false}
                keyboardType="url"
              />
              <Text style={styles.hint}>留空则创建手动订阅</Text>
            </View>
          </View>

          {/* Remark */}
          <View style={styles.card}>
            <View style={styles.cardHeader}>
              <View style={[styles.cardIcon, { backgroundColor: "rgba(0,0,0,0.04)" }]}>
                <Ionicons name="chatbubble-outline" size={16} color={colors.textTertiary} />
              </View>
              <Text style={styles.cardTitle}>备注</Text>
            </View>
            <View style={styles.field}>
              <TextInput
                style={[styles.input, styles.textArea]}
                value={remark}
                onChangeText={setRemark}
                placeholder="可选备注"
                placeholderTextColor={colors.textDisabled}
                multiline
                numberOfLines={3}
                textAlignVertical="top"
              />
            </View>
          </View>

          <TouchableOpacity style={styles.submitBtn} onPress={handleNext} activeOpacity={0.8}>
            <Text style={styles.submitText}>下一步 — 选择规则</Text>
          </TouchableOpacity>

          <TouchableOpacity
            style={styles.skipBtn}
            onPress={handleCreate}
            disabled={isPending}
            activeOpacity={0.6}
          >
            <Text style={styles.skipText}>{isPending ? "创建中..." : "跳过，直接创建"}</Text>
          </TouchableOpacity>
        </ScrollView>
      ) : (
        <View style={styles.step2Container}>
          {/* Category tabs */}
          <View style={styles.categoryWrap}>
            <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.categoryTabs}>
              {TEMPLATE_CATEGORIES.map((cat) => (
                <TouchableOpacity
                  key={cat.key}
                  style={[styles.categoryTab, activeCategory === cat.key && styles.categoryTabActive]}
                  onPress={() => setActiveCategory(cat.key)}
                  activeOpacity={0.7}
                >
                  <Text style={[styles.categoryTabText, activeCategory === cat.key && styles.categoryTabTextActive]}>
                    {cat.label}
                  </Text>
                </TouchableOpacity>
              ))}
            </ScrollView>
          </View>

          {/* Template list */}
          <ScrollView style={styles.templateScroll} contentContainerStyle={styles.templateList}>
            {filteredTemplates.map((item) => {
              const selected = selectedTemplates.has(item.id);
              return (
                <TouchableOpacity
                  key={item.id}
                  style={[styles.templateItem, selected && styles.templateItemSelected]}
                  onPress={() => toggleTemplate(item.id)}
                  activeOpacity={0.7}
                >
                  <Ionicons
                    name={selected ? "checkbox" : "square-outline"}
                    size={20}
                    color={selected ? colors.primary : colors.textDisabled}
                  />
                  <View style={styles.templateInfo}>
                    <Text style={styles.templateName}>
                      {item.emoji ? `${item.emoji} ` : ""}{item.name}
                    </Text>
                    {item.description ? (
                      <Text style={styles.templateDesc} numberOfLines={1}>{item.description}</Text>
                    ) : null}
                  </View>
                </TouchableOpacity>
              );
            })}
          </ScrollView>

          {/* Bottom bar */}
          <View style={styles.bottomBar}>
            <TouchableOpacity style={styles.backBtn} onPress={() => setStep(1)} activeOpacity={0.7}>
              <Ionicons name="arrow-back" size={16} color={colors.textSecondary} />
              <Text style={styles.backBtnText}>上一步</Text>
            </TouchableOpacity>
            <TouchableOpacity
              style={[styles.createBtn, isPending && styles.createBtnDisabled]}
              onPress={handleCreate}
              disabled={isPending}
              activeOpacity={0.8}
            >
              <Text style={styles.createBtnText}>
                {isPending ? "创建中..." : selectedTemplates.size > 0 ? `创建订阅 + ${selectedTemplates.size} 条规则` : "创建订阅"}
              </Text>
            </TouchableOpacity>
          </View>
        </View>
      )}
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.xl, paddingBottom: 40 },

  // Step bar
  stepBar: { paddingHorizontal: spacing.xxxl, paddingTop: spacing.lg, paddingBottom: spacing.sm },
  stepRow: { flexDirection: "row", alignItems: "center", justifyContent: "center" },
  stepDot: {
    width: 28, height: 28, borderRadius: 14,
    backgroundColor: colors.surfaceHover, justifyContent: "center", alignItems: "center",
  },
  stepDotActive: { backgroundColor: colors.primary },
  stepNum: { fontSize: fontSize.sm, fontWeight: "700", color: colors.textDisabled },
  stepNumActive: { color: "#fff" },
  stepLine: { flex: 1, height: 2, backgroundColor: colors.border, marginHorizontal: spacing.sm },
  stepLineActive: { backgroundColor: colors.primary },
  stepLabels: { flexDirection: "row", justifyContent: "space-between", marginTop: spacing.xs, paddingHorizontal: spacing.xl },
  stepLabel: { fontSize: fontSize.xs, color: colors.textDisabled, fontWeight: "600" },
  stepLabelActive: { color: colors.primary },

  // Cards
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
  textArea: { height: 80, paddingTop: spacing.md },
  hint: { fontSize: fontSize.xs, color: colors.textDisabled, marginTop: 2 },

  submitBtn: {
    height: 50, borderRadius: radius.lg, backgroundColor: colors.primary,
    justifyContent: "center", alignItems: "center", marginTop: spacing.sm,
  },
  submitText: { fontSize: fontSize.lg, fontWeight: "700", color: "#fff" },
  skipBtn: { alignItems: "center", marginTop: spacing.lg },
  skipText: { fontSize: fontSize.sm, color: colors.textTertiary },

  // Step 2
  step2Container: { flex: 1 },
  categoryWrap: { height: 52, flexShrink: 0 },
  categoryTabs: { paddingHorizontal: spacing.lg, paddingVertical: spacing.md, gap: spacing.sm, alignItems: "center" },
  categoryTab: {
    paddingHorizontal: spacing.lg, paddingVertical: spacing.sm,
    borderRadius: radius.lg, backgroundColor: colors.surface,
    borderWidth: 1, borderColor: colors.border, height: 34, justifyContent: "center",
  },
  categoryTabActive: { backgroundColor: colors.primarySoft, borderColor: colors.primary },
  categoryTabText: { fontSize: fontSize.sm, fontWeight: "600", color: colors.textTertiary },
  categoryTabTextActive: { color: colors.primary, fontWeight: "700" },

  templateScroll: { flex: 1 },
  templateList: { paddingHorizontal: spacing.lg, paddingBottom: spacing.lg },
  templateItem: {
    flexDirection: "row", alignItems: "flex-start", gap: spacing.md,
    backgroundColor: colors.surface, borderRadius: radius.xl,
    borderWidth: 1, borderColor: colors.border, padding: spacing.lg, marginBottom: spacing.sm,
  },
  templateItemSelected: { borderColor: colors.primary, backgroundColor: colors.primarySoft },
  templateInfo: { flex: 1, gap: 2 },
  templateName: { fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
  templateDesc: { fontSize: fontSize.xs, color: colors.textTertiary },

  bottomBar: {
    flexDirection: "row", gap: spacing.md, padding: spacing.lg,
    backgroundColor: colors.surface, borderTopWidth: 1, borderTopColor: colors.border,
  },
  backBtn: {
    flexDirection: "row", alignItems: "center", gap: spacing.xs,
    paddingHorizontal: spacing.lg, paddingVertical: spacing.md,
    borderRadius: radius.lg, borderWidth: 1, borderColor: colors.border,
  },
  backBtnText: { fontSize: fontSize.sm, fontWeight: "600", color: colors.textSecondary },
  createBtn: {
    flex: 1, height: 46, borderRadius: radius.lg, backgroundColor: colors.primary,
    justifyContent: "center", alignItems: "center",
  },
  createBtnDisabled: { opacity: 0.5 },
  createBtnText: { fontSize: fontSize.base, fontWeight: "700", color: "#fff" },
});
