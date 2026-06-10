import { View, Text, FlatList, StyleSheet, RefreshControl, TouchableOpacity, Alert, Modal, TextInput, ScrollView } from "react-native";
import { useState, useCallback, useMemo } from "react";
import { router } from "expo-router";
import { useTranslation } from "react-i18next";
import { Ionicons } from "@expo/vector-icons";
import * as Clipboard from "expo-clipboard";
import { useVpsAssetsQuery, useVpsAssetSummaryQuery, useDeleteVpsAsset } from "../../api/vps-asset";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";
import type { VpsAsset, VpsAssetStatus } from "../../types/api";
import { formatApiError } from "../../lib/format-api-error";

const FLAG_MAP: Record<string, string> = {
  hk: "🇭🇰", "hong kong": "🇭🇰", jp: "🇯🇵", japan: "🇯🇵", tokyo: "🇯🇵",
  us: "🇺🇸", sg: "🇸🇬", singapore: "🇸🇬", de: "🇩🇪", germany: "🇩🇪",
  uk: "🇬🇧", kr: "🇰🇷", tw: "🇹🇼", nl: "🇳🇱", fr: "🇫🇷", ca: "🇨🇦", au: "🇦🇺",
};

function guessFlag(location?: string): string {
  if (!location) return "🌐";
  const lower = location.toLowerCase();
  for (const [k, v] of Object.entries(FLAG_MAP)) {
    if (lower.includes(k)) return v;
  }
  return "🌐";
}

function statusColor(status: VpsAssetStatus, c: AppColors) {
  switch (status) {
    case "normal": return c.success;
    case "expiring": return c.warning;
    case "expired": return c.error;
  }
}

function currencySymbol(c: string) {
  switch (c.toUpperCase()) {
    case "CNY": return "¥";
    case "USD": return "$";
    case "EUR": return "€";
    case "GBP": return "£";
    default: return c + " ";
  }
}

type StatusFilterKey = "all" | "normal" | "expiring" | "expired";

const STATUS_FILTERS: { key: StatusFilterKey; status: VpsAssetStatus | null }[] = [
  { key: "all", status: null },
  { key: "normal", status: "normal" },
  { key: "expiring", status: "expiring" },
  { key: "expired", status: "expired" },
];

const STATUS_FILTER_LABEL: Record<StatusFilterKey, string> = {
  all: "filter_all",
  normal: "filter_normal",
  expiring: "filter_expiring",
  expired: "filter_expired",
};

