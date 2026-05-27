import { View, Text, ScrollView, StyleSheet, RefreshControl } from "react-native";
import { useState, useCallback } from "react";
import { Ionicons } from "@expo/vector-icons";
import { useSubscriptionsQuery } from "../../api/subscription";
import { useVpsAssetSummaryQuery } from "../../api/vps-asset";
import { useAgentsQuery } from "../../api/agent";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import { useAuthStore } from "../../stores/auth-store";

export default function DashboardScreen() {
  const user = useAuthStore((s) => s.user);
  const subs = useSubscriptionsQuery();
  const vpsSummary = useVpsAssetSummaryQuery();
  const agents = useAgentsQuery();

  const [refreshing, setRefreshing] = useState(false);
  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await Promise.all([subs.refetch(), vpsSummary.refetch(), agents.refetch()]);
    setRefreshing(false);
  }, []);

  const subCount = subs.data?.items?.length ?? 0;
  const nodeCount = subs.data?.items?.reduce((acc, s) => acc + s.node_count, 0) ?? 0;
  const vpsTotal = vpsSummary.data?.total ?? 0;
  const vpsExpiring = vpsSummary.data?.expiring ?? 0;
  const agentOnline = agents.data?.items?.filter((a) => a.online).length ?? 0;
  const agentTotal = agents.data?.items?.length ?? 0;

  return (
    <ScrollView
      style={styles.container}
      contentContainerStyle={styles.content}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor={colors.primary} />
      }
    >
      <Text style={styles.greeting}>你好，{user?.username ?? "Admin"}</Text>
      <Text style={styles.greetingSub}>基础设施一览</Text>

      <View style={styles.grid}>
        <StatCard
          icon="book-outline"
          iconColor={colors.primary}
          iconBg={colors.primarySoft}
          label="订阅"
          value={String(subCount)}
          sub={`${nodeCount} 个节点`}
        />
        <StatCard
          icon="hardware-chip-outline"
          iconColor={colors.info}
          iconBg={colors.infoBg}
          label="VPS 资产"
          value={String(vpsTotal)}
          sub={vpsExpiring > 0 ? `${vpsExpiring} 台即将到期` : "全部正常"}
          highlight={vpsExpiring > 0}
        />
        <StatCard
          icon="radio-outline"
          iconColor={colors.success}
          iconBg={colors.successBg}
          label="探针"
          value={`${agentOnline}/${agentTotal}`}
          sub="在线"
        />
        <StatCard
          icon="shield-checkmark-outline"
          iconColor={colors.warning}
          iconBg={colors.warningBg}
          label="即将到期"
          value={String(vpsExpiring)}
          sub={vpsExpiring > 0 ? "需要关注" : "无"}
          highlight={vpsExpiring > 0}
        />
      </View>
    </ScrollView>
  );
}

function StatCard({
  icon,
  iconColor,
  iconBg,
  label,
  value,
  sub,
  highlight,
}: {
  icon: keyof typeof Ionicons.glyphMap;
  iconColor: string;
  iconBg: string;
  label: string;
  value: string;
  sub: string;
  highlight?: boolean;
}) {
  return (
    <View style={styles.card}>
      <View style={[styles.cardIcon, { backgroundColor: iconBg }]}>
        <Ionicons name={icon} size={18} color={iconColor} />
      </View>
      <Text style={styles.cardLabel}>{label}</Text>
      <Text style={styles.cardValue}>{value}</Text>
      <Text style={[styles.cardSub, highlight && { color: colors.warning }]}>{sub}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.xl },
  greeting: { fontSize: fontSize.xxxl, fontWeight: "800", color: colors.textPrimary, letterSpacing: -0.5 },
  greetingSub: { fontSize: fontSize.sm, color: colors.textTertiary, marginTop: 6, marginBottom: spacing.xxxl },
  grid: { flexDirection: "row", flexWrap: "wrap", gap: spacing.md },
  card: {
    width: "48%",
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    gap: spacing.xs,
  },
  cardIcon: { width: 36, height: 36, borderRadius: radius.md, justifyContent: "center", alignItems: "center", marginBottom: spacing.xs },
  cardLabel: { fontSize: fontSize.xs, fontWeight: "600", color: colors.textTertiary, textTransform: "uppercase", letterSpacing: 0.5 },
  cardValue: { fontSize: 28, fontWeight: "800", color: colors.textPrimary },
  cardSub: { fontSize: fontSize.xs, color: colors.textTertiary },
});
