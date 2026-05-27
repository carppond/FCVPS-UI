import { View, Text, FlatList, StyleSheet, TouchableOpacity, RefreshControl, Platform, Alert, Modal } from "react-native";
import { useState, useCallback } from "react";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useSubscriptionsQuery, useSyncSubscription, useDeleteSubscription } from "../../api/subscription";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { Subscription } from "../../types/api";

export default function SubscriptionsScreen() {
  const { data, isLoading, refetch } = useSubscriptionsQuery();
  const syncMutation = useSyncSubscription();
  const deleteMutation = useDeleteSubscription();
  const [refreshing, setRefreshing] = useState(false);
  const [menuVisible, setMenuVisible] = useState(false);
  const [selectedSub, setSelectedSub] = useState<Subscription | null>(null);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const openDetail = (sub: Subscription) => {
    router.push(`/subscription/${sub.id}`);
  };

  const openMenu = (sub: Subscription) => {
    setSelectedSub(sub);
    setMenuVisible(true);
  };

  const closeMenu = () => {
    setMenuVisible(false);
    setSelectedSub(null);
  };

  const handleSync = () => {
    if (!selectedSub) return;
    closeMenu();
    syncMutation.mutate(selectedSub.id, {
      onSuccess: (result) => {
        Alert.alert("同步成功", `节点数: ${result.node_count}\n新增: ${result.added_count}\n移除: ${result.removed_count}`);
      },
      onError: (err: any) => Alert.alert("同步失败", err.message),
    });
  };

  const handleEdit = () => {
    if (!selectedSub) return;
    closeMenu();
    router.push(`/subscription/edit?id=${selectedSub.id}`);
  };

  const handleDelete = () => {
    if (!selectedSub) return;
    const subId = selectedSub.id;
    closeMenu();
    Alert.alert("确认删除", `确定要删除订阅「${selectedSub.name}」吗？`, [
      { text: "取消", style: "cancel" },
      {
        text: "删除",
        style: "destructive",
        onPress: () => {
          deleteMutation.mutate(subId, {
            onSuccess: () => Alert.alert("已删除", "订阅已删除"),
            onError: (err: any) => Alert.alert("删除失败", err.message),
          });
        },
      },
    ]);
  };

  return (
    <View style={styles.wrapper}>
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
                <Text style={styles.badgeText}>{item.node_count} 个节点</Text>
              </View>
              <TouchableOpacity
                style={styles.menuBtn}
                onPress={() => openMenu(item)}
                activeOpacity={0.6}
                hitSlop={{ top: 8, bottom: 8, left: 8, right: 8 }}
              >
                <Ionicons name="ellipsis-horizontal" size={18} color={colors.textTertiary} />
              </TouchableOpacity>
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

      {/* Action menu modal */}
      <Modal
        visible={menuVisible}
        animationType="fade"
        transparent
        onRequestClose={closeMenu}
      >
        <TouchableOpacity style={styles.modalOverlay} activeOpacity={1} onPress={closeMenu}>
          <View style={styles.menuSheet}>
            <Text style={styles.menuTitle} numberOfLines={1}>{selectedSub?.name}</Text>
            <TouchableOpacity style={styles.menuItem} onPress={handleSync} activeOpacity={0.6}>
              <Ionicons name="sync-outline" size={20} color={colors.info} />
              <Text style={styles.menuItemText}>同步</Text>
            </TouchableOpacity>
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
    </View>
  );
}

const styles = StyleSheet.create({
  wrapper: { flex: 1, backgroundColor: colors.bg },
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
  menuBtn: { padding: spacing.xs },
  cardUrl: { fontSize: fontSize.xs, color: colors.textDisabled, fontFamily: Platform?.OS === "ios" ? "Menlo" : "monospace" },
  cardBottom: { flexDirection: "row", justifyContent: "space-between", alignItems: "center" },
  cardMeta: { fontSize: fontSize.xs, color: colors.textTertiary, textTransform: "uppercase", fontWeight: "600" },
  cardActions: { flexDirection: "row", gap: spacing.md },
  modalOverlay: {
    flex: 1,
    justifyContent: "flex-end",
    backgroundColor: "rgba(0,0,0,0.5)",
  },
  menuSheet: {
    backgroundColor: colors.surface,
    borderTopLeftRadius: radius.xl,
    borderTopRightRadius: radius.xl,
    padding: spacing.xl,
    paddingBottom: 40,
  },
  menuTitle: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
    marginBottom: spacing.lg,
    textAlign: "center",
  },
  menuItem: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
    paddingVertical: spacing.lg,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  menuItemText: {
    fontSize: fontSize.base,
    fontWeight: "600",
    color: colors.textPrimary,
  },
  menuCancel: {
    alignItems: "center",
    paddingVertical: spacing.lg,
    marginTop: spacing.sm,
  },
  menuCancelText: {
    fontSize: fontSize.base,
    fontWeight: "600",
    color: colors.textTertiary,
  },
});
