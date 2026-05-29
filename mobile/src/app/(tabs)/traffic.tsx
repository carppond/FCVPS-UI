import { View, Text, FlatList, StyleSheet, RefreshControl } from "react-native";
import { useState, useCallback, useEffect } from "react";
import { Ionicons } from "@expo/vector-icons";
import { useTrafficSummary } from "../../api/traffic";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { AgentTrafficSummary } from "../../types/api";
import { pushTrafficToWidget } from "../../lib/widget-sync";

function formatBytes(n: number): string {
  if (n === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(n) / Math.log(1024));
  const idx = Math.min(i, units.length - 1);
  return (n / Math.pow(1024, idx)).toFixed(idx === 0 ? 0 : 1) + " " + units[idx];
}

export default function TrafficScreen() {
  const { data, isLoading, refetch } = useTrafficSummary();
  const [refreshing, setRefreshing] = useState(false);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  // Feed the home-screen widget whenever fresh traffic loads (no-op when the
  // widget runtime isn't linked — Expo Go / before prebuild).
  useEffect(() => {
    if (data) pushTrafficToWidget(data);
  }, [data]);

  const agents = data?.agents ?? [];

  return (
    <FlatList
      style={styles.container}
      contentContainerStyle={agents.length === 0 && !data ? styles.empty : styles.list}
      data={agents}
      keyExtractor={(item) => item.agent_id}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor={colors.primary} />
      }
      ListHeaderComponent={
        data ? (
          <View style={styles.summarySection}>
            <View style={styles.summaryRow}>
              <SumChip label="上传" value={formatBytes(data.total_in)} icon="arrow-up-outline" />
              <SumChip label="下载" value={formatBytes(data.total_out)} icon="arrow-down-outline" />
            </View>
            <View style={styles.summaryRow}>
              <SumChip label="总用量" value={formatBytes(data.total_used)} icon="swap-vertical-outline" />
              <SumChip
                label="使用率"
                value={`${Math.round(data.usage_percent)}%`}
                icon="pie-chart-outline"
                color={data.usage_percent > 80 ? colors.error : data.usage_percent > 50 ? colors.warning : colors.success}
              />
            </View>
            {agents.length > 0 && (
              <Text style={styles.sectionTitle}>各探针流量</Text>
            )}
          </View>
        ) : null
      }
      ListEmptyComponent={
        !isLoading && !data ? (
          <View style={styles.emptyBox}>
            <Ionicons name="analytics-outline" size={48} color={colors.textDisabled} />
            <Text style={styles.emptyText}>暂无流量数据</Text>
          </View>
        ) : null
      }
      renderItem={({ item }) => <AgentTrafficCard agent={item} />}
    />
  );
}

function SumChip({ label, value, icon, color }: { label: string; value: string; icon: string; color?: string }) {
  return (
    <View style={styles.sumChip}>
      <Ionicons name={icon as keyof typeof Ionicons.glyphMap} size={14} color={color ?? colors.textTertiary} />
      <Text style={styles.sumLabel}>{label}</Text>
      <Text style={[styles.sumValue, color ? { color } : null]}>{value}</Text>
    </View>
  );
}

function AgentTrafficCard({ agent }: { agent: AgentTrafficSummary }) {
  return (
    <View style={styles.card}>
      <Text style={styles.cardName} numberOfLines={1}>{agent.agent_name}</Text>
      <View style={styles.metricsRow}>
        <View style={styles.metric}>
          <Text style={styles.metricLabel}>上传</Text>
          <Text style={styles.metricValue}>{formatBytes(agent.total_in)}</Text>
        </View>
        <View style={styles.metric}>
          <Text style={styles.metricLabel}>下载</Text>
          <Text style={styles.metricValue}>{formatBytes(agent.total_out)}</Text>
        </View>
        <View style={styles.metric}>
          <Text style={styles.metricLabel}>总计</Text>
          <Text style={[styles.metricValue, { color: colors.primary }]}>{formatBytes(agent.total_used)}</Text>
        </View>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  list: { padding: spacing.lg },
  empty: { flex: 1, justifyContent: "center", alignItems: "center" },
  emptyBox: { alignItems: "center", gap: spacing.md },
  emptyText: { fontSize: fontSize.base, color: colors.textTertiary },
  summarySection: { gap: spacing.sm, marginBottom: spacing.lg },
  summaryRow: { flexDirection: "row", gap: spacing.sm },
  sumChip: {
    flex: 1,
    backgroundColor: colors.surface,
    borderRadius: radius.lg,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.md,
    alignItems: "center",
    gap: 2,
  },
  sumLabel: { fontSize: fontSize.xs, color: colors.textTertiary, fontWeight: "600" },
  sumValue: { fontSize: fontSize.lg, fontWeight: "800", color: colors.textPrimary },
  sectionTitle: { fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary, marginTop: spacing.md },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.md,
    gap: spacing.sm,
  },
  cardName: { fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
  metricsRow: { flexDirection: "row", gap: spacing.sm },
  metric: {
    flex: 1,
    backgroundColor: colors.surfaceHover,
    borderRadius: radius.md,
    padding: spacing.sm,
    alignItems: "center",
  },
  metricLabel: { fontSize: 8, fontWeight: "700", color: colors.textDisabled, letterSpacing: 0.5 },
  metricValue: { fontSize: fontSize.sm, fontWeight: "800", color: colors.textSecondary, marginTop: 2 },
});
