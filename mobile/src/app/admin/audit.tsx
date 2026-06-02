import { useState, useCallback, useMemo } from "react";
import {
  View,
  Text,
  FlatList,
  StyleSheet,
  RefreshControl,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useAuditLogs } from "../../api/admin";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";
import type { AuditLog } from "../../types/api";

function formatTime(ts: number): string {
  return new Date(ts * 1000).toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function AdminAuditScreen() {
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const { data, isLoading, refetch } = useAuditLogs();
  const [refreshing, setRefreshing] = useState(false);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const renderItem = ({ item }: { item: AuditLog }) => (
    <View style={styles.card}>
      <View style={styles.cardHeader}>
        <View style={styles.actionRow}>
          <Text style={styles.action} numberOfLines={1}>
            {item.action}
          </Text>
          <View
            style={[
              styles.statusBadge,
              {
                backgroundColor: item.success
                  ? colors.successBg
                  : colors.errorBg,
              },
            ]}
          >
            <Text
              style={[
                styles.statusText,
                {
                  color: item.success ? colors.success : colors.error,
                },
              ]}
            >
              {item.success ? "成功" : "失败"}
            </Text>
          </View>
        </View>
        <Text style={styles.time}>{formatTime(item.created_at)}</Text>
      </View>
      <View style={styles.metaRow}>
        {item.resource_type ? (
          <View style={styles.metaItem}>
            <Ionicons
              name="cube-outline"
              size={12}
              color={colors.textTertiary}
            />
            <Text style={styles.metaText}>{item.resource_type}</Text>
          </View>
        ) : null}
        {item.user_id ? (
          <View style={styles.metaItem}>
            <Ionicons
              name="person-outline"
              size={12}
              color={colors.textTertiary}
            />
            <Text style={styles.metaText} numberOfLines={1}>
              {item.user_id.slice(0, 8)}
            </Text>
          </View>
        ) : null}
        {item.ip ? (
          <View style={styles.metaItem}>
            <Ionicons
              name="globe-outline"
              size={12}
              color={colors.textTertiary}
            />
            <Text style={styles.metaText}>{item.ip}</Text>
          </View>
        ) : null}
      </View>
    </View>
  );

  return (
    <FlatList
      style={styles.container}
      contentContainerStyle={items.length === 0 ? styles.empty : styles.list}
      data={items}
      keyExtractor={(item) => String(item.id)}
      refreshControl={
        <RefreshControl
          refreshing={refreshing}
          onRefresh={onRefresh}
          tintColor={colors.primary}
        />
      }
      ListEmptyComponent={
        !isLoading ? (
          <View style={styles.emptyBox}>
            <Ionicons
              name="document-text-outline"
              size={48}
              color={colors.textDisabled}
            />
            <Text style={styles.emptyText}>暂无审计日志</Text>
          </View>
        ) : null
      }
      renderItem={renderItem}
    />
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  list: { padding: spacing.lg },
  empty: { flex: 1, justifyContent: "center", alignItems: "center" },
  emptyBox: { alignItems: "center", gap: spacing.md },
  emptyText: { fontSize: fontSize.base, color: colors.textTertiary },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.md,
  },
  cardHeader: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    marginBottom: spacing.sm,
  },
  actionRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
    flex: 1,
  },
  action: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
    flex: 1,
  },
  statusBadge: {
    borderRadius: radius.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
  },
  statusText: {
    fontSize: fontSize.xs,
    fontWeight: "700",
  },
  time: {
    fontSize: fontSize.xs,
    color: colors.textTertiary,
    marginLeft: spacing.sm,
  },
  metaRow: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: spacing.md,
  },
  metaItem: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
  },
  metaText: {
    fontSize: fontSize.xs,
    color: colors.textTertiary,
  },
});
