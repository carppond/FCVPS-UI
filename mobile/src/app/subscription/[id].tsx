import { useState } from "react";
import { View, Text, ScrollView, StyleSheet, TouchableOpacity, Share, Alert, ActivityIndicator } from "react-native";
import { useLocalSearchParams } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import * as Clipboard from "expo-clipboard";
import { useQuery, useMutation } from "@tanstack/react-query";
import { apiFetch } from "../../lib/api-client";
import { useAuthStore } from "../../stores/auth-store";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { SubscriptionDetail, SyncResult } from "../../types/api";

const TARGETS = [
  { key: "clash", label: "Clash", icon: "flash-outline" as const, color: colors.primary },
  { key: "singbox", label: "sing-box", icon: "cube-outline" as const, color: colors.info },
  { key: "surge", label: "Surge", icon: "thunderstorm-outline" as const, color: colors.warning },
  { key: "v2ray", label: "V2Ray", icon: "link-outline" as const, color: colors.success },
];

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

  const getDownloadUrl = (target: string) =>
    `${serverUrl}/download/${id}?target=${target}`;

  const copyLink = async (target: string, label: string) => {
    const url = getDownloadUrl(target);
    await Clipboard.setStringAsync(url);
    Alert.alert("已复制", `${label} 订阅链接已复制到剪贴板`);
  };

  const shareLink = async (target: string, label: string) => {
    const url = getDownloadUrl(target);
    await Share.share({ message: url, title: `${data?.name} - ${label}` });
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

      {/* Download links */}
      <Text style={styles.sectionTitle}>订阅链接</Text>
      <View style={styles.linksGrid}>
        {TARGETS.map((t) => (
          <View key={t.key} style={styles.linkCard}>
            <View style={styles.linkHeader}>
              <View style={[styles.linkIcon, { backgroundColor: t.color + "18" }]}>
                <Ionicons name={t.icon} size={18} color={t.color} />
              </View>
              <Text style={styles.linkLabel}>{t.label}</Text>
            </View>
            <View style={styles.linkActions}>
              <TouchableOpacity
                style={styles.linkBtn}
                onPress={() => copyLink(t.key, t.label)}
                activeOpacity={0.6}
              >
                <Ionicons name="copy-outline" size={14} color={colors.textSecondary} />
                <Text style={styles.linkBtnText}>复制</Text>
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
      <Text style={styles.sectionTitle}>节点列表 ({data.nodes?.length ?? 0})</Text>
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
  linkLabel: { fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
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
