import { View, Text, FlatList, StyleSheet, RefreshControl } from "react-native";
import { useState, useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Ionicons } from "@expo/vector-icons";
import { useNodesQuery } from "../../api/node";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";
import type { Node, NodeProtocol } from "../../types/api";

function protocolColor(protocol: NodeProtocol, c: AppColors): string {
  switch (protocol) {
    case "vmess": return c.info;
    case "vless": return c.success;
    case "ss": return c.warning;
    case "trojan": return c.primary;
    case "hysteria":
    case "hysteria2": return "#c084fc";
    case "tuic": return "#fb923c";
    default: return c.textTertiary;
  }
}

export default function NodesPage() {
  const { t } = useTranslation("nodes");
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const { data, isLoading, refetch } = useNodesQuery();
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
            <Ionicons name="server-outline" size={48} color={colors.textDisabled} />
            <Text style={styles.emptyText}>{t("no_nodes")}</Text>
          </View>
        ) : null
      }
      renderItem={({ item }) => <NodeCard node={item} />}
    />
  );
}

function NodeCard({ node }: { node: Node }) {
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
  emptyBox: { alignItems: "center", gap: spacing.md },
  emptyText: { fontSize: fontSize.base, color: colors.textTertiary },
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