export default function VpsAssetsScreen() {
  const { t } = useTranslation(["vps", "common"]);
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const { data, isLoading, refetch } = useVpsAssetsQuery();
  const summary = useVpsAssetSummaryQuery();
  const deleteMutation = useDeleteVpsAsset();
  const [refreshing, setRefreshing] = useState(false);
  const [menuVisible, setMenuVisible] = useState(false);
  const [selectedVps, setSelectedVps] = useState<VpsAsset | null>(null);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilterKey>("all");

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await Promise.all([refetch(), summary.refetch()]);
    setRefreshing(false);
  }, []);

  const allItems = data?.items ?? [];

  const items = useMemo(() => {
    let filtered = allItems;
    const mappedStatus = STATUS_FILTERS.find((f) => f.key === statusFilter)?.status ?? null;
    if (mappedStatus) {
      filtered = filtered.filter((v) => v.status === mappedStatus);
    }
    const q = search.trim().toLowerCase();
    if (q) {
      filtered = filtered.filter((v) => {
        const name = v.name?.toLowerCase() ?? "";
        const provider = v.provider?.toLowerCase() ?? "";
        const ip = v.ip?.toLowerCase() ?? "";
        return name.includes(q) || provider.includes(q) || ip.includes(q);
      });
    }
    return filtered;
  }, [allItems, search, statusFilter]);

  const copyIp = async (ip: string) => {
    await Clipboard.setStringAsync(ip);
    Alert.alert(t("copied"), ip);
  };

  const openMenu = (vps: VpsAsset) => {
    setSelectedVps(vps);
    setMenuVisible(true);
  };

  const closeMenu = () => {
    setMenuVisible(false);
    setSelectedVps(null);
  };

  const handleEdit = () => {
    if (!selectedVps) return;
    closeMenu();
    router.push(`/vps-asset/edit?id=${selectedVps.id}`);
  };

  const handleSSH = () => {
    if (!selectedVps) return;
    closeMenu();
    router.push(`/vps-asset/ssh?id=${selectedVps.id}`);
  };

  const handleDelete = () => {
    if (!selectedVps) return;
    const vpsId = selectedVps.id;
    const vpsName = selectedVps.name;
    closeMenu();
    Alert.alert(t("delete_confirm_title"), t("delete_confirm_message", { name: vpsName }), [
      { text: t("common:cancel"), style: "cancel" },
      {
        text: t("delete"),
        style: "destructive",
        onPress: () => {
          deleteMutation.mutate(vpsId, {
            onSuccess: () => Alert.alert(t("delete_success"), t("delete_success_message")),
            onError: (err: any) => Alert.alert(t("delete_failed"), formatApiError(err, t)),
          });
        },
      },
    ]);
  };

  return (
    <View style={styles.wrapper}>
      <FlatList
        style={styles.container}
        contentContainerStyle={items.length === 0 && !search && statusFilter === "all" ? styles.empty : styles.list}
        data={items}
        keyExtractor={(item) => item.id}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor={colors.primary} />
        }
        ListHeaderComponent={
          <View>
            {summary.data ? (
              <View style={styles.summaryRow}>
                <SumChip label={t("summary_total")} value={String(summary.data.total)} />
                <SumChip label={t("summary_expiring")} value={String(summary.data.expiring)} color={summary.data.expiring > 0 ? colors.warning : undefined} />
                <SumChip label={t("summary_expired")} value={String(summary.data.expired)} color={summary.data.expired > 0 ? colors.error : undefined} />
              </View>
            ) : null}
            {allItems.length > 0 ? (
              <>
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
                  {STATUS_FILTERS.map((s) => (
                    <TouchableOpacity
                      key={s.key}
                      style={[styles.filterChip, statusFilter === s.key && styles.filterChipActive]}
                      onPress={() => setStatusFilter(s.key)}
                      activeOpacity={0.7}
                    >
                      <Text style={[styles.filterChipText, statusFilter === s.key && styles.filterChipTextActive]}>{t(STATUS_FILTER_LABEL[s.key])}</Text>
                    </TouchableOpacity>
                  ))}
                </ScrollView>
              </>
            ) : null}
          </View>
        }
        ListEmptyComponent={
          !isLoading ? (
            <View style={styles.emptyBox}>
              <Ionicons name="hardware-chip-outline" size={48} color={colors.textDisabled} />
              <Text style={styles.emptyText}>{t("empty")}</Text>
            </View>
          ) : null
        }
        renderItem={({ item }) => <VpsCard vps={item} onCopyIp={copyIp} onLongPress={openMenu} />}
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
            <Text style={styles.menuTitle} numberOfLines={1}>{selectedVps?.name}</Text>
            <TouchableOpacity style={styles.menuItem} onPress={handleSSH} activeOpacity={0.6}>
              <Ionicons name="terminal-outline" size={20} color={colors.success} />
              <Text style={styles.menuItemText}>{t("ssh_connect")}</Text>
            </TouchableOpacity>
            <TouchableOpacity style={styles.menuItem} onPress={handleEdit} activeOpacity={0.6}>
              <Ionicons name="create-outline" size={20} color={colors.primary} />
              <Text style={styles.menuItemText}>{t("edit")}</Text>
            </TouchableOpacity>
            <TouchableOpacity style={styles.menuItem} onPress={handleDelete} activeOpacity={0.6}>
              <Ionicons name="trash-outline" size={20} color={colors.error} />
              <Text style={[styles.menuItemText, { color: colors.error }]}>{t("delete")}</Text>
            </TouchableOpacity>
            <TouchableOpacity style={styles.menuCancel} onPress={closeMenu} activeOpacity={0.6}>
              <Text style={styles.menuCancelText}>{t("common:cancel")}</Text>
            </TouchableOpacity>
          </View>
        </TouchableOpacity>
      </Modal>
    </View>
  );
}

