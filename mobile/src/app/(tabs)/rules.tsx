import { View, Text, FlatList, StyleSheet, RefreshControl, TouchableOpacity, Alert, Modal } from "react-native";
import { useState, useCallback } from "react";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useRulesQuery, useDeleteRule, useUpdateRule } from "../../api/rule";
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
  const deleteMutation = useDeleteRule();
  const updateMutation = useUpdateRule();
  const [refreshing, setRefreshing] = useState(false);
  const [menuVisible, setMenuVisible] = useState(false);
  const [selectedRule, setSelectedRule] = useState<CustomRule | null>(null);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const openMenu = (rule: CustomRule) => {
    setSelectedRule(rule);
    setMenuVisible(true);
  };

  const closeMenu = () => {
    setMenuVisible(false);
    setSelectedRule(null);
  };

  const handleEdit = () => {
    if (!selectedRule) return;
    const rule = selectedRule;
    closeMenu();
    router.push(
      `/rule/create?editId=${rule.id}&editName=${encodeURIComponent(rule.name)}&editType=${rule.type}&editMode=${rule.mode}&editContent=${encodeURIComponent(rule.content)}&editEnabled=${rule.enabled}`,
    );
  };

  const handleToggleEnabled = () => {
    if (!selectedRule) return;
    const rule = selectedRule;
    closeMenu();
    updateMutation.mutate(
      { id: rule.id, data: { enabled: !rule.enabled } },
      {
        onSuccess: () => {
          refetch();
        },
        onError: (err: any) => Alert.alert("操作失败", err.message),
      },
    );
  };

  const handleDelete = () => {
    if (!selectedRule) return;
    const rule = selectedRule;
    closeMenu();
    Alert.alert("删除确认", `确定删除规则「${rule.name}」吗？`, [
      { text: "取消", style: "cancel" },
      {
        text: "删除",
        style: "destructive",
        onPress: () => {
          deleteMutation.mutate(rule.id, {
            onSuccess: () => refetch(),
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
        ListHeaderComponent={
          <TouchableOpacity
            style={styles.addBtn}
            onPress={() => router.push("/rule/create")}
            activeOpacity={0.7}
          >
            <Ionicons name="add-circle-outline" size={18} color={colors.primary} />
            <Text style={styles.addBtnText}>添加规则</Text>
          </TouchableOpacity>
        }
        ListEmptyComponent={
          !isLoading ? (
            <View style={styles.emptyBox}>
              <Ionicons name="shield-outline" size={48} color={colors.textDisabled} />
              <Text style={styles.emptyText}>暂无规则</Text>
              <TouchableOpacity
                style={styles.emptyCreateBtn}
                onPress={() => router.push("/rule/create")}
                activeOpacity={0.7}
              >
                <Ionicons name="add-outline" size={16} color="#fff" />
                <Text style={styles.emptyCreateText}>添加规则</Text>
              </TouchableOpacity>
            </View>
          ) : null
        }
        renderItem={({ item }) => <RuleCard rule={item} onMenu={openMenu} />}
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
            <Text style={styles.menuTitle} numberOfLines={1}>{selectedRule?.name}</Text>
            <TouchableOpacity style={styles.menuItem} onPress={handleEdit} activeOpacity={0.6}>
              <Ionicons name="create-outline" size={20} color={colors.primary} />
              <Text style={styles.menuItemText}>编辑</Text>
            </TouchableOpacity>
            <TouchableOpacity style={styles.menuItem} onPress={handleToggleEnabled} activeOpacity={0.6}>
              <Ionicons
                name={selectedRule?.enabled ? "pause-circle-outline" : "play-circle-outline"}
                size={20}
                color={selectedRule?.enabled ? colors.warning : colors.success}
              />
              <Text style={styles.menuItemText}>
                {selectedRule?.enabled ? "禁用" : "启用"}
              </Text>
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

function RuleCard({ rule, onMenu }: { rule: CustomRule; onMenu: (rule: CustomRule) => void }) {
  const tc = typeColor(rule.type);
  const mc = modeColor(rule.mode);

  return (
    <View style={[styles.card, !rule.enabled && styles.cardDisabled]}>
      <View style={styles.cardHeader}>
        <Text style={styles.cardName} numberOfLines={1}>{rule.name}</Text>
        <TouchableOpacity
          style={styles.menuBtn}
          onPress={() => onMenu(rule)}
          activeOpacity={0.6}
          hitSlop={{ top: 8, bottom: 8, left: 8, right: 8 }}
        >
          <Ionicons name="ellipsis-horizontal" size={18} color={colors.textTertiary} />
        </TouchableOpacity>
      </View>
      <View style={styles.badgesRow}>
        <View style={[styles.badge, { backgroundColor: tc + "1a" }]}>
          <Text style={[styles.badgeText, { color: tc }]}>{rule.type.toUpperCase()}</Text>
        </View>
        <View style={[styles.badge, { backgroundColor: mc + "1a" }]}>
          <Text style={[styles.badgeText, { color: mc }]}>{modeLabel(rule.mode)}</Text>
        </View>
        <View style={[styles.enabledDot, { backgroundColor: rule.enabled ? colors.success : colors.textDisabled }]} />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  wrapper: { flex: 1, backgroundColor: colors.bg },
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
  menuBtn: { padding: spacing.xs },
  enabledDot: { width: 8, height: 8, borderRadius: 4 },
  badgesRow: { flexDirection: "row", gap: spacing.sm, alignItems: "center" },
  badge: {
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: radius.sm,
  },
  badgeText: { fontSize: fontSize.xs, fontWeight: "700", letterSpacing: 0.5 },
  addBtn: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.xs,
    backgroundColor: colors.primarySoft,
    borderRadius: radius.lg,
    paddingVertical: spacing.md,
    marginBottom: spacing.md,
  },
  addBtnText: { fontSize: fontSize.sm, fontWeight: "600", color: colors.primary },
  emptyCreateBtn: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
    backgroundColor: colors.primary,
    borderRadius: radius.lg,
    paddingHorizontal: spacing.xl,
    paddingVertical: spacing.md,
    marginTop: spacing.md,
  },
  emptyCreateText: { fontSize: fontSize.base, fontWeight: "700", color: "#fff" },
  // Modal action sheet
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
