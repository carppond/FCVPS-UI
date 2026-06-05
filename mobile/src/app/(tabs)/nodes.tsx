import { View, Text, FlatList, StyleSheet, RefreshControl, TouchableOpacity, Alert, TextInput, ScrollView } from "react-native";
import { useState, useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Ionicons } from "@expo/vector-icons";
import { useNodesQuery, useTcpingMutation } from "../../api/node";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";
import type { Node, NodeProtocol, TCPingResult } from "../../types/api";

// 协议筛选「全部」用非中文哨兵值,展示文案走 i18n。
const ALL = "__all__";

function protocolColor(protocol: NodeProtocol, c: AppColors): string {
  switch (protocol) {
    case "vmess": return c.info;
    case "vless": return c.success;
    case "ss": return c.warning;
    case "trojan": return c.primary;
    case "hysteria":
    case "hysteria2": return c.purple;
    case "tuic": return c.primary2;
    default: return c.textTertiary;
  }
}

export default function NodesScreen() {
  const { t } = useTranslation(["nodes", "common"]);
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const { data, isLoading, refetch } = useNodesQuery();
  const tcpingMutation = useTcpingMutation();
  const [refreshing, setRefreshing] = useState(false);
  const [latencyMap, setLatencyMap] = useState<Record<string, TCPingResult>>({});
  const [search, setSearch] = useState("");
  const [selectedProtocol, setSelectedProtocol] = useState<string>(ALL);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const allItems = data?.items ?? [];

  const protocols = useMemo(() => {
    const set = new Set(allItems.map((n) => n.protocol));
    return [ALL, ...Array.from(set)];
  }, [allItems]);

  const items = useMemo(() => {
    let filtered = allItems;
    if (selectedProtocol !== ALL) {
      filtered = filtered.filter((n) => n.protocol === selectedProtocol);
    }
    const q = search.trim().toLowerCase();
    if (q) {
      filtered = filtered.filter((n) => {
        const tag = n.tag?.toLowerCase() ?? "";
        const server = n.server?.toLowerCase() ?? "";
        const tags = n.tags?.join(" ").toLowerCase() ?? "";
        return tag.includes(q) || server.includes(q) || tags.includes(q);
      });
    }
    return filtered;
  }, [allItems, search, selectedProtocol]);

  const handleTcping = () => {
    if (items.length === 0) {
      Alert.alert(t("common:tip"), t("no_nodes_to_test"));
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
          Alert.alert(t("tcping_done"), t("tcping_done_message", { count: resp.results.length }));
        },
        onError: (err: any) => Alert.alert(t("tcping_failed"), err.message),
      },
    );
  };

  return (
    <FlatList
      style={styles.container}
      contentContainerStyle={items.length === 0 && !search && selectedProtocol === ALL ? styles.empty : styles.list}
      data={items}
      keyExtractor={(item) => item.id}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor={colors.primary} />
      }
      ListHeaderComponent={
        allItems.length > 0 ? (
          <View>
            <View style={styles.searchBar}>
              <Ionicons name="search-outline" size={16} color={colors.textTertiary} />
              <TextInput
                style={styles.searchInput}
                value={search}
                onChangeText={setSearch}
                placeholder={t("search_placeholder")}
                placeholderTextColor={colors.textDisabled}
              />
              {search ? (
                <TouchableOpacity onPress={() => setSearch("")}>
                  <Ionicons name="close-circle" size={16} color={colors.textDisabled} />
                </TouchableOpacity>
              ) : null}
            </View>
            <ScrollView horizontal showsHorizontalScrollIndicator={false} style={styles.filterRow} contentContainerStyle={styles.filterRowContent}>
              {protocols.map((p) => (
                <TouchableOpacity
                  key={p}
                  style={[styles.filterChip, selectedProtocol === p && styles.filterChipActive]}
                  onPress={() => setSelectedProtocol(p)}
                  activeOpacity={0.7}
                >
                  <Text style={[styles.filterChipText, selectedProtocol === p && styles.filterChipTextActive]}>
                    {p === ALL ? t("common:all") : p.toUpperCase()}
                  </Text>
                </TouchableOpacity>
              ))}
            </ScrollView>
            <TouchableOpacity
              style={[styles.tcpingBtn, tcpingMutation.isPending && styles.tcpingBtnDisabled]}
              onPress={handleTcping}
              disabled={tcpingMutation.isPending}
              activeOpacity={0.7}
            >
              <Ionicons name="speedometer-outline" size={16} color="#fff" />
              <Text style={styles.tcpingBtnText}>
                {tcpingMutation.isPending ? t("tcping_running") : t("tcping")}
              </Text>
            </TouchableOpacity>
          </View>
        ) : null
      }
      ListEmptyComponent={
        !isLoading ? (
          <View style={styles.emptyBox}>
            <Ionicons name="server-outline" size={48} color={colors.textDisabled} />
            <Text style={styles.emptyText}>{t("no_nodes")}</Text>
          </View>
        ) : null
      }
      renderItem={({ item }) => <NodeCard node={item} latency={latencyMap[item.id]} />}
    />
  );
}

function NodeCard({ node, latency }: { node: Node; latency?: TCPingResult }) {
  const { t } = useTranslation("nodes");
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const pc = protocolColor(node.protocol, colors);

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
              {latency.reachable ? `${latency.latency_ms}ms` : t("timeout")}
            </Text>
          </View>
        )}
      </View>
      <Text style={styles.serverText} numberOfLines={1}>
        {node.server}:{node.port}
      </Text>
      {node.tags.length > 0 && (
        <View style={styles.tagsRow}>
          {node.tags.map((tag) => (
            <View key={tag} style={styles.tagChip}>
              <Text style={styles.tagText}>{tag}</Text>
            </View>
          ))}
        </View>
      )}
    </View>
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  list: { padding: spacing.lg },
  empty: { flex: 1, justifyContent: "center", alignItems: "center" },
  searchBar: {
    flexDirection: "row", alignItems: "center", gap: spacing.sm,
    backgroundColor: colors.surface, borderRadius: radius.lg,
    borderWidth: 1, borderColor: colors.border,
    paddingHorizontal: spacing.md, height: 40, marginBottom: spacing.md,
  },
  searchInput: {
    flex: 1, fontSize: fontSize.sm, color: colors.textPrimary,
  },
  filterRow: { marginBottom: spacing.md },
  filterRowContent: { gap: spacing.sm },
  filterChip: {
    paddingHorizontal: spacing.md, paddingVertical: spacing.xs,
    borderRadius: radius.lg, backgroundColor: colors.surface,
    borderWidth: 1, borderColor: colors.border,
  },
  filterChipActive: {
    backgroundColor: colors.primary, borderColor: colors.primary,
  },
  filterChipText: {
    fontSize: fontSize.xs, fontWeight: "600", color: colors.textTertiary,
  },
  filterChipTextActive: {
    color: "#fff",
  },
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
