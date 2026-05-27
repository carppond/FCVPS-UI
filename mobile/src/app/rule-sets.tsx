import { useState, useCallback } from "react";
import {
  View,
  Text,
  FlatList,
  StyleSheet,
  RefreshControl,
  TouchableOpacity,
  Alert,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useRuleSetsQuery, useSyncAllRuleSets } from "../api/rule-set";
import { colors, spacing, radius, fontSize } from "../lib/theme";
import type { RuleSetProvider } from "../types/api";

function syncStatusColor(status?: string): string {
  switch (status) {
    case "ok":
      return colors.success;
    case "error":
      return colors.error;
    case "pending":
      return colors.warning;
    default:
      return colors.textDisabled;
  }
}

function behaviorColor(behavior: string): string {
  switch (behavior) {
    case "domain":
      return colors.info;
    case "ipcidr":
      return colors.warning;
    case "classical":
      return colors.primary;
    default:
      return colors.textTertiary;
  }
}

export default function RuleSetsScreen() {
  const { data, isLoading, refetch } = useRuleSetsQuery();
  const syncAll = useSyncAllRuleSets();
  const [refreshing, setRefreshing] = useState(false);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const handleSyncAll = () => {
    syncAll.mutate(undefined, {
      onSuccess: () => {
        Alert.alert("同步成功", "所有规则集已开始同步");
        refetch();
      },
      onError: (err: any) => Alert.alert("同步失败", err.message),
    });
  };

  const renderItem = ({ item }: { item: RuleSetProvider }) => (
    <View style={styles.card}>
      <View style={styles.cardTop}>
        <View
          style={[
            styles.syncDot,
            { backgroundColor: syncStatusColor(item.last_sync_status) },
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
                { backgroundColor: behaviorColor(item.behavior) + "1a" },
              ]}
            >
              <Text
                style={[
                  styles.badgeText,
                  { color: behaviorColor(item.behavior) },
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
        <View
          style={[
            styles.enabledChip,
            {
              backgroundColor: item.enabled
                ? colors.successBg
                : "rgba(255,255,255,0.04)",
            },
          ]}
        >
          <Text
            style={[
              styles.enabledText,
              {
                color: item.enabled
                  ? colors.success
                  : colors.textDisabled,
              },
            ]}
          >
            {item.enabled ? "启用" : "禁用"}
          </Text>
        </View>
      </View>
    </View>
  );

  return (
    <View style={styles.container}>
      <FlatList
        contentContainerStyle={
          items.length === 0 ? styles.empty : styles.list
        }
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
              {syncAll.isPending ? "同步中..." : "一键同步"}
            </Text>
          </TouchableOpacity>
        }
        ListEmptyComponent={
          !isLoading ? (
            <View style={styles.emptyBox}>
              <Ionicons
                name="layers-outline"
                size={48}
                color={colors.textDisabled}
              />
              <Text style={styles.emptyText}>暂无规则集</Text>
            </View>
          ) : null
        }
        renderItem={renderItem}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  list: { padding: spacing.lg },
  empty: { flex: 1, justifyContent: "center", alignItems: "center" },
  emptyBox: { alignItems: "center", gap: spacing.md },
  emptyText: { fontSize: fontSize.base, color: colors.textTertiary },
  syncAllBtn: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.sm,
    backgroundColor: colors.primarySoft,
    borderRadius: radius.lg,
    padding: spacing.md,
    marginBottom: spacing.lg,
  },
  syncAllBtnDisabled: { opacity: 0.5 },
  syncAllText: {
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
  enabledChip: {
    borderRadius: radius.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 4,
  },
  enabledText: {
    fontSize: fontSize.xs,
    fontWeight: "700",
  },
});
