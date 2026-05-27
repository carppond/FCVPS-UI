import { View, Text, FlatList, StyleSheet, RefreshControl, TouchableOpacity, Alert, Modal } from "react-native";
import { useState, useCallback } from "react";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useAgentsQuery, useDeleteAgent } from "../../api/agent";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { AgentListItem } from "../../types/api";

export default function AgentsScreen() {
  const { data, isLoading, refetch } = useAgentsQuery();
  const deleteMutation = useDeleteAgent();
  const [refreshing, setRefreshing] = useState(false);
  const [menuVisible, setMenuVisible] = useState(false);
  const [selectedAgent, setSelectedAgent] = useState<AgentListItem | null>(null);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const openMenu = (agent: AgentListItem) => {
    setSelectedAgent(agent);
    setMenuVisible(true);
  };

  const closeMenu = () => {
    setMenuVisible(false);
    setSelectedAgent(null);
  };

  const handleEdit = () => {
    if (!selectedAgent) return;
    closeMenu();
    router.push(`/agent/edit?id=${selectedAgent.id}&name=${encodeURIComponent(selectedAgent.name)}`);
  };

  const handleDelete = () => {
    if (!selectedAgent) return;
    const agentId = selectedAgent.id;
    const agentName = selectedAgent.name;
    closeMenu();
    Alert.alert("确认删除", `确定要删除探针「${agentName}」吗？`, [
      { text: "取消", style: "cancel" },
      {
        text: "删除",
        style: "destructive",
        onPress: () => {
          deleteMutation.mutate(agentId, {
            onSuccess: () => Alert.alert("已删除", "探针已删除"),
            onError: (err: any) => Alert.alert("删除失败", err.message),
          });
        },
      },
    ]);
  };

  return (
    <>
    <FlatList
      style={styles.container}
      contentContainerStyle={items.length === 0 ? styles.empty : styles.list}
      data={items}
      keyExtractor={(item) => item.id}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor={colors.primary} />
      }
      ListHeaderComponent={
        items.length > 0 ? (
          <TouchableOpacity
            style={styles.addBtn}
            onPress={() => router.push("/agent/create")}
            activeOpacity={0.7}
          >
            <Ionicons name="add-circle-outline" size={18} color={colors.primary} />
            <Text style={styles.addBtnText}>新建探针</Text>
          </TouchableOpacity>
        ) : null
      }
      ListEmptyComponent={
        !isLoading ? (
          <View style={styles.emptyBox}>
            <Ionicons name="radio-outline" size={48} color={colors.textDisabled} />
            <Text style={styles.emptyText}>暂无探针</Text>
            <TouchableOpacity
              style={styles.emptyBtn}
              onPress={() => router.push("/agent/create")}
              activeOpacity={0.7}
            >
              <Ionicons name="add-outline" size={16} color="#fff" />
              <Text style={styles.emptyBtnText}>新建探针</Text>
            </TouchableOpacity>
          </View>
        ) : null
      }
      renderItem={({ item }) => <AgentCard agent={item} onPress={(a) => router.push(`/agent/${a.id}`)} onLongPress={openMenu} />}
    />

    {/* Action menu modal */}
    <Modal visible={menuVisible} animationType="fade" transparent onRequestClose={closeMenu}>
      <TouchableOpacity style={styles.modalOverlay} activeOpacity={1} onPress={closeMenu}>
        <View style={styles.menuSheet}>
          <Text style={styles.menuTitle} numberOfLines={1}>{selectedAgent?.name}</Text>
          <TouchableOpacity style={styles.menuItem} onPress={handleEdit} activeOpacity={0.6}>
            <Ionicons name="create-outline" size={20} color={colors.primary} />
            <Text style={styles.menuItemText}>编辑</Text>
          </TouchableOpacity>
          <TouchableOpacity style={styles.menuItem} onPress={handleDelete} activeOpacity={0.6}>
            <Ionicons name="trash-outline" size={20} color={colors.error} />
            <Text style={[styles.menuItemText, { color: colors.error }]}>删除</Text>
          </TouchableOpacity>
          <TouchableOpacity style={styles.menuCancel} onPress={closeMenu} activeOpacity={0.6}>
            <Text style={styles.menuCancelText}>取消</Text>
          </TouchableOpacity>
        </View>
      </TouchableOpacity>
    </Modal>
    </>
  );
}

function AgentCard({ agent, onPress, onLongPress }: { agent: AgentListItem; onPress: (a: AgentListItem) => void; onLongPress: (a: AgentListItem) => void }) {
  const online = agent.online;
  const cpu = agent.latest_metrics?.cpu_percent;
  const memUsed = agent.latest_metrics?.mem_used;
  const memTotal = agent.latest_metrics?.mem_total;
  const memPct = memTotal ? Math.round((memUsed ?? 0) / memTotal * 100) : null;

  return (
    <TouchableOpacity style={[styles.card, online ? styles.cardOnline : styles.cardOffline]} onPress={() => onPress(agent)} onLongPress={() => onLongPress(agent)} activeOpacity={0.7}>
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
    </TouchableOpacity>
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
  emptyBtn: {
    flexDirection: "row", alignItems: "center", gap: spacing.xs,
    backgroundColor: colors.primary, borderRadius: radius.lg,
    paddingHorizontal: spacing.xl, paddingVertical: spacing.md, marginTop: spacing.md,
  },
  emptyBtnText: { fontSize: fontSize.base, fontWeight: "700", color: "#fff" },
  addBtn: {
    flexDirection: "row", alignItems: "center", justifyContent: "center", gap: spacing.xs,
    backgroundColor: colors.primarySoft, borderRadius: radius.lg,
    paddingVertical: spacing.md, marginBottom: spacing.md,
  },
  addBtnText: { fontSize: fontSize.sm, fontWeight: "600", color: colors.primary },
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
  modalOverlay: { flex: 1, backgroundColor: "rgba(0,0,0,0.4)", justifyContent: "flex-end" },
  menuSheet: { backgroundColor: colors.surface, borderTopLeftRadius: radius.xl, borderTopRightRadius: radius.xl, padding: spacing.xl, paddingBottom: 40 },
  menuTitle: { fontSize: fontSize.lg, fontWeight: "700", color: colors.textPrimary, marginBottom: spacing.lg, textAlign: "center" },
  menuItem: { flexDirection: "row", alignItems: "center", gap: spacing.md, paddingVertical: spacing.md },
  menuItemText: { fontSize: fontSize.base, fontWeight: "500", color: colors.textPrimary },
  menuCancel: { alignItems: "center", paddingVertical: spacing.md, marginTop: spacing.sm, borderTopWidth: 1, borderTopColor: colors.border },
  menuCancelText: { fontSize: fontSize.base, fontWeight: "600", color: colors.textTertiary },
});
