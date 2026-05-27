import { useState } from "react";
import { View, Text, ScrollView, StyleSheet, TouchableOpacity, Share, Alert, ActivityIndicator } from "react-native";
import { useLocalSearchParams } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import * as Clipboard from "expo-clipboard";
import { useQuery, useMutation } from "@tanstack/react-query";
import { apiFetch } from "../../lib/api-client";
import { useAuthStore } from "../../stores/auth-store";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { SubscriptionDetail, SyncResult, ShortLink } from "../../types/api";

const TARGETS: { key: string; label: string; icon: keyof typeof Ionicons.glyphMap; color: string }[] = [
  { key: "clash", label: "Clash", icon: "flash-outline", color: colors.primary },
  { key: "clashmeta", label: "Clash Meta", icon: "flash-outline", color: colors.warning },
  { key: "clash-verge-rev", label: "Clash Verge Rev", icon: "desktop-outline", color: colors.info },
  { key: "stash", label: "Stash", icon: "phone-portrait-outline", color: "#c084fc" },
  { key: "singbox", label: "sing-box", icon: "cube-outline", color: colors.info },
  { key: "shadowrocket", label: "Shadowrocket", icon: "rocket-outline", color: colors.success },
  { key: "surge", label: "Surge", icon: "thunderstorm-outline", color: colors.warning },
  { key: "surge-ios", label: "Surge iOS", icon: "phone-portrait-outline", color: colors.warning },
  { key: "quantumult-x", label: "Quantumult X", icon: "grid-outline", color: "#c084fc" },
  { key: "loon", label: "Loon", icon: "globe-outline", color: colors.info },
  { key: "v2ray", label: "V2Ray", icon: "link-outline", color: colors.success },
];

