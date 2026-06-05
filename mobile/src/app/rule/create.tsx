import { useState, useMemo } from "react";
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
import { router, useLocalSearchParams } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useRuleTemplates, useCreateRule, useUpdateRule } from "../../api/rule";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";
import type { RuleTemplate, RuleType, RuleMode } from "../../types/api";

const buildCategories = (t: TFunction) =>
  [
    { key: "region", label: t("rule_create_category_region") },
    { key: "app", label: t("rule_create_category_app") },
    { key: "block", label: t("rule_create_category_block") },
    { key: "common", label: t("rule_create_category_common") },
  ] as const;

const buildRuleTypes = (t: TFunction): { key: RuleType; label: string }[] => [
  { key: "dns", label: t("rule_create_type_dns") },
  { key: "rules", label: t("rule_create_type_rules") },
  { key: "rule-providers", label: t("rule_create_type_rule_providers") },
];

const buildRuleModes = (t: TFunction): { key: RuleMode; label: string }[] => [
  { key: "replace", label: t("rule_create_mode_replace") },
  { key: "prepend", label: t("rule_create_mode_prepend") },
  { key: "append", label: t("rule_create_mode_append") },
];

export default function RuleCreateScreen() {
  const { t } = useTranslation(["rules", "common"]);
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const CATEGORIES = useMemo(() => buildCategories(t), [t]);
  const RULE_TYPES = useMemo(() => buildRuleTypes(t), [t]);
  const RULE_MODES = useMemo(() => buildRuleModes(t), [t]);
  const params = useLocalSearchParams<{
    editId?: string;
    editName?: string;
    editType?: string;
    editMode?: string;
    editContent?: string;
    editEnabled?: string;
  }>();

  const isEdit = !!params.editId;

  const templatesQuery = useRuleTemplates();
  const createMutation = useCreateRule();
  const updateMutation = useUpdateRule();
  const [mode, setMode] = useState<"template" | "manual">(isEdit ? "manual" : "template");
  const [activeCategory, setActiveCategory] = useState("region");
  const [selectedTemplates, setSelectedTemplates] = useState<Set<string>>(
    new Set(),
  );

  // Manual form state — prefill from edit params if present
  const [name, setName] = useState(params.editName ?? "");
  const [ruleType, setRuleType] = useState<RuleType>((params.editType as RuleType) ?? "rules");
  const [ruleMode, setRuleMode] = useState<RuleMode>((params.editMode as RuleMode) ?? "prepend");
  const [content, setContent] = useState(params.editContent ?? "");

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
      Alert.alert(t("common:tip"), t("rule_create_select_at_least_one"));
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
      Alert.alert(t("rule_create_success_title"), t("rule_create_created_count", { count: selected.length }), [
        { text: t("common:ok"), onPress: () => router.back() },
      ]);
    } catch (err: any) {
      Alert.alert(t("common:create_failed"), err.message);
    }
  };

  const handleManualCreate = () => {
    if (!name.trim()) {
      Alert.alert(t("common:tip"), t("rule_create_enter_name"));
      return;
    }
    if (!content.trim()) {
      Alert.alert(t("common:tip"), t("rule_create_enter_content"));
      return;
    }

    if (isEdit) {
      updateMutation.mutate(
        {
          id: params.editId!,
          data: {
            name: name.trim(),
            mode: ruleMode,
            content: content.trim(),
            enabled: params.editEnabled === "true",
          },
        },
        {
          onSuccess: () => {
            Alert.alert(t("common:save_success"), t("rule_create_updated_one"), [
              { text: t("common:ok"), onPress: () => router.back() },
            ]);
          },
          onError: (err: any) => Alert.alert(t("common:save_failed"), err.message),
        },
      );
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
          Alert.alert(t("rule_create_success_title"), t("rule_create_created_one"), [
            { text: t("common:ok"), onPress: () => router.back() },
          ]);
        },
        onError: (err: any) => Alert.alert(t("common:create_failed"), err.message),
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
            {t("rule_create_tab_template")}
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
            {t("rule_create_tab_manual")}
          </Text>
        </TouchableOpacity>
      </View>

      {mode === "template" ? (
        <View style={styles.templateContainer}>
          {/* Category Tabs */}
          <View style={styles.categoryTabsWrap}>
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
          </View>

          {/* Template List */}
          <FlatList
            contentContainerStyle={styles.templateList}
            data={filteredTemplates}
            keyExtractor={(item) => item.id}
            ListEmptyComponent={
              templatesQuery.isLoading ? (
                <Text style={styles.loadingText}>{t("common:loading")}</Text>
              ) : (
                <Text style={styles.loadingText}>{t("rule_create_empty_category")}</Text>
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
                    style={{ marginTop: 2 }}
                  />
                  <View style={styles.templateInfo}>
                    <Text style={styles.templateName}>
                      {item.emoji ? `${item.emoji} ` : ""}
                      {item.name}
                    </Text>
                    {item.description ? (
                      <Text style={styles.templateDesc} numberOfLines={1}>
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
                  ? t("common:creating")
                  : t("rule_create_creating_selected", { count: selectedTemplates.size })}
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
                <Text style={styles.cardTitle}>{t("rule_create_section_basic")}</Text>
              </View>
              <View style={styles.field}>
                <Text style={styles.label}>
                  {t("rule_create_label_name")} <Text style={styles.required}>*</Text>
                </Text>
                <TextInput
                  style={styles.input}
                  value={name}
                  onChangeText={setName}
                  placeholder={t("rule_create_name_placeholder")}
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
                <Text style={styles.cardTitle}>{t("rule_create_section_type_mode")}</Text>
              </View>
              <View style={styles.field}>
                <Text style={styles.label}>{t("rule_create_label_type")}</Text>
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
                <Text style={styles.label}>{t("rule_create_label_mode")}</Text>
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
                <Text style={styles.cardTitle}>{t("rule_create_content_title")}</Text>
              </View>
              <View style={styles.field}>
                <TextInput
                  style={[styles.input, styles.textArea]}
                  value={content}
                  onChangeText={setContent}
                  placeholder={t("rule_create_content_placeholder")}
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
                (createMutation.isPending || updateMutation.isPending) && styles.submitBtnDisabled,
              ]}
              onPress={handleManualCreate}
              disabled={createMutation.isPending || updateMutation.isPending}
              activeOpacity={0.8}
            >
              <Text style={styles.submitText}>
                {isEdit
                  ? updateMutation.isPending ? t("common:saving") : t("rule_create_submit_edit")
                  : createMutation.isPending ? t("common:creating") : t("rule_create_submit")}
              </Text>
            </TouchableOpacity>
          </ScrollView>
        </KeyboardAvoidingView>
      )}
    </View>
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
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
  categoryTabsWrap: {
    height: 52,
    flexShrink: 0,
  },
  categoryTabs: {
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
    gap: spacing.sm,
    alignItems: "center",
  },
  categoryTab: {
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.sm,
    borderRadius: radius.lg,
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    height: 34,
    justifyContent: "center",
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
    alignItems: "flex-start",
    gap: spacing.md,
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.sm,
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
