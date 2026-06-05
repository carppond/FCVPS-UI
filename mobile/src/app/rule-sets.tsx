import { useState, useCallback, useMemo } from "react";
import {
  View,
  Text,
  FlatList,
  StyleSheet,
  RefreshControl,
  TouchableOpacity,
  Alert,
  Modal,
  Switch,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useTranslation } from "react-i18next";
import {
  useRuleSetsQuery,
  useSyncAllRuleSets,
  useRuleSetPresets,
  useCreateRuleSet,
  useDeleteRuleSet,
  useUpdateRuleSet,
  useSyncRuleSet,
} from "../api/rule-set";
import { spacing, radius, fontSize, type AppColors } from "../lib/theme";
import { useColors } from "../lib/useColors";
import type { RuleSetProvider, RuleSetPreset } from "../types/api";

function syncStatusColor(status: string | undefined, c: AppColors): string {
  switch (status) {
    case "ok":
      return c.success;
    case "error":
      return c.error;
    case "pending":
      return c.warning;
    default:
      return c.textDisabled;
  }
}

function behaviorColor(behavior: string, c: AppColors): string {
  switch (behavior) {
    case "domain":
      return c.info;
    case "ipcidr":
      return c.warning;
    case "classical":
      return c.primary;
    default:
      return c.textTertiary;
  }
}