function formatBytes(bytes: number): string {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const val = bytes / Math.pow(1024, i);
  return `${val.toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

function daysUntil(epochSeconds: number): number {
  const nowMs = Date.now();
  const targetMs = epochSeconds > 1e12 ? epochSeconds : epochSeconds * 1000;
  return Math.max(0, Math.ceil((targetMs - nowMs) / 86_400_000));
}

function formatDate(epoch: number): string {
  const ms = epoch > 1e12 ? epoch : epoch * 1000;
  const d = new Date(ms);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

export default function SubscriptionDetailScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const serverUrl = useAuthStore((s) => s.serverUrl);

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["subscription", "detail", id],
    queryFn: () => apiFetch<SubscriptionDetail>(`/api/subscriptions/${id}`),
    enabled: !!id,
  });

  const syncMutation = useMutation({
    mutationFn: () =>
      apiFetch<SyncResult>(`/api/subscriptions/${id}/sync`, { method: "POST" }),
    onSuccess: (res) => {
      Alert.alert("同步成功", `新增 ${res.added_count} 个节点，移除 ${res.removed_count} 个`);
      refetch();
    },
    onError: (err: any) => Alert.alert("同步失败", err.message),
  });

  // Download URL uses /download/{name}?token={share_token}&target=... (matches web)
  const getDownloadUrl = (target: string) => {
    if (!data) return "";
    const name = encodeURIComponent(data.name);
    const token = data.share_token ? encodeURIComponent(data.share_token) : "";
    return `${serverUrl}/download/${name}?token=${token}&target=${encodeURIComponent(target)}`;
  };

  const shortLinkMutation = useMutation({
    mutationFn: (targetUrl: string) =>
      apiFetch<ShortLink>("/api/shortlinks", {
        method: "POST",
        body: JSON.stringify({ target_url: targetUrl }),
      }),
  });

  const copyLink = async (target: string, label: string) => {
    const url = getDownloadUrl(target);
    await Clipboard.setStringAsync(url);
    Alert.alert("已复制", `${label} 订阅链接已复制到剪贴板`);
  };

  const shareLink = async (target: string, label: string) => {
    const url = getDownloadUrl(target);
    await Share.share({ message: url, title: `${data?.name} - ${label}` });
  };

  const createShortLink = async (target: string, label: string) => {
    const url = getDownloadUrl(target);
    try {
      const result = await shortLinkMutation.mutateAsync(url);
      await Clipboard.setStringAsync(result.short_url);
      Alert.alert("短链已生成", `${result.short_url}\n\n已复制到剪贴板`);
    } catch (err: any) {
      Alert.alert("生成失败", err.message);
    }
  };

  if (isLoading || !data) {
    return (
      <View style={styles.loading}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }

  const syncStatus = data.last_sync_status;
  const statusColor = syncStatus === "ok" ? colors.success : syncStatus === "error" ? colors.error : colors.textDisabled;

  const hasTraffic = data.traffic_total && data.traffic_total > 0;
  const trafficPct = hasTraffic
    ? Math.min(100, ((data.traffic_used ?? 0) / data.traffic_total!) * 100)
    : 0;

  const progressBarColor =
    trafficPct >= 90 ? colors.error : trafficPct >= 70 ? colors.warning : colors.info;

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {/* Header card */}
      <View style={styles.headerCard}>
        <View style={styles.headerTop}>
          <View style={[styles.statusDot, { backgroundColor: statusColor }]} />
          <Text style={styles.name}>{data.name}</Text>
        </View>
        <View style={styles.metaRow}>
          <MetaChip icon="layers-outline" text={`${data.node_count} 个节点`} />
          <MetaChip icon="sync-outline" text={data.type.toUpperCase()} />
        </View>
        {data.source_url && (
          <Text style={styles.sourceUrl} numberOfLines={2}>{data.source_url}</Text>
        )}
        <TouchableOpacity
          style={[styles.syncBtn, syncMutation.isPending && styles.syncBtnDisabled]}
          onPress={() => syncMutation.mutate()}
          disabled={syncMutation.isPending}
          activeOpacity={0.7}
        >
          <Ionicons
            name="sync-outline"
            size={16}
            color="#fff"
            style={syncMutation.isPending ? { opacity: 0.5 } : undefined}
          />
          <Text style={styles.syncBtnText}>
            {syncMutation.isPending ? "同步中..." : "立即同步"}
          </Text>
        </TouchableOpacity>
      </View>

      {/* Traffic info section */}
      <View style={styles.trafficCard}>
        <View style={styles.trafficRow}>
          <Ionicons name="cloud-outline" size={14} color={colors.textTertiary} />
          <Text style={styles.trafficLabel}>流量</Text>
          <Text style={styles.trafficValue}>
            {formatBytes(data.traffic_used ?? 0)} / {hasTraffic ? formatBytes(data.traffic_total!) : "无限制"}
          </Text>
        </View>
        {hasTraffic && (
          <View style={styles.progressBarBg}>
            <View
              style={[
                styles.progressBarFill,
                { width: `${trafficPct}%`, backgroundColor: progressBarColor },
              ]}
            />
          </View>
        )}
        {hasTraffic && (
          <Text style={styles.trafficPctText}>
            已使用 {trafficPct.toFixed(1)}%
          </Text>
        )}
        <View style={styles.trafficRow}>
          <Ionicons name="calendar-outline" size={14} color={colors.textTertiary} />
          <Text style={styles.trafficLabel}>到期</Text>
          <Text style={styles.trafficValue}>
            {data.expire_at
              ? `${formatDate(data.expire_at)} (剩余 ${daysUntil(data.expire_at)} 天)`
              : "无期限"}
          </Text>
        </View>
      </View>

      {/* Download links */}
      <Text style={styles.sectionTitle}>订阅链接</Text>
      <View style={styles.linksGrid}>
        {TARGETS.map((t) => (
          <View key={t.key} style={styles.linkCard}>
            <View style={styles.linkHeader}>
              <View style={[styles.linkIcon, { backgroundColor: t.color + "18" }]}>
                <Ionicons name={t.icon} size={18} color={t.color} />
              </View>
              <Text style={styles.linkLabel} numberOfLines={1}>{t.label}</Text>
            </View>
            <View style={styles.linkActions}>
              <TouchableOpacity
                style={styles.linkBtn}
                onPress={() => copyLink(t.key, t.label)}
                activeOpacity={0.6}
              >
                <Ionicons name="copy-outline" size={12} color={colors.textSecondary} />
                <Text style={styles.linkBtnText}>复制</Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={styles.linkBtn}
                onPress={() => createShortLink(t.key, t.label)}
                activeOpacity={0.6}
              >
                <Ionicons name="link-outline" size={12} color={colors.primary} />
                <Text style={[styles.linkBtnText, { color: colors.primary }]}>短链</Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={styles.linkBtn}
                onPress={() => shareLink(t.key, t.label)}
                activeOpacity={0.6}
              >
                <Ionicons name="share-outline" size={14} color={colors.textSecondary} />
                <Text style={styles.linkBtnText}>分享</Text>
              </TouchableOpacity>
            </View>
          </View>
        ))}
      </View>

      {/* Nodes */}
      <Text style={styles.sectionTitle}>节点列表 ({data.nodes?.length ?? 0}) · 总计 {data.nodes_total ?? 0}</Text>
      {(!data.nodes || data.nodes.length === 0) && (
        <View style={{ padding: 20, alignItems: "center" }}>
          <Text style={{ color: colors.textTertiary, fontSize: 13 }}>暂无节点，请先同步订阅</Text>
        </View>
      )}
      {(data.nodes ?? []).map((node) => (
        <View key={node.id} style={styles.nodeCard}>
          <View style={styles.nodeTop}>
            <View style={[styles.protoBadge, { backgroundColor: colors.primarySoft }]}>
              <Text style={styles.protoText}>{node.protocol}</Text>
            </View>
            <Text style={styles.nodeTag} numberOfLines={1}>{node.tag}</Text>
          </View>
          <Text style={styles.nodeServer}>{node.server}:{node.port}</Text>
        </View>
      ))}
    </ScrollView>
  );
}

function MetaChip({ icon, text }: { icon: keyof typeof Ionicons.glyphMap; text: string }) {
  return (
    <View style={styles.metaChip}>
      <Ionicons name={icon} size={12} color={colors.textTertiary} />
      <Text style={styles.metaChipText}>{text}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.xl, paddingBottom: 40 },
  loading: { flex: 1, justifyContent: "center", alignItems: "center", backgroundColor: colors.bg },
  headerCard: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.xl,
    gap: spacing.md,
  },
  headerTop: { flexDirection: "row", alignItems: "center", gap: spacing.sm },
  statusDot: { width: 10, height: 10, borderRadius: 5 },
  name: { fontSize: fontSize.xl, fontWeight: "800", color: colors.textPrimary, flex: 1 },
  metaRow: { flexDirection: "row", gap: spacing.sm },
  metaChip: { flexDirection: "row", alignItems: "center", gap: 4, backgroundColor: colors.surfaceHover, borderRadius: radius.sm, paddingHorizontal: spacing.sm, paddingVertical: 3 },
  metaChipText: { fontSize: fontSize.xs, color: colors.textTertiary, fontWeight: "600" },
  sourceUrl: { fontSize: fontSize.xs, color: colors.textDisabled, fontFamily: "monospace" },
  syncBtn: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.sm,
    backgroundColor: colors.primary,
    borderRadius: radius.lg,
    paddingVertical: spacing.md,
    marginTop: spacing.xs,
  },
  syncBtnDisabled: { opacity: 0.5 },
  syncBtnText: { fontSize: fontSize.base, fontWeight: "700", color: "#fff" },
  // Traffic info
  trafficCard: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.xl,
    marginTop: spacing.md,
    gap: spacing.sm,
  },
  trafficRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
  },
  trafficLabel: {
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.textTertiary,
  },
  trafficValue: {
    flex: 1,
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.textPrimary,
    textAlign: "right",
  },
  progressBarBg: {
    height: 4,
    backgroundColor: colors.border,
    borderRadius: 2,
    overflow: "hidden",
    marginVertical: spacing.xs,
  },
  progressBarFill: {
    height: 4,
    borderRadius: 2,
  },
  trafficPctText: {
    fontSize: fontSize.xs,
    color: colors.textTertiary,
    textAlign: "right",
  },
  // Section
  sectionTitle: {
    fontSize: fontSize.sm,
    fontWeight: "700",
    color: colors.textTertiary,
    textTransform: "uppercase",
    letterSpacing: 0.5,
    marginTop: spacing.xxl,
    marginBottom: spacing.md,
  },
  linksGrid: { flexDirection: "row", flexWrap: "wrap", gap: spacing.md },
  linkCard: {
    width: "48%",
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    gap: spacing.md,
  },
  linkHeader: { flexDirection: "row", alignItems: "center", gap: spacing.sm },
  linkIcon: { width: 32, height: 32, borderRadius: radius.md, justifyContent: "center", alignItems: "center" },
  linkLabel: { fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary, flex: 1 },
  linkActions: { flexDirection: "row", gap: spacing.sm },
  linkBtn: {
    flex: 1,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: 4,
    backgroundColor: colors.surfaceHover,
    borderRadius: radius.md,
    paddingVertical: spacing.sm,
  },
  linkBtnText: { fontSize: fontSize.xs, fontWeight: "600", color: colors.textSecondary },
  nodeCard: {
    backgroundColor: colors.surface,
    borderRadius: radius.lg,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.sm,
    gap: spacing.xs,
  },
  nodeTop: { flexDirection: "row", alignItems: "center", gap: spacing.sm },
  protoBadge: { paddingHorizontal: spacing.sm, paddingVertical: 2, borderRadius: radius.sm },
  protoText: { fontSize: fontSize.xs, fontWeight: "700", color: colors.primary, textTransform: "uppercase" },
  nodeTag: { flex: 1, fontSize: fontSize.base, fontWeight: "600", color: colors.textPrimary },
  nodeServer: { fontSize: fontSize.xs, color: colors.textTertiary, fontFamily: "monospace" },
});
