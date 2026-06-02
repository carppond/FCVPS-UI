import { useState, useCallback, useMemo } from "react";
import {
  View,
  Text,
  FlatList,
  StyleSheet,
  RefreshControl,
  TouchableOpacity,
  Modal,
  Alert,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import {
  useProxyGroupsQuery,
  useProxyGroupPresets,
  useCreateProxyGroup,
} from "../api/proxy-group";
import { spacing, radius, fontSize, type AppColors } from "../lib/theme";
import { useColors } from "../lib/useColors";
import type { ProxyGroupCategory, ProxyGroupPreset } from "../types/api";

function typeColor(type: string, c: AppColors): string {
  switch (type) {
    case "select":
      return c.primary;
    case "url-test":
      return c.success;
    case "fallback":
      return c.warning;
    case "load-balance":
      return c.info;
    case "relay":
      return c.textTertiary;
    default:
      return c.textDisabled;
  }
}

function typeLabel(type: string): string {
  const map: Record<string, string> = {
    select: "手动选择",
    "url-test": "自动测速",
    fallback: "故障转移",
    "load-balance": "负载均衡",
    relay: "链式代理",
  };
  return map[type] ?? type;
}

export default function ProxyGroupsScreen() {
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const { data, isLoading, refetch } = useProxyGroupsQuery();
  const presetsQuery = useProxyGroupPresets();
  const createMutation = useCreateProxyGroup();
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

  const handleBatchCreate = async () => {
    const presets = presetsQuery.data ?? [];
    const selected = presets.filter((p) => selectedPresets.has(p.id));
    if (selected.length === 0) {
      Alert.alert("提示", "请至少选择一个预设");
      return;
    }
    try {
      for (const preset of selected) {
        await createMutation.mutateAsync({
          name: preset.name,
          type: preset.type,
          icon: preset.icon,
          test_url: preset.test_url,
          test_interval: preset.test_interval,
          filter: preset.filter,
          include_all: preset.include_all,
          member_proxies: preset.member_proxies,
          member_groups: preset.member_groups,
        });
      }
      setPresetModalVisible(false);
      Alert.alert("创建成功", `已从预设创建 ${selected.length} 个代理组`);
      refetch();
    } catch (err: any) {
      Alert.alert("创建失败", err.message);
    }
  };

  const renderItem = ({ item }: { item: ProxyGroupCategory }) => {
    const memberCount =
      (item.member_proxies?.length ?? 0) + (item.member_groups?.length ?? 0);
    return (
      <View style={styles.card}>
        <View style={styles.cardTop}>
          <View style={styles.cardInfo}>
            <Text style={styles.cardName} numberOfLines={1}>
              {item.name}
            </Text>
            <View style={styles.badgeRow}>
              <View
                style={[
                  styles.badge,
                  { backgroundColor: typeColor(item.type, colors) + "1a" },
                ]}
              >
                <Text
                  style={[styles.badgeText, { color: typeColor(item.type, colors) }]}
                >
                  {typeLabel(item.type)}
                </Text>
              </View>
              {item.include_all && (
                <View
                  style={[
                    styles.badge,
                    { backgroundColor: colors.successBg },
                  ]}
                >
                  <Text style={[styles.badgeText, { color: colors.success }]}>
                    全部节点
                  </Text>
                </View>
              )}
              <Text style={styles.memberCount}>{memberCount} 个成员</Text>
            </View>
          </View>
        </View>
      </View>
    );
  };

  return (
    <View style={styles.container}>
      <FlatList
        contentContainerStyle={items.length === 0 ? styles.empty : styles.list}
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
            <Text style={styles.presetBtnText}>从预设</Text>
          </TouchableOpacity>
        }
        ListEmptyComponent={
          !isLoading ? (
            <View style={styles.emptyBox}>
              <Ionicons
                name="git-branch-outline"
                size={48}
                color={colors.textDisabled}
              />
              <Text style={styles.emptyText}>暂无代理组</Text>
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
              <Text style={styles.modalCancel}>取消</Text>
            </TouchableOpacity>
            <Text style={styles.modalTitle}>选择预设</Text>
            <TouchableOpacity
              onPress={handleBatchCreate}
              disabled={createMutation.isPending}
            >
              <Text
                style={[
                  styles.modalDone,
                  createMutation.isPending && { opacity: 0.5 },
                ]}
              >
                {createMutation.isPending
                  ? "创建中..."
                  : `创建 (${selectedPresets.size})`}
              </Text>
            </TouchableOpacity>
          </View>
          <FlatList
            contentContainerStyle={styles.modalList}
            data={presetsQuery.data ?? []}
            keyExtractor={(item) => item.id}
            ListEmptyComponent={
              presetsQuery.isLoading ? (
                <Text style={styles.loadingText}>加载中...</Text>
              ) : (
                <Text style={styles.loadingText}>暂无预设</Text>
              )
            }
            renderItem={({ item }: { item: ProxyGroupPreset }) => {
              const selected = selectedPresets.has(item.id);
              return (
                <TouchableOpacity
                  style={[styles.presetItem, selected && styles.presetItemSelected]}
                  onPress={() => togglePreset(item.id)}
                  activeOpacity={0.7}
                >
                  <Ionicons
                    name={selected ? "checkbox" : "square-outline"}
                    size={20}
                    color={selected ? colors.primary : colors.textDisabled}
                  />
                  <View style={styles.presetInfo}>
                    <Text style={styles.presetName}>{item.name}</Text>
                    <View style={styles.badgeRow}>
                      <View
                        style={[
                          styles.badge,
                          { backgroundColor: typeColor(item.type, colors) + "1a" },
                        ]}
                      >
                        <Text
                          style={[
                            styles.badgeText,
                            { color: typeColor(item.type, colors) },
                          ]}
                        >
                          {typeLabel(item.type)}
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
  emptyBox: { alignItems: "center", gap: spacing.md },
  emptyText: { fontSize: fontSize.base, color: colors.textTertiary },
  presetBtn: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.sm,
    backgroundColor: colors.primarySoft,
    borderRadius: radius.lg,
    padding: spacing.md,
    marginBottom: spacing.lg,
  },
  presetBtnText: {
    fontSize: fontSize.sm,
    fontWeight: "700",
    color: colors.primary,
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
  cardInfo: { flex: 1, gap: 4 },
  cardName: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
  },
  badgeRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
    flexWrap: "wrap",
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
  memberCount: { fontSize: fontSize.xs, color: colors.textTertiary },
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
