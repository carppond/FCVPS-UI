import { View, Text, FlatList, StyleSheet, RefreshControl } from "react-native";
import { useState, useCallback } from "react";
import { Ionicons } from "@expo/vector-icons";
import { useAgentsQuery } from "../../api/agent";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { AgentListItem } from "../../types/api";

export default function AgentsScreen() {
  const { data, isLoading, refetch } = useAgentsQuery();
  const [refreshing, setRefreshing] = useState(false);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  return (
    <FlatList
      style={styles.container}
      contentContainerStyle={items.length === 0 ? styles.empty : styles.list}
      data={items}
      keyExtractor={(item) => item.id}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor={colors.primary} />
      }
      ListEmptyComponent={
        !isLoading ? (
          <View style={styles.emptyBox}>
            <Ionicons name="radio-outline" size={48} color={colors.textDisabled} />
            <Text style={styles.emptyText}>暂无探针</Text>
          </View>
        ) : null
      }
      renderItem={({ item }) => <AgentCard agent={item} />}
    />
  );
}

function AgentCard({ agent }: { agent: AgentListItem }) {
  const online = agent.online;
  const cpu = agent.latest_metrics?.cpu_percent;
  const memUsed = agent.latest_metrics?.mem_used;
  const memTotal = agent.latest_metrics?.mem_total;
  const memPct = memTotal ? Math.round((memUsed ?? 0) / memTotal * 100) : null;

  return (
    <View style={[styles.card, online ? styles.cardOnline : styles.cardOffline]}>
      <View style={styles.cardHeader}>
        <View style={[styles.dot, { backgroundColor: online ? colors.success : colors.error }]} />
        <Text style={styles.cardName} numberOfLines={1}>{agent.name}</Text>
        <Text style={styles.kindBadge}>{agent.kind}</Text>
      </View>
      {online && cpu !== undefined && (
        <View style={styles.metricsRow}>
          <MetricChip label="CPU" value={`${Math.round(cpu)}%`} color={cpu > 80 ? colors.error : cpu > 50 ? colors.warning : colors.success} />
          {memPct !== null && (
            <MetricChip label="MEM" value={`${memPct}%`} color={memPct > 80 ? colors.error : memPct > 50 ? colors.warning : colors.info} />
          )}
        </View>
      )}
      {!online && (
        <Text style={styles.offlineText}>离线</Text>
      )}
      <View style={styles.metaRow}>
        {agent.public_ip && <Text style={styles.metaText}>{agent.public_ip}</Text>}
        {agent.os && <Text style={styles.metaText}>{agent.os} {agent.arch}</Text>}
      </View>
    </View>
  );
}

function MetricChip({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <View style={styles.metric}>
      <Text style={styles.metricLabel}>{label}</Text>
      <Text style={[styles.metricValue, { color }]}>{value}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  list: { padding: spacing.lg },
  empty: { flex: 1, justifyContent: "center", alignItems: "center" },
  emptyBox: { alignItems: "center", gap: spacing.md },
  emptyText: { fontSize: fontSize.base, color: colors.textTertiary },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    padding: spacing.lg,
    marginBottom: spacing.md,
    gap: spacing.sm,
  },
  cardOnline: { borderColor: colors.border },
  cardOffline: { borderColor: "rgba(248,113,113,0.15)" },
  cardHeader: { flexDirection: "row", alignItems: "center", gap: spacing.sm },
  dot: { width: 8, height: 8, borderRadius: 4 },
  cardName: { flex: 1, fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
  kindBadge: { fontSize: fontSize.xs, color: colors.textTertiary, backgroundColor: colors.surfaceHover, paddingHorizontal: spacing.sm, paddingVertical: 2, borderRadius: radius.sm, fontWeight: "600" },
  metricsRow: { flexDirection: "row", gap: spacing.sm },
  metric: { flex: 1, backgroundColor: colors.surfaceHover, borderRadius: radius.md, padding: spacing.sm, alignItems: "center" },
  metricLabel: { fontSize: 8, fontWeight: "700", color: colors.textDisabled, textTransform: "uppercase", letterSpacing: 0.5 },
  metricValue: { fontSize: fontSize.lg, fontWeight: "800", marginTop: 2 },
  offlineText: { fontSize: fontSize.sm, color: colors.error },
  metaRow: { flexDirection: "row", gap: spacing.md },
  metaText: { fontSize: fontSize.xs, color: colors.textDisabled, fontFamily: "monospace" },
});
