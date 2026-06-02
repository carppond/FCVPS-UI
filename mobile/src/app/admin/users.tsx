import { useState, useCallback, useMemo } from "react";
import {
  View,
  Text,
  FlatList,
  StyleSheet,
  RefreshControl,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useUsersQuery } from "../../api/admin";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";
import type { User } from "../../types/api";

export default function AdminUsersScreen() {
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const { data, isLoading, refetch } = useUsersQuery();
  const [refreshing, setRefreshing] = useState(false);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const renderItem = ({ item }: { item: User }) => (
    <View style={styles.card}>
      <View style={styles.cardLeft}>
        <View style={styles.iconBox}>
          <Ionicons name="person-outline" size={18} color={colors.primary} />
        </View>
      </View>
      <View style={styles.cardBody}>
        <View style={styles.nameRow}>
          <Text style={styles.username} numberOfLines={1}>
            {item.username}
          </Text>
          <View
            style={[
              styles.roleBadge,
              {
                backgroundColor:
                  item.role === "admin" ? colors.primarySoft : colors.infoBg,
              },
            ]}
          >
            <Text
              style={[
                styles.roleText,
                {
                  color: item.role === "admin" ? colors.primary : colors.info,
                },
              ]}
            >
              {item.role === "admin" ? "管理员" : "用户"}
            </Text>
          </View>
        </View>
        <Text style={styles.email} numberOfLines={1}>
          {item.email || "未设置邮箱"}
        </Text>
      </View>
      <View
        style={[
          styles.statusDot,
          {
            backgroundColor: item.is_active
              ? colors.success
              : colors.textDisabled,
          },
        ]}
      />
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
              name="people-outline"
              size={48}
              color={colors.textDisabled}
            />
            <Text style={styles.emptyText}>暂无用户</Text>
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
    flexDirection: "row",
    alignItems: "center",
  },
  cardLeft: { marginRight: spacing.md },
  iconBox: {
    width: 36,
    height: 36,
    borderRadius: radius.md,
    backgroundColor: colors.primarySoft,
    justifyContent: "center",
    alignItems: "center",
  },
  cardBody: { flex: 1, gap: 4 },
  nameRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
  },
  username: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
  },
  roleBadge: {
    borderRadius: radius.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
  },
  roleText: {
    fontSize: fontSize.xs,
    fontWeight: "700",
  },
  email: {
    fontSize: fontSize.sm,
    color: colors.textTertiary,
  },
  statusDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
    marginLeft: spacing.md,
  },
});
