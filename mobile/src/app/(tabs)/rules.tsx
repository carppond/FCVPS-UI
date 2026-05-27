import { View, Text, FlatList, StyleSheet, RefreshControl } from "react-native";
import { useState, useCallback } from "react";
import { Ionicons } from "@expo/vector-icons";
import { useRulesQuery } from "../../api/rule";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { CustomRule, RuleType, RuleMode } from "../../types/api";

function typeColor(type: RuleType): string {
  switch (type) {
    case "dns": return colors.info;
    case "rules": return colors.success;
    case "rule-providers": return "#c084fc";
  }
}

function modeLabel(mode: RuleMode): string {
  switch (mode) {
    case "replace": return "替换";
    case "prepend": return "前置";
    case "append": return "追加";
  }
}

function modeColor(mode: RuleMode): string {
  switch (mode) {
    case "replace": return colors.warning;
    case "prepend": return colors.info;
    case "append": return colors.success;
  }
}

export default function RulesScreen() {
  const { data, isLoading, refetch } = useRulesQuery();
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
            <Ionicons name="shield-outline" size={48} color={colors.textDisabled} />
            <Text style={styles.emptyText}>暂无规则</Text>
          </View>
        ) : null
      }
      renderItem={({ item }) => <RuleCard rule={item} />}
    />
  );
}

function RuleCard({ rule }: { rule: CustomRule }) {
  const tc = typeColor(rule.type);
  const mc = modeColor(rule.mode);

  return (
    <View style={[styles.card, !rule.enabled && styles.cardDisabled]}>
      <View style={styles.cardHeader}>
        <Text style={styles.cardName} numberOfLines={1}>{rule.name}</Text>
        <View style={[styles.enabledDot, { backgroundColor: rule.enabled ? colors.success : colors.textDisabled }]} />
      </View>
      <View style={styles.badgesRow}>
        <View style={[styles.badge, { backgroundColor: tc + "1a" }]}>
          <Text style={[styles.badgeText, { color: tc }]}>{rule.type.toUpperCase()}</Text>
        </View>
        <View style={[styles.badge, { backgroundColor: mc + "1a" }]}>
          <Text style={[styles.badgeText, { color: mc }]}>{modeLabel(rule.mode)}</Text>
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
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.md,
    gap: spacing.sm,
  },
  cardDisabled: { opacity: 0.5 },
  cardHeader: { flexDirection: "row", alignItems: "center", gap: spacing.sm },
  cardName: { flex: 1, fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
  enabledDot: { width: 8, height: 8, borderRadius: 4 },
  badgesRow: { flexDirection: "row", gap: spacing.sm },
  badge: {
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: radius.sm,
  },
  badgeText: { fontSize: fontSize.xs, fontWeight: "700", letterSpacing: 0.5 },
});
