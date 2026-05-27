import { View, Text, FlatList, StyleSheet, RefreshControl, TouchableOpacity, Alert } from "react-native";
import { useState, useCallback } from "react";
import { Ionicons } from "@expo/vector-icons";
import { useNodesQuery, useTcpingMutation } from "../../api/node";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { Node, NodeProtocol, TCPingResult } from "../../types/api";

function protocolColor(protocol: NodeProtocol): string {
  switch (protocol) {
    case "vmess": return colors.info;
    case "vless": return colors.success;
    case "ss": return colors.warning;
    case "trojan": return colors.primary;
    case "hysteria":
    case "hysteria2": return "#c084fc";
    case "tuic": return "#fb923c";
    default: return colors.textTertiary;
  }
}

export default function NodesScreen() {
  const { data, isLoading, refetch } = useNodesQuery();
  const tcpingMutation = useTcpingMutation();
  const [refreshing, setRefreshing] = useState(false);
  const [latencyMap, setLatencyMap] = useState<Record<string, TCPingResult>>({});

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const handleTcping = () => {
    if (items.length === 0) {
      Alert.alert("提示", "暂无节点可测速");
      return;
    }
    const nodeIds = items.map((n) => n.id);
    tcpingMutation.mutate(
      { node_ids: nodeIds, timeout_ms: 5000 },
      {
        onSuccess: (resp) => {
          const map: Record<string, TCPingResult> = {};
          for (const r of resp.results) {
            map[r.node_id] = r;
          }
          setLatencyMap(map);
          Alert.alert("测速完成", `已测试 ${resp.results.length} 个节点`);
        },
        onError: (err: any) => Alert.alert("测速失败", err.message),
      },
    );
  };

  return (
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
            style={[styles.tcpingBtn, tcpingMutation.isPending && styles.tcpingBtnDisabled]}
            onPress={handleTcping}
            disabled={tcpingMutation.isPending}
            activeOpacity={0.7}
          >
            <Ionicons name="speedometer-outline" size={16} color="#fff" />
            <Text style={styles.tcpingBtnText}>
              {tcpingMutation.isPending ? "测速中..." : "测速"}
            </Text>
          </TouchableOpacity>
        ) : null
      }
      ListEmptyComponent={
        !isLoading ? (
          <View style={styles.emptyBox}>
            <Ionicons name="server-outline" size={48} color={colors.textDisabled} />
            <Text style={styles.emptyText}>暂无节点</Text>
          </View>
        ) : null
      }
      renderItem={({ item }) => <NodeCard node={item} latency={latencyMap[item.id]} />}
    />
  );
}

function NodeCard({ node, latency }: { node: Node; latency?: TCPingResult }) {
  const pc = protocolColor(node.protocol);

  return (
    <View style={styles.card}>
      <View style={styles.cardHeader}>
        <View style={[styles.protocolBadge, { backgroundColor: pc + "1a" }]}>
          <Text style={[styles.protocolText, { color: pc }]}>{node.protocol.toUpperCase()}</Text>
        </View>
        <Text style={styles.cardName} numberOfLines={1}>{node.tag}</Text>
        {latency && (
          <View style={[
            styles.latencyBadge,
            { backgroundColor: latency.reachable ? colors.successBg : colors.errorBg },
          ]}>
            <Text style={[
              styles.latencyText,
              { color: latency.reachable ? colors.success : colors.error },
            ]}>
              {latency.reachable ? `${latency.latency_ms}ms` : "超时"}
            </Text>
          </View>
        )}
      </View>
      <Text style={styles.serverText} numberOfLines={1}>
        {node.server}:{node.port}
      </Text>
      {node.tags.length > 0 && (
        <View style={styles.tagsRow}>
          {node.tags.map((t) => (
            <View key={t} style={styles.tagChip}>
              <Text style={styles.tagText}>{t}</Text>
            </View>
          ))}
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  list: { padding: spacing.lg },
  empty: { flex: 1, justifyContent: "center", alignItems: "center" },
  emptyBox: { alignItems: "center", gap: spacing.md },
  emptyText: { fontSize: fontSize.base, color: colors.textTertiary },
  tcpingBtn: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.xs,
    backgroundColor: colors.primary,
    borderRadius: radius.lg,
    paddingVertical: spacing.md,
    marginBottom: spacing.lg,
  },
  tcpingBtnDisabled: { opacity: 0.5 },
  tcpingBtnText: { fontSize: fontSize.base, fontWeight: "700", color: "#fff" },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.md,
    gap: spacing.sm,
  },
  cardHeader: { flexDirection: "row", alignItems: "center", gap: spacing.sm },
  protocolBadge: {
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: radius.sm,
  },
  protocolText: { fontSize: fontSize.xs, fontWeight: "700", letterSpacing: 0.5 },
  cardName: { flex: 1, fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
  latencyBadge: {
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: radius.sm,
  },
  latencyText: { fontSize: fontSize.xs, fontWeight: "700" },
  serverText: { fontSize: fontSize.xs, color: colors.textSecondary, fontFamily: "monospace" },
  tagsRow: { flexDirection: "row", flexWrap: "wrap", gap: spacing.xs },
  tagChip: {
    backgroundColor: colors.surfaceHover,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: radius.sm,
  },
  tagText: { fontSize: fontSize.xs, color: colors.textTertiary, fontWeight: "600" },
});