export default function RuleSetsScreen() {
  const { t } = useTranslation(["rules", "common"]);
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const { data, isLoading, refetch } = useRuleSetsQuery();
  const syncAll = useSyncAllRuleSets();
  const presetsQuery = useRuleSetPresets();
  const createMutation = useCreateRuleSet();
  const deleteMutation = useDeleteRuleSet();
  const updateMutation = useUpdateRuleSet();
  const syncSingleMutation = useSyncRuleSet();
  const [refreshing, setRefreshing] = useState(false);
  const [presetModalVisible, setPresetModalVisible] = useState(false);
  const [selectedPresets, setSelectedPresets] = useState<Set<string>>(
    new Set(),
  );

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const handleSyncAll = () => {
    syncAll.mutate(undefined, {
      onSuccess: () => {
        Alert.alert(t("ruleset_sync_success"), t("ruleset_sync_all_started"));
        refetch();
      },
      onError: (err: any) => Alert.alert(t("ruleset_sync_failed"), err.message),
    });
  };

  const openPresetModal = async () => {
    await presetsQuery.refetch();
    setSelectedPresets(new Set());
    setPresetModalVisible(true);
  };

  const togglePreset = (id: string) => {
    setSelectedPresets((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleBatchImport = async () => {
    const presets = presetsQuery.data ?? [];
    const selected = presets.filter((p) => selectedPresets.has(p.id));
    if (selected.length === 0) {
      Alert.alert(t("common:tip"), t("ruleset_select_at_least_one"));
      return;
    }
    try {
      for (const preset of selected) {
        await createMutation.mutateAsync({
          name: preset.name,
          behavior: preset.behavior,
          format: preset.format,
          url: preset.url,
          interval_seconds: preset.interval_seconds,
          enabled: true,
        });
      }
      setPresetModalVisible(false);
      Alert.alert(t("ruleset_import_success"), t("ruleset_imported_count", { count: selected.length }));
      refetch();
    } catch (err: any) {
      Alert.alert(t("ruleset_import_failed"), err.message);
    }
  };

  const handleDelete = (item: RuleSetProvider) => {
    Alert.alert(t("common:delete_confirm_title"), t("ruleset_delete_confirm", { name: item.name }), [
      { text: t("common:cancel"), style: "cancel" },
      {
        text: t("common:delete"),
        style: "destructive",
        onPress: () => {
          deleteMutation.mutate(item.id, {
            onSuccess: () => refetch(),
            onError: (err: any) => Alert.alert(t("common:delete_failed"), err.message),
          });
        },
      },
    ]);
  };

  const handleToggleEnabled = (item: RuleSetProvider) => {
    updateMutation.mutate(
      { id: item.id, data: { enabled: !item.enabled } },
      {
        onSuccess: () => refetch(),
        onError: (err: any) => Alert.alert(t("common:operation_failed"), err.message),
      },
    );
  };

  const handleSyncSingle = (item: RuleSetProvider) => {
    syncSingleMutation.mutate(item.id, {
      onSuccess: () => {
        Alert.alert(t("ruleset_sync_success"), t("ruleset_sync_single_done", { name: item.name }));
        refetch();
      },
      onError: (err: any) => Alert.alert(t("ruleset_sync_failed"), err.message),
    });
  };

  const renderItem = ({ item }: { item: RuleSetProvider }) => (
    <View style={styles.card}>
      <View style={styles.cardTop}>
        <View
          style={[
            styles.syncDot,
            { backgroundColor: syncStatusColor(item.last_sync_status, colors) },
          ]}
        />
        <View style={styles.cardInfo}>
          <Text style={styles.cardName} numberOfLines={1}>
            {item.name}
          </Text>
          <View style={styles.badgeRow}>
            <View
              style={[
                styles.badge,
                { backgroundColor: behaviorColor(item.behavior, colors) + "1a" },
              ]}
            >
              <Text
                style={[
                  styles.badgeText,
                  { color: behaviorColor(item.behavior, colors) },
                ]}
              >
                {item.behavior}
              </Text>
            </View>
            <View style={styles.badge}>
              <Text style={styles.badgeText}>{item.format}</Text>
            </View>
          </View>
        </View>
        <View style={styles.cardActions}>
          <Switch
            value={item.enabled}
            onValueChange={() => handleToggleEnabled(item)}
            trackColor={{
              false: colors.surfaceHover,
              true: colors.primary,
            }}
            thumbColor="#fff"
            style={styles.toggleSwitch}
          />
          <TouchableOpacity
            onPress={() => handleSyncSingle(item)}
            hitSlop={{ top: 8, bottom: 8, left: 8, right: 8 }}
            activeOpacity={0.6}
          >
            <Ionicons name="sync-outline" size={16} color={colors.info} />
          </TouchableOpacity>
          <TouchableOpacity
            onPress={() => handleDelete(item)}
            hitSlop={{ top: 8, bottom: 8, left: 8, right: 8 }}
          >
            <Ionicons name="trash-outline" size={16} color={colors.error} />
          </TouchableOpacity>
        </View>
      </View>
    </View>
  );

  return (
    <View style={styles.container}>
      <FlatList
        contentContainerStyle={styles.list}
        data={items}
        keyExtractor={(item) => item.id}
        refreshControl={
          <RefreshControl
            refreshing={refreshing}
            onRefresh={onRefresh}
            tintColor={colors.primary}
          />
        }
        ListHeaderComponent={
          <View style={styles.headerBtns}>
            <TouchableOpacity
              style={[
                styles.syncAllBtn,
                syncAll.isPending && styles.syncAllBtnDisabled,
              ]}
              onPress={handleSyncAll}
              disabled={syncAll.isPending}
              activeOpacity={0.7}
            >
              <Ionicons name="sync-outline" size={16} color={colors.primary} />
              <Text style={styles.syncAllText}>
                {syncAll.isPending ? t("ruleset_syncing") : t("ruleset_sync_all")}
              </Text>
            </TouchableOpacity>
            <TouchableOpacity
              style={styles.presetBtn}
              onPress={openPresetModal}
              activeOpacity={0.7}
            >
              <Ionicons
                name="albums-outline"
                size={16}
                color={colors.primary}
              />
              <Text style={styles.syncAllText}>{t("ruleset_from_preset")}</Text>
            </TouchableOpacity>
          </View>
        }
        ListEmptyComponent={
          !isLoading ? (
            <View style={styles.emptyBox}>
              <Ionicons
                name="layers-outline"
                size={48}
                color={colors.textDisabled}
              />
              <Text style={styles.emptyText}>{t("ruleset_empty")}</Text>
              <Text style={styles.emptyHint}>{t("ruleset_empty_hint")}</Text>
            </View>
          ) : null
        }
        renderItem={renderItem}
      />

      {/* Preset Modal */}
      <Modal
        visible={presetModalVisible}
        animationType="slide"
        presentationStyle="pageSheet"
        onRequestClose={() => setPresetModalVisible(false)}
      >
        <View style={styles.modalContainer}>
          <View style={styles.modalHeader}>
            <TouchableOpacity onPress={() => setPresetModalVisible(false)}>
              <Text style={styles.modalCancel}>{t("common:cancel")}</Text>
            </TouchableOpacity>
            <Text style={styles.modalTitle}>{t("ruleset_select_preset")}</Text>
            <TouchableOpacity
              onPress={handleBatchImport}
              disabled={createMutation.isPending}
            >
              <Text
                style={[
                  styles.modalDone,
                  createMutation.isPending && { opacity: 0.5 },
                ]}
              >
                {createMutation.isPending
                  ? t("ruleset_importing")
                  : t("ruleset_importing_count", { count: selectedPresets.size })}
              </Text>
            </TouchableOpacity>
          </View>
          <FlatList
            contentContainerStyle={styles.modalList}
            data={presetsQuery.data ?? []}
            keyExtractor={(item) => item.id}
            ListEmptyComponent={
              presetsQuery.isLoading ? (
                <Text style={styles.loadingText}>{t("common:loading")}</Text>
              ) : (
                <Text style={styles.loadingText}>{t("ruleset_no_preset")}</Text>
              )
            }
            renderItem={({ item }: { item: RuleSetPreset }) => {
              const selected = selectedPresets.has(item.id);
              return (
                <TouchableOpacity
                  style={[
                    styles.presetItem,
                    selected && styles.presetItemSelected,
                  ]}
                  onPress={() => togglePreset(item.id)}
                  activeOpacity={0.7}
                >
                  <Ionicons
                    name={selected ? "checkbox" : "square-outline"}
                    size={20}
                    color={selected ? colors.primary : colors.textDisabled}
                  />
                  <View style={styles.presetInfo}>
                    <Text style={styles.presetName}>
                      {item.emoji ? `${item.emoji} ` : ""}
                      {item.name}
                    </Text>
                    <View style={styles.badgeRow}>
                      <View
                        style={[
                          styles.badge,
                          {
                            backgroundColor:
                              behaviorColor(item.behavior, colors) + "1a",
                          },
                        ]}
                      >
                        <Text
                          style={[
                            styles.badgeText,
                            { color: behaviorColor(item.behavior, colors) },
                          ]}
                        >
                          {item.behavior}
                        </Text>
                      </View>
                      {item.description && (
                        <Text style={styles.presetDesc} numberOfLines={1}>
                          {item.description}
                        </Text>
                      )}
                    </View>
                  </View>
                </TouchableOpacity>
              );
            }}
          />
        </View>
      </Modal>
    </View>
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  list: { padding: spacing.lg },
  empty: { flex: 1, justifyContent: "center", alignItems: "center" },
  emptyBox: { alignItems: "center", gap: spacing.md, paddingTop: 80 },
  emptyText: { fontSize: fontSize.base, color: colors.textTertiary },
  emptyHint: { fontSize: fontSize.xs, color: colors.textDisabled, marginTop: spacing.xs },
  headerBtns: {
    flexDirection: "row",
    gap: spacing.sm,
    marginBottom: spacing.lg,
  },
  syncAllBtn: {
    flex: 1,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.sm,
    backgroundColor: colors.primarySoft,
    borderRadius: radius.lg,
    padding: spacing.md,
  },
  syncAllBtnDisabled: { opacity: 0.5 },
  syncAllText: {
    fontSize: fontSize.sm,
    fontWeight: "700",
    color: colors.primary,
  },
  presetBtn: {
    flex: 1,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.sm,
    backgroundColor: colors.primarySoft,
    borderRadius: radius.lg,
    padding: spacing.md,
  },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.md,
  },
  cardTop: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
  },
  syncDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
  },
  cardInfo: { flex: 1, gap: 4 },
  cardName: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
  },
  badgeRow: {
    flexDirection: "row",
    gap: spacing.sm,
    alignItems: "center",
  },
  badge: {
    backgroundColor: colors.elevated,
    borderRadius: radius.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
  },
  badgeText: {
    fontSize: fontSize.xs,
    fontWeight: "600",
    color: colors.textSecondary,
  },
  cardActions: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
  },
  toggleSwitch: {
    transform: [{ scaleX: 0.8 }, { scaleY: 0.8 }],
  },
  // Modal
  modalContainer: { flex: 1, backgroundColor: colors.bg },
  modalHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: spacing.xl,
    paddingVertical: spacing.lg,
    backgroundColor: colors.surface,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  modalCancel: {
    fontSize: fontSize.base,
    color: colors.textSecondary,
  },
  modalTitle: {
    fontSize: fontSize.lg,
    fontWeight: "700",
    color: colors.textPrimary,
  },
  modalDone: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.primary,
  },
  modalList: { padding: spacing.lg },
  loadingText: {
    fontSize: fontSize.base,
    color: colors.textTertiary,
    textAlign: "center",
    marginTop: spacing.xxl,
  },
  presetItem: {
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
  presetItemSelected: {
    borderColor: colors.primary,
    backgroundColor: colors.primarySoft,
  },
  presetInfo: { flex: 1, gap: 4 },
  presetName: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
  },
  presetDesc: { fontSize: fontSize.xs, color: colors.textTertiary, flex: 1 },
});
