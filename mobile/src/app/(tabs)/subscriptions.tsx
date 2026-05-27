import { View, Text, FlatList, StyleSheet, TouchableOpacity, RefreshControl, Platform } from "react-native";
import { useState, useCallback } from "react";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useSubscriptionsQuery } from "../../api/subscription";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { Subscription } from "../../types/api";

export default function SubscriptionsScreen() {
  const { data, isLoading, refetch } = useSubscriptionsQuery();
  const [refreshing, setRefreshing] = useState(false);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const openDetail = (sub: Subscription) => {
    router.push(`/subscription/${sub.id}`);
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
      ListEmptyComponent={
        !isLoading ? (
          <View style={styles.emptyBox}>
            <Ionicons name="book-outline" size={48} color={colors.textDisabled} />
            <Text style={styles.emptyText}>暂无订阅</Text>
            <TouchableOpacity
              style={styles.emptyBtn}
              onPress={() => router.push("/subscription/create")}
              activeOpacity={0.7}
            >
              <Ionicons name="add-outline" size={16} color="#fff" />
              <Text style={styles.emptyBtnText}>新建订阅</Text>
            </TouchableOpacity>
          </View>
        ) : null
      }
      renderItem={({ item }) => (
        <TouchableOpacity style={styles.card} activeOpacity={0.7} onPress={() => openDetail(item)}>
          <View style={styles.cardTop}>
            <View style={[styles.statusDot, { backgroundColor: item.last_sync_status === "ok" ? colors.success : item.last_sync_status === "error" ? colors.error : colors.textDisabled }]} />
            <Text style={styles.cardName} numberOfLines={1}>{item.name}</Text>
            <View style={styles.badge}>
              <Text style={styles.badgeText}>{item.node_count} nodes</Text>
            </View>
          </View>
          {item.source_url && (
            <Text style={styles.cardUrl} numberOfLines={1}>{item.source_url}</Text>
          )}
          <View style={styles.cardBottom}>
            <Text style={styles.cardMeta}>{item.type}</Text>
            <View style={styles.cardActions}>
              <Ionicons name="share-outline" size={16} color={colors.textTertiary} />
            </View>
          </View>
        </TouchableOpacity>
      )}
    />
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  list: { padding: spacing.lg, gap: spacing.md },
  empty: { flex: 1, justifyContent: "center", alignItems: "center" },
  emptyBox: { alignItems: "center", gap: spacing.md },
  emptyText: { fontSize: fontSize.base, color: colors.textTertiary },
  emptyBtn: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
    backgroundColor: colors.primary,
    borderRadius: radius.lg,
    paddingHorizontal: spacing.xl,
    paddingVertical: spacing.md,
    marginTop: spacing.md,
  },
  emptyBtnText: { fontSize: fontSize.base, fontWeight: "700", color: "#fff" },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.md,
    gap: spacing.sm,
  },
  cardTop: { flexDirection: "row", alignItems: "center", gap: spacing.sm },
  statusDot: { width: 8, height: 8, borderRadius: 4 },
  cardName: { flex: 1, fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
  badge: { backgroundColor: colors.primarySoft, paddingHorizontal: spacing.sm, paddingVertical: 2, borderRadius: radius.sm },
  badgeText: { fontSize: fontSize.xs, fontWeight: "600", color: colors.primary },
  cardUrl: { fontSize: fontSize.xs, color: colors.textDisabled, fontFamily: Platform?.OS === "ios" ? "Menlo" : "monospace" },
  cardBottom: { flexDirection: "row", justifyContent: "space-between", alignItems: "center" },
  cardMeta: { fontSize: fontSize.xs, color: colors.textTertiary, textTransform: "uppercase", fontWeight: "600" },
  cardActions: { flexDirection: "row", gap: spacing.md },
});
