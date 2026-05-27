import { View, Text, ScrollView, StyleSheet, RefreshControl, ActivityIndicator } from "react-native";
import { useLocalSearchParams } from "expo-router";
import { useState, useCallback } from "react";
import { Ionicons } from "@expo/vector-icons";
import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../../lib/api-client";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { AgentListItem } from "../../types/api";

function formatBytes(bytes: number): string {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}天 ${h}小时`;
  if (h > 0) return `${h}小时 ${m}分钟`;
  return `${m}分钟`;
}

export default function AgentDetailScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const [refreshing, setRefreshing] = useState(false);

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["agent", "detail", id],
    queryFn: () => apiFetch<AgentListItem>(`/api/agents/${id}`),
    enabled: !!id,
    refetchInterval: 10_000,
  });

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  if (isLoading || !data) {
    return (
      <View style={styles.loading}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }

  const m = data.latest_metrics;
  const online = data.online;
  const memPct = m?.mem_total ? Math.round((m.mem_used / m.mem_total) * 100) : null;
  const diskPct = m?.disk_total ? Math.round((m.disk_used / m.disk_total) * 100) : null;
  const swapPct = m?.swap_total ? Math.round((m.swap_used / m.swap_total) * 100) : null;

  return (
    <ScrollView
      style={styles.container}
      contentContainerStyle={styles.content}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor={colors.primary} />}
    >
      {/* Status card */}
      <View style={styles.statusCard}>
        <View style={[styles.statusDot, { backgroundColor: online ? colors.success : colors.error }]} />
        <View style={{ flex: 1 }}>
          <Text style={styles.agentName}>{data.name}</Text>
          <Text style={styles.agentMeta}>{data.kind} · {online ? "在线" : "离线"}</Text>
        </View>
      </View>

      {/* Info section */}
      <View style={styles.card}>
        <Text style={styles.sectionTitle}>基本信息</Text>
        <InfoRow label="IP 地址" value={data.public_ip || "—"} mono />
        <InfoRow label="操作系统" value={data.os ? `${data.os} ${data.arch ?? ""}` : "—"} />
        <InfoRow label="版本" value={data.version || "—"} />
        <InfoRow label="类型" value={data.kind === "native" ? "原生 Agent" : "哪吒兼容"} />
      </View>

      {/* Metrics */}
      {online && m && (
        <>
          <View style={styles.card}>
            <Text style={styles.sectionTitle}>系统指标</Text>
            <View style={styles.metricsGrid}>
              <MetricCard label="CPU" value={`${Math.round(m.cpu_percent)}%`} color={m.cpu_percent > 80 ? colors.error : m.cpu_percent > 50 ? colors.warning : colors.success} pct={m.cpu_percent} />
              {memPct !== null && <MetricCard label="内存" value={`${memPct}%`} sub={`${formatBytes(m.mem_used)} / ${formatBytes(m.mem_total)}`} color={memPct > 80 ? colors.error : memPct > 50 ? colors.warning : colors.info} pct={memPct} />}
              {diskPct !== null && <MetricCard label="磁盘" value={`${diskPct}%`} sub={`${formatBytes(m.disk_used)} / ${formatBytes(m.disk_total)}`} color={diskPct > 80 ? colors.error : colors.info} pct={diskPct} />}
              {swapPct !== null && m.swap_total > 0 && <MetricCard label="Swap" value={`${swapPct}%`} sub={`${formatBytes(m.swap_used)} / ${formatBytes(m.swap_total)}`} color={colors.textTertiary} pct={swapPct} />}
            </View>
          </View>

          <View style={styles.card}>
            <Text style={styles.sectionTitle}>网络</Text>
            <View style={styles.netRow}>
              <View style={styles.netItem}>
                <Ionicons name="arrow-up-outline" size={14} color={colors.success} />
                <Text style={styles.netLabel}>上行速度</Text>
                <Text style={styles.netValue}>{formatBytes(m.net_out_speed)}/s</Text>
              </View>
              <View style={styles.netItem}>
                <Ionicons name="arrow-down-outline" size={14} color={colors.info} />
                <Text style={styles.netLabel}>下行速度</Text>
                <Text style={styles.netValue}>{formatBytes(m.net_in_speed)}/s</Text>
              </View>
            </View>
            <View style={styles.netRow}>
              <View style={styles.netItem}>
                <Ionicons name="cloud-upload-outline" size={14} color={colors.textTertiary} />
                <Text style={styles.netLabel}>总上行</Text>
                <Text style={styles.netValue}>{formatBytes(m.net_out)}</Text>
              </View>
              <View style={styles.netItem}>
                <Ionicons name="cloud-download-outline" size={14} color={colors.textTertiary} />
                <Text style={styles.netLabel}>总下行</Text>
                <Text style={styles.netValue}>{formatBytes(m.net_in)}</Text>
              </View>
            </View>
          </View>

          <View style={styles.card}>
            <Text style={styles.sectionTitle}>其他</Text>
            <InfoRow label="运行时间" value={formatUptime(m.uptime)} />
            <InfoRow label="负载 (1/5/15)" value={`${m.load1.toFixed(2)} / ${m.load5.toFixed(2)} / ${m.load15.toFixed(2)}`} />
            <InfoRow label="TCP 连接" value={String(m.conn_tcp)} />
            <InfoRow label="UDP 连接" value={String(m.conn_udp)} />
            {m.process_count !== undefined && <InfoRow label="进程数" value={String(m.process_count)} />}
          </View>
        </>
      )}

      {!online && (
        <View style={styles.offlineCard}>
          <Ionicons name="cloud-offline-outline" size={36} color={colors.error} />
          <Text style={styles.offlineText}>探针离线，无法获取指标</Text>
        </View>
      )}
    </ScrollView>
  );
}

function InfoRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <View style={styles.infoRow}>
      <Text style={styles.infoLabel}>{label}</Text>
      <Text style={[styles.infoValue, mono && { fontFamily: "monospace" }]} numberOfLines={1}>{value}</Text>
    </View>
  );
}

function MetricCard({ label, value, sub, color, pct }: { label: string; value: string; sub?: string; color: string; pct: number }) {
  return (
    <View style={styles.metricCard}>
      <Text style={styles.metricLabel}>{label}</Text>
      <Text style={[styles.metricValue, { color }]}>{value}</Text>
      <View style={styles.progressBg}>
        <View style={[styles.progressFill, { width: `${Math.min(100, pct)}%`, backgroundColor: color }]} />
      </View>
      {sub && <Text style={styles.metricSub}>{sub}</Text>}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.xl, paddingBottom: 40 },
  loading: { flex: 1, justifyContent: "center", alignItems: "center", backgroundColor: colors.bg },
  statusCard: {
    flexDirection: "row", alignItems: "center", gap: spacing.md,
    backgroundColor: colors.surface, borderRadius: radius.xl,
    borderWidth: 1, borderColor: colors.border, padding: spacing.xl, marginBottom: spacing.lg,
  },
  statusDot: { width: 12, height: 12, borderRadius: 6 },
  agentName: { fontSize: fontSize.xl, fontWeight: "800", color: colors.textPrimary },
  agentMeta: { fontSize: fontSize.sm, color: colors.textTertiary, marginTop: 2 },
  card: {
    backgroundColor: colors.surface, borderRadius: radius.xl,
    borderWidth: 1, borderColor: colors.border, padding: spacing.xl, marginBottom: spacing.lg,
  },
  sectionTitle: {
    fontSize: fontSize.xs, fontWeight: "700", color: colors.textDisabled,
    textTransform: "uppercase", letterSpacing: 0.5, marginBottom: spacing.md,
  },
  infoRow: {
    flexDirection: "row", justifyContent: "space-between", alignItems: "center",
    paddingVertical: spacing.sm, borderBottomWidth: 1, borderBottomColor: colors.border,
  },
  infoLabel: { fontSize: fontSize.sm, color: colors.textTertiary },
  infoValue: { fontSize: fontSize.sm, fontWeight: "600", color: colors.textPrimary, maxWidth: "60%" },
  metricsGrid: { flexDirection: "row", flexWrap: "wrap", gap: spacing.sm },
  metricCard: {
    width: "48%", backgroundColor: colors.elevated, borderRadius: radius.lg,
    padding: spacing.md, gap: spacing.xs,
  },
  metricLabel: { fontSize: fontSize.xs, fontWeight: "700", color: colors.textDisabled, textTransform: "uppercase" },
  metricValue: { fontSize: 22, fontWeight: "800" },
  metricSub: { fontSize: fontSize.xs, color: colors.textTertiary },
  progressBg: { height: 4, backgroundColor: colors.border, borderRadius: 2, overflow: "hidden" },
  progressFill: { height: 4, borderRadius: 2 },
  netRow: { flexDirection: "row", gap: spacing.sm, marginBottom: spacing.sm },
  netItem: {
    flex: 1, flexDirection: "row", alignItems: "center", gap: spacing.xs,
    backgroundColor: colors.elevated, borderRadius: radius.md, padding: spacing.sm,
  },
  netLabel: { fontSize: fontSize.xs, color: colors.textTertiary, flex: 1 },
  netValue: { fontSize: fontSize.sm, fontWeight: "700", color: colors.textPrimary },
  offlineCard: {
    alignItems: "center", gap: spacing.md, padding: spacing.xxxl,
    backgroundColor: colors.surface, borderRadius: radius.xl,
    borderWidth: 1, borderColor: colors.border,
  },
  offlineText: { fontSize: fontSize.sm, color: colors.textTertiary },
});
