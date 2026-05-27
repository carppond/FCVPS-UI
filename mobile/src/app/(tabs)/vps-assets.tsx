import { View, Text, FlatList, StyleSheet, RefreshControl, TouchableOpacity, Alert } from "react-native";
import { useState, useCallback } from "react";
import { Ionicons } from "@expo/vector-icons";
import * as Clipboard from "expo-clipboard";
import { useVpsAssetsQuery, useVpsAssetSummaryQuery } from "../../api/vps-asset";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { VpsAsset, VpsAssetStatus } from "../../types/api";

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

function statusColor(status: VpsAssetStatus) {
  switch (status) {
    case "normal": return colors.success;
    case "expiring": return colors.warning;
    case "expired": return colors.error;
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

export default function VpsAssetsScreen() {
  const { data, isLoading, refetch } = useVpsAssetsQuery();
  const summary = useVpsAssetSummaryQuery();
  const [refreshing, setRefreshing] = useState(false);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await Promise.all([refetch(), summary.refetch()]);
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const copyIp = async (ip: string) => {
    await Clipboard.setStringAsync(ip);
    Alert.alert("已复制", ip);
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
        summary.data ? (
          <View style={styles.summaryRow}>
            <SumChip label="总数" value={String(summary.data.total)} />
            <SumChip label="即将到期" value={String(summary.data.expiring)} color={summary.data.expiring > 0 ? colors.warning : undefined} />
            <SumChip label="已到期" value={String(summary.data.expired)} color={summary.data.expired > 0 ? colors.error : undefined} />
          </View>
        ) : null
      }
      ListEmptyComponent={
        !isLoading ? (
          <View style={styles.emptyBox}>
            <Ionicons name="hardware-chip-outline" size={48} color={colors.textDisabled} />
            <Text style={styles.emptyText}>暂无 VPS 资产</Text>
          </View>
        ) : null
      }
      renderItem={({ item }) => <VpsCard vps={item} onCopyIp={copyIp} />}
    />
  );
}

function SumChip({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <View style={styles.sumChip}>
      <Text style={styles.sumLabel}>{label}</Text>
      <Text style={[styles.sumValue, color ? { color } : null]}>{value}</Text>
    </View>
  );
}

function VpsCard({ vps, onCopyIp }: { vps: VpsAsset; onCopyIp: (ip: string) => void }) {
  const sc = statusColor(vps.status);
  const flag = guessFlag(vps.location);
  const sym = currencySymbol(vps.currency);
  const spec = [vps.cpu, vps.memory, vps.disk].filter(Boolean).join(" · ");

  return (
    <View style={[styles.card, vps.status !== "normal" && { borderColor: sc + "33" }]}>
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
            {vps.days_until_expiry <= 0 ? "EXPIRED" : "DAYS"}
          </Text>
        </View>
      </View>
      {vps.ip && (
        <TouchableOpacity style={styles.ipRow} onPress={() => onCopyIp(vps.ip!)} activeOpacity={0.6}>
          <Ionicons name="copy-outline" size={10} color={colors.textTertiary} />
          <Text style={styles.ipText}>{vps.ip}</Text>
        </TouchableOpacity>
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
});
