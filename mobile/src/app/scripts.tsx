import { useState, useCallback, useMemo } from "react";
import {
  View,
  Text,
  FlatList,
  StyleSheet,
  RefreshControl,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useScriptsQuery } from "../api/script";
import { spacing, radius, fontSize, type AppColors } from "../lib/theme";
import { useColors } from "../lib/useColors";
import type { Script, HookType } from "../types/api";

function hookLabel(hook: HookType, t: TFunction): string {
  const map: Record<HookType, string> = {
    pre_save_nodes: t("script_hook_pre_save_nodes"),
    post_fetch: t("script_hook_post_fetch"),
  };
  return map[hook] ?? hook;
}

function hookColor(hook: HookType, c: AppColors): string {
  switch (hook) {
    case "pre_save_nodes":
      return c.info;
    case "post_fetch":
      return c.warning;
    default:
      return c.textTertiary;
  }
}

function formatDate(ts: number | undefined, t: TFunction): string {
  if (!ts) return t("script_never_run");
  const d = new Date(ts * 1000);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")} ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

export default function ScriptsScreen() {
  const { t } = useTranslation("rules");
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const { data, isLoading, refetch } = useScriptsQuery();
  const [refreshing, setRefreshing] = useState(false);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const renderItem = ({ item }: { item: Script }) => (
    <View style={styles.card}>
      <View style={styles.cardTop}>
        <View
          style={[
            styles.enabledChip,
            {
              backgroundColor: item.enabled
                ? colors.successBg
                : "rgba(255,255,255,0.04)",
            },
          ]}
        >
          <View
            style={[
              styles.dot,
              {
                backgroundColor: item.enabled
                  ? colors.success
                  : colors.textDisabled,
              },
            ]}
          />
        </View>
        <View style={styles.cardInfo}>
          <Text style={styles.cardName} numberOfLines={1}>
            {item.name}
          </Text>
          <View style={styles.badgeRow}>
            <View
              style={[
                styles.badge,
                { backgroundColor: hookColor(item.hook, colors) + "1a" },
              ]}
            >
              <Text
                style={[styles.badgeText, { color: hookColor(item.hook, colors) }]}
              >
                {hookLabel(item.hook, t)}
              </Text>
            </View>
            <Text style={styles.runText}>
              {formatDate(item.last_run_at, t)}
            </Text>
          </View>
        </View>
      </View>
      {item.last_error ? (
        <Text style={styles.errorText} numberOfLines={2}>
          {item.last_error}
        </Text>
      ) : null}
    </View>
  );

  return (
    <FlatList
      style={styles.container}
      contentContainerStyle={items.length === 0 ? styles.empty : styles.list}
      data={items}
      keyExtractor={(item) => item.id}
      refreshControl={
        <RefreshControl
          refreshing={refreshing}
          onRefresh={onRefresh}
          tintColor={colors.primary}
        />
      }
      ListEmptyComponent={
        !isLoading ? (
          <View style={styles.emptyBox}>
            <Ionicons
              name="code-working-outline"
              size={48}
              color={colors.textDisabled}
            />
            <Text style={styles.emptyText}>{t("script_empty")}</Text>
          </View>
        ) : null
      }
      renderItem={renderItem}
    />
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
  cardTop: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
  },
  enabledChip: {
    width: 28,
    height: 28,
    borderRadius: 14,
    justifyContent: "center",
    alignItems: "center",
  },
  dot: { width: 8, height: 8, borderRadius: 4 },
  cardInfo: { flex: 1, gap: 4 },
  cardName: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
  },
  badgeRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
  },
  badge: {
    backgroundColor: colors.elevated,
    borderRadius: radius.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
  },
  badgeText: {
    fontSize: fontSize.xs,
    fontWeight: "600",
    color: colors.textSecondary,
  },
  runText: { fontSize: fontSize.xs, color: colors.textDisabled },
  errorText: {
    fontSize: fontSize.xs,
    color: colors.error,
    backgroundColor: colors.errorBg,
    borderRadius: radius.sm,
    padding: spacing.sm,
  },
});
