import { View, Text, ScrollView, StyleSheet, RefreshControl, TouchableOpacity } from "react-native";
import { useState, useCallback } from "react";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useSubscriptionsQuery, useSyncSubscription } from "../../api/subscription";
import { useVpsAssetSummaryQuery } from "../../api/vps-asset";
import { useAgentsQuery } from "../../api/agent";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import { useAuthStore } from "../../stores/auth-store";
import { Alert } from "react-native";

export default function DashboardScreen() {
  const user = useAuthStore((s) => s.user);
  const subs = useSubscriptionsQuery();
  const vpsSummary = useVpsAssetSummaryQuery();
  const agents = useAgentsQuery();
  const syncMutation = useSyncSubscription();

  const [refreshing, setRefreshing] = useState(false);
  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await Promise.all([subs.refetch(), vpsSummary.refetch(), agents.refetch()]);
    setRefreshing(false);
  }, []);

  const handleSyncAll = () => {
    const items = subs.data?.items ?? [];
    if (items.length === 0) {
      Alert.alert("提示", "暂无订阅可同步");
      return;
    }
    let completed = 0;
    let failed = 0;
    for (const sub of items) {
      syncMutation.mutate(sub.id, {
        onSuccess: () => {
          completed++;
          if (completed + failed === items.length) {
            Alert.alert("同步完成", `成功: ${completed}，失败: ${failed}`);
            subs.refetch();
          }
        },
        onError: () => {
          failed++;
          if (completed + failed === items.length) {
            Alert.alert("同步完成", `成功: ${completed}，失败: ${failed}`);
            subs.refetch();
          }
        },
      });
    }
  };

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

      {/* Quick actions */}
      <Text style={styles.sectionTitle}>快捷操作</Text>
      <View style={styles.quickActions}>
        <TouchableOpacity
          style={styles.actionBtn}
          onPress={() => router.push("/subscription/create")}
          activeOpacity={0.7}
        >
          <View style={[styles.actionIcon, { backgroundColor: colors.primarySoft }]}>
            <Ionicons name="add-circle-outline" size={20} color={colors.primary} />
          </View>
          <Text style={styles.actionText}>新建订阅</Text>
        </TouchableOpacity>
        <TouchableOpacity
          style={styles.actionBtn}
          onPress={handleSyncAll}
          activeOpacity={0.7}
        >
          <View style={[styles.actionIcon, { backgroundColor: colors.infoBg }]}>
            <Ionicons name="sync-outline" size={20} color={colors.info} />
          </View>
          <Text style={styles.actionText}>同步全部</Text>
        </TouchableOpacity>
        <TouchableOpacity
          style={styles.actionBtn}
          onPress={() => router.push("/(tabs)/nodes")}
          activeOpacity={0.7}
        >
          <View style={[styles.actionIcon, { backgroundColor: colors.successBg }]}>
            <Ionicons name="server-outline" size={20} color={colors.success} />
          </View>
          <Text style={styles.actionText}>查看节点</Text>
        </TouchableOpacity>
      </View>

      {/* Recent events placeholder */}
      <Text style={styles.sectionTitle}>近期动态</Text>
      <View style={styles.eventsCard}>
        <Ionicons name="newspaper-outline" size={24} color={colors.textDisabled} />
        <Text style={styles.eventsPlaceholder}>暂无近期动态</Text>
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
  sectionTitle: {
    fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary,
    marginTop: spacing.xxxl, marginBottom: spacing.md,
  },
  quickActions: {
    flexDirection: "row", gap: spacing.md,
  },
  actionBtn: {
    flex: 1, alignItems: "center", gap: spacing.sm,
    backgroundColor: colors.surface, borderRadius: radius.xl,
    borderWidth: 1, borderColor: colors.border,
    padding: spacing.lg,
  },
  actionIcon: {
    width: 40, height: 40, borderRadius: radius.md,
    justifyContent: "center", alignItems: "center",
  },
  actionText: {
    fontSize: fontSize.xs, fontWeight: "600", color: colors.textSecondary, textAlign: "center",
  },
  eventsCard: {
    backgroundColor: colors.surface, borderRadius: radius.xl,
    borderWidth: 1, borderColor: colors.border,
    padding: spacing.xl, alignItems: "center", justifyContent: "center",
    gap: spacing.sm, minHeight: 80,
  },
  eventsPlaceholder: {
    fontSize: fontSize.sm, color: colors.textDisabled,
  },
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