function SumChip({ label, value, color }: { label: string; value: string; color?: string }) {
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  return (
    <View style={styles.sumChip}>
      <Text style={styles.sumLabel}>{label}</Text>
      <Text style={[styles.sumValue, color ? { color } : null]}>{value}</Text>
    </View>
  );
}

function VpsCard({ vps, onCopyIp, onLongPress }: { vps: VpsAsset; onCopyIp: (ip: string) => void; onLongPress: (vps: VpsAsset) => void }) {
  const { t } = useTranslation("vps");
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const sc = statusColor(vps.status, colors);
  const flag = guessFlag(vps.location);
  const sym = currencySymbol(vps.currency);
  const spec = [vps.cpu, vps.memory, vps.disk].filter(Boolean).join(" · ");

  return (
    <TouchableOpacity
      style={[styles.card, vps.status !== "normal" && { borderColor: sc + "33" }]}
      activeOpacity={0.7}
      onLongPress={() => onLongPress(vps)}
    >
      <View style={styles.cardTop}>
        <Text style={styles.flag}>{flag}</Text>
        <View style={styles.cardInfo}>
          <Text style={styles.cardName} numberOfLines={1}>{vps.name}</Text>
          <Text style={styles.cardProvider}>{vps.provider} · {sym}{vps.price}</Text>
          {spec ? <Text style={styles.cardSpec}>{spec}</Text> : null}
        </View>
        <View style={styles.dayChip}>
          <Text style={[styles.dayNum, { color: sc }]}>{vps.days_until_expiry}</Text>
          <Text style={[styles.dayLabel, { color: sc }]}>
            {vps.days_until_expiry <= 0 ? t("expired") : t("unit_day")}
          </Text>
        </View>
      </View>
      {vps.ip && (
        <TouchableOpacity style={styles.ipRow} onPress={() => onCopyIp(vps.ip!)} activeOpacity={0.6}>
          <Ionicons name="copy-outline" size={10} color={colors.textTertiary} />
          <Text style={styles.ipText}>{vps.ip}</Text>
        </TouchableOpacity>
      )}
    </TouchableOpacity>
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
  wrapper: { flex: 1, backgroundColor: colors.bg },
  container: { flex: 1, backgroundColor: colors.bg },
  list: { padding: spacing.lg },
  empty: { flex: 1, justifyContent: "center", alignItems: "center" },
  emptyBox: { alignItems: "center", gap: spacing.md },
  emptyText: { fontSize: fontSize.base, color: colors.textTertiary },
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
  summaryRow: { flexDirection: "row", gap: spacing.sm, marginBottom: spacing.lg },
  sumChip: { flex: 1, backgroundColor: colors.surface, borderRadius: radius.lg, borderWidth: 1, borderColor: colors.border, padding: spacing.md, alignItems: "center" },
  sumLabel: { fontSize: fontSize.xs, color: colors.textTertiary, fontWeight: "600", textTransform: "uppercase" },
  sumValue: { fontSize: 20, fontWeight: "800", color: colors.textPrimary, marginTop: 2 },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.md,
  },
  cardTop: { flexDirection: "row", alignItems: "center", gap: spacing.md },
  flag: { fontSize: 28 },
  cardInfo: { flex: 1, gap: 1 },
  cardName: { fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
  cardProvider: { fontSize: fontSize.xs, color: colors.textTertiary },
  cardSpec: { fontSize: fontSize.xs, color: colors.textSecondary, fontFamily: "monospace" },
  dayChip: { alignItems: "center", minWidth: 50 },
  dayNum: { fontSize: 28, fontWeight: "800" },
  dayLabel: { fontSize: 8, fontWeight: "700", letterSpacing: 0.5, textTransform: "uppercase" },
  ipRow: { flexDirection: "row", alignItems: "center", gap: 4, marginTop: spacing.sm, backgroundColor: colors.surfaceHover, borderRadius: radius.sm, paddingHorizontal: spacing.sm, paddingVertical: 4, alignSelf: "flex-start" },
  ipText: { fontSize: fontSize.xs, color: colors.textSecondary, fontFamily: "monospace" },
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
