import { useState } from "react";
import {
  View,
  Text,
  TextInput,
  FlatList,
  ScrollView,
  StyleSheet,
  TouchableOpacity,
  Alert,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useRuleTemplates, useCreateRule } from "../../api/rule";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { RuleTemplate, RuleType, RuleMode } from "../../types/api";

const CATEGORIES = [
  { key: "region", label: "地区" },
  { key: "app", label: "应用" },
  { key: "block", label: "拦截" },
  { key: "common", label: "通用" },
] as const;

const RULE_TYPES: { key: RuleType; label: string }[] = [
  { key: "dns", label: "DNS" },
  { key: "rules", label: "规则" },
  { key: "rule-providers", label: "规则集" },
];

const RULE_MODES: { key: RuleMode; label: string }[] = [
  { key: "replace", label: "替换" },
  { key: "prepend", label: "前置" },
  { key: "append", label: "追加" },
];

export default function RuleCreateScreen() {
  const templatesQuery = useRuleTemplates();
  const createMutation = useCreateRule();
  const [mode, setMode] = useState<"template" | "manual">("template");
  const [activeCategory, setActiveCategory] = useState("region");
  const [selectedTemplates, setSelectedTemplates] = useState<Set<string>>(
    new Set(),
  );

  // Manual form state
  const [name, setName] = useState("");
  const [ruleType, setRuleType] = useState<RuleType>("rules");
  const [ruleMode, setRuleMode] = useState<RuleMode>("prepend");
  const [content, setContent] = useState("");

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

  const handleBatchCreate = async () => {
    const selected = templates.filter((t) => selectedTemplates.has(t.id));
    if (selected.length === 0) {
      Alert.alert("提示", "请至少选择一个模板");
      return;
    }
    try {
      for (const tpl of selected) {
        await createMutation.mutateAsync({
          name: tpl.name,
          type: tpl.rule_type ?? "rules",
          mode: tpl.mode ?? "prepend",
          content: tpl.content,
          enabled: true,
        });
      }
      Alert.alert("创建成功", `已创建 ${selected.length} 条规则`, [
        { text: "好", onPress: () => router.back() },
      ]);
    } catch (err: any) {
      Alert.alert("创建失败", err.message);
    }
  };

  const handleManualCreate = () => {
    if (!name.trim()) {
      Alert.alert("提示", "请输入规则名称");
      return;
    }
    if (!content.trim()) {
      Alert.alert("提示", "请输入规则内容");
      return;
    }
    createMutation.mutate(
      {
        name: name.trim(),
        type: ruleType,
        mode: ruleMode,
        content: content.trim(),
        enabled: true,
      },
      {
        onSuccess: () => {
          Alert.alert("创建成功", "规则已添加", [
            { text: "好", onPress: () => router.back() },
          ]);
        },
        onError: (err: any) => Alert.alert("创建失败", err.message),
      },
    );
  };

  return (
    <View style={styles.container}>
      {/* Mode Tabs */}
      <View style={styles.modeTabs}>
        <TouchableOpacity
          style={[styles.modeTab, mode === "template" && styles.modeTabActive]}
          onPress={() => setMode("template")}
          activeOpacity={0.7}
        >
          <Text
            style={[
              styles.modeTabText,
              mode === "template" && styles.modeTabTextActive,
            ]}
          >
            模板
          </Text>
        </TouchableOpacity>
        <TouchableOpacity
          style={[styles.modeTab, mode === "manual" && styles.modeTabActive]}
          onPress={() => setMode("manual")}
          activeOpacity={0.7}
        >
          <Text
            style={[
              styles.modeTabText,
              mode === "manual" && styles.modeTabTextActive,
            ]}
          >
            手动
          </Text>
        </TouchableOpacity>
      </View>

      {mode === "template" ? (
        <View style={styles.templateContainer}>
          {/* Category Tabs */}
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={styles.categoryTabs}
          >
            {CATEGORIES.map((cat) => (
              <TouchableOpacity
                key={cat.key}
                style={[
                  styles.categoryTab,
                  activeCategory === cat.key && styles.categoryTabActive,
                ]}
                onPress={() => setActiveCategory(cat.key)}
                activeOpacity={0.7}
              >
                <Text
                  style={[
                    styles.categoryTabText,
                    activeCategory === cat.key && styles.categoryTabTextActive,
                  ]}
                >
                  {cat.label}
                </Text>
              </TouchableOpacity>
            ))}
          </ScrollView>

          {/* Template List */}
          <FlatList
            contentContainerStyle={styles.templateList}
            data={filteredTemplates}
            keyExtractor={(item) => item.id}
            ListEmptyComponent={
              templatesQuery.isLoading ? (
                <Text style={styles.loadingText}>加载中...</Text>
              ) : (
                <Text style={styles.loadingText}>该分类暂无模板</Text>
              )
            }
            renderItem={({ item }: { item: RuleTemplate }) => {
              const selected = selectedTemplates.has(item.id);
              return (
                <TouchableOpacity
                  style={[
                    styles.templateItem,
                    selected && styles.templateItemSelected,
                  ]}
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
                      {item.emoji ? `${item.emoji} ` : ""}
                      {item.name}
                    </Text>
                    {item.description ? (
                      <Text style={styles.templateDesc} numberOfLines={2}>
                        {item.description}
                      </Text>
                    ) : null}
                  </View>
                </TouchableOpacity>
              );
            }}
          />

          {/* Batch Create Button */}
          <View style={styles.bottomBar}>
            <TouchableOpacity
              style={[
                styles.submitBtn,
                (createMutation.isPending || selectedTemplates.size === 0) &&
                  styles.submitBtnDisabled,
              ]}
              onPress={handleBatchCreate}
              disabled={createMutation.isPending || selectedTemplates.size === 0}
              activeOpacity={0.8}
            >
              <Text style={styles.submitText}>
                {createMutation.isPending
                  ? "创建中..."
                  : `创建选中规则 (${selectedTemplates.size})`}
              </Text>
            </TouchableOpacity>
          </View>
        </View>
      ) : (
        <KeyboardAvoidingView
          style={styles.manualContainer}
          behavior={Platform.OS === "ios" ? "padding" : "height"}
        >
          <ScrollView contentContainerStyle={styles.manualContent}>
            {/* Name */}
            <View style={styles.card}>
              <View style={styles.cardHeader}>
                <View
                  style={[
                    styles.cardIcon,
                    { backgroundColor: colors.primarySoft },
                  ]}
                >
                  <Ionicons
                    name="create-outline"
                    size={16}
                    color={colors.primary}
                  />
                </View>
                <Text style={styles.cardTitle}>基本信息</Text>
              </View>
              <View style={styles.field}>
                <Text style={styles.label}>
                  名称 <Text style={styles.required}>*</Text>
                </Text>
                <TextInput
                  style={styles.input}
                  value={name}
                  onChangeText={setName}
                  placeholder="如：自定义规则"
                  placeholderTextColor={colors.textDisabled}
                />
              </View>
            </View>

            {/* Type & Mode */}
            <View style={styles.card}>
              <View style={styles.cardHeader}>
                <View
                  style={[
                    styles.cardIcon,
                    { backgroundColor: colors.infoBg },
                  ]}
                >
                  <Ionicons
                    name="options-outline"
                    size={16}
                    color={colors.info}
                  />
                </View>
                <Text style={styles.cardTitle}>类型与模式</Text>
              </View>
              <View style={styles.field}>
                <Text style={styles.label}>类型</Text>
                <View style={styles.optionRow}>
                  {RULE_TYPES.map((rt) => (
                    <TouchableOpacity
                      key={rt.key}
                      style={[
                        styles.optionChip,
                        ruleType === rt.key && styles.optionChipActive,
                      ]}
                      onPress={() => setRuleType(rt.key)}
                      activeOpacity={0.7}
                    >
                      <Text
                        style={[
                          styles.optionChipText,
                          ruleType === rt.key && styles.optionChipTextActive,
                        ]}
                      >
                        {rt.label}
                      </Text>
                    </TouchableOpacity>
                  ))}
                </View>
              </View>
              <View style={[styles.field, { marginTop: spacing.md }]}>
                <Text style={styles.label}>模式</Text>
                <View style={styles.optionRow}>
                  {RULE_MODES.map((rm) => (
                    <TouchableOpacity
                      key={rm.key}
                      style={[
                        styles.optionChip,
                        ruleMode === rm.key && styles.optionChipActive,
                      ]}
                      onPress={() => setRuleMode(rm.key)}
                      activeOpacity={0.7}
                    >
                      <Text
                        style={[
                          styles.optionChipText,
                          ruleMode === rm.key && styles.optionChipTextActive,
                        ]}
                      >
                        {rm.label}
                      </Text>
                    </TouchableOpacity>
                  ))}
                </View>
              </View>
            </View>

            {/* Content */}
            <View style={styles.card}>
              <View style={styles.cardHeader}>
                <View
                  style={[
                    styles.cardIcon,
                    { backgroundColor: colors.warningBg },
                  ]}
                >
                  <Ionicons
                    name="code-outline"
                    size={16}
                    color={colors.warning}
                  />
                </View>
                <Text style={styles.cardTitle}>规则内容</Text>
              </View>
              <View style={styles.field}>
                <TextInput
                  style={[styles.input, styles.textArea]}
                  value={content}
                  onChangeText={setContent}
                  placeholder="DOMAIN-SUFFIX,example.com,PROXY"
                  placeholderTextColor={colors.textDisabled}
                  multiline
                  numberOfLines={6}
                  textAlignVertical="top"
                />
              </View>
            </View>

            {/* Submit */}
            <TouchableOpacity
              style={[
                styles.submitBtn,
                createMutation.isPending && styles.submitBtnDisabled,
              ]}
              onPress={handleManualCreate}
              disabled={createMutation.isPending}
              activeOpacity={0.8}
            >
              <Text style={styles.submitText}>
                {createMutation.isPending ? "创建中..." : "创建规则"}
              </Text>
            </TouchableOpacity>
          </ScrollView>
        </KeyboardAvoidingView>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  modeTabs: {
    flexDirection: "row",
    marginHorizontal: spacing.lg,
    marginTop: spacing.lg,
    backgroundColor: colors.surfaceHover,
    borderRadius: radius.lg,
    padding: 3,
  },
  modeTab: {
    flex: 1,
    paddingVertical: spacing.sm,
    borderRadius: radius.md,
    alignItems: "center",
  },
  modeTabActive: {
    backgroundColor: colors.surface,
  },
  modeTabText: {
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.textTertiary,
  },
  modeTabTextActive: {
    color: colors.textPrimary,
    fontWeight: "700",
  },
  // Template mode
  templateContainer: { flex: 1 },
  categoryTabs: {
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
    gap: spacing.sm,
  },
  categoryTab: {
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.sm,
    borderRadius: radius.lg,
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
  },
  categoryTabActive: {
    backgroundColor: colors.primarySoft,
    borderColor: colors.primary,
  },
  categoryTabText: {
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.textTertiary,
  },
  categoryTabTextActive: {
    color: colors.primary,
    fontWeight: "700",
  },
  templateList: { paddingHorizontal: spacing.lg },
  loadingText: {
    fontSize: fontSize.base,
    color: colors.textTertiary,
    textAlign: "center",
    marginTop: spacing.xxl,
  },
  templateItem: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.md,
  },
  templateItemSelected: {
    borderColor: colors.primary,
    backgroundColor: colors.primarySoft,
  },
  templateInfo: { flex: 1, gap: 2 },
  templateName: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
  },
  templateDesc: {
    fontSize: fontSize.xs,
    color: colors.textTertiary,
  },
  bottomBar: {
    padding: spacing.lg,
    backgroundColor: colors.surface,
    borderTopWidth: 1,
    borderTopColor: colors.border,
  },
  // Manual mode
  manualContainer: { flex: 1 },
  manualContent: { padding: spacing.xl, paddingBottom: 40 },
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
  field: { gap: spacing.xs },
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
  textArea: { height: 140, paddingTop: spacing.md },
  optionRow: {
    flexDirection: "row",
    gap: spacing.sm,
    flexWrap: "wrap",
  },
  optionChip: {
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.sm,
    borderRadius: radius.lg,
    backgroundColor: colors.elevated,
    borderWidth: 1,
    borderColor: colors.border,
  },
  optionChipActive: {
    backgroundColor: colors.primarySoft,
    borderColor: colors.primary,
  },
  optionChipText: {
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.textTertiary,
  },
  optionChipTextActive: {
    color: colors.primary,
    fontWeight: "700",
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
});
