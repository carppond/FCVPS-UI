import { useState, useCallback } from "react";
import {
  View,
  Text,
  ScrollView,
  StyleSheet,
  TouchableOpacity,
  RefreshControl,
  Linking,
  Alert,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useOtaStatus, useOtaHistory } from "../../api/admin";
import { apiFetch } from "../../lib/api-client";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { OTAHistoryItem } from "../../types/api";

function formatTime(ts: number): string {
  return new Date(ts * 1000).toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function AdminOtaScreen() {
  const queryClient = useQueryClient();
  const { data: status, isLoading: statusLoading, refetch: refetchStatus } = useOtaStatus();
  const { data: history, isLoading: historyLoading, refetch: refetchHistory } = useOtaHistory();
  const [refreshing, setRefreshing] = useState(false);

  const checkMutation = useMutation({
    mutationFn: () =>
      apiFetch<void>("/api/admin/ota/check"),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "ota"] });
      queryClient.invalidateQueries({ queryKey: ["admin", "ota-history"] });
      Alert.alert("检查完成", "已获取最新版本信息");
    },
    onError: (err: any) => Alert.alert("检查失败", err.message),
  });

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await Promise.all([refetchStatus(), refetchHistory()]);
    setRefreshing(false);
  }, []);

  const historyItems = history ?? [];

  const renderHistoryItem = (item: OTAHistoryItem, index: number) => (
    <View key={`${item.version}-${index}`} style={styles.historyCard}>
      <View style={styles.historyHeader}>
        <Text style={styles.historyVersion}>{item.version}</Text>
        <View
          style={[
            styles.statusBadge,
            {
              backgroundColor:
                item.status === "success" ? colors.successBg : colors.errorBg,
            },
          ]}
        >
          <Text
            style={[
              styles.statusText,
              {
                color:
                  item.status === "success" ? colors.success : colors.error,
              },
            ]}
          >
            {item.status === "success" ? "成功" : "失败"}
          </Text>
        </View>
      </View>
      <Text style={styles.historyTime}>{formatTime(item.applied_at)}</Text>
      {item.error ? (
        <Text style={styles.historyError} numberOfLines={2}>
          {item.error}
        </Text>
      ) : null}
    </View>
  );

  return (
    <ScrollView
      style={styles.container}
      contentContainerStyle={styles.content}
      refreshControl={
        <RefreshControl
          refreshing={refreshing}
          onRefresh={onRefresh}
          tintColor={colors.primary}
        />
      }
    >
      {/* Version info */}
      <View style={styles.card}>
        <View style={styles.cardHeader}>
          <View style={[styles.cardIcon, { backgroundColor: colors.infoBg }]}>
            <Ionicons
              name="cloud-download-outline"
              size={16}
              color={colors.info}
            />
          </View>
          <Text style={styles.cardTitle}>版本信息</Text>
          {status?.has_update ? (
            <View style={styles.updateBadge}>
              <Text style={styles.updateText}>有更新</Text>
            </View>
          ) : null}
        </View>

        <View style={styles.infoRow}>
          <Text style={styles.infoLabel}>当前版本</Text>
          <Text style={styles.infoValue}>
            {status?.current_version ?? "--"}
          </Text>
        </View>
        <View style={styles.infoRow}>
          <Text style={styles.infoLabel}>最新版本</Text>
          <Text style={styles.infoValue}>
            {status?.latest_version ?? "--"}
          </Text>
        </View>
        {status?.release_url ? (
          <TouchableOpacity
            style={styles.linkRow}
            onPress={() => Linking.openURL(status.release_url)}
            activeOpacity={0.6}
          >
            <Ionicons name="open-outline" size={14} color={colors.primary} />
            <Text style={styles.linkText}>查看发布页</Text>
          </TouchableOpacity>
        ) : null}
      </View>

      {/* Check update button */}
      <TouchableOpacity
        style={[
          styles.checkBtn,
          checkMutation.isPending && styles.checkBtnDisabled,
        ]}
        onPress={() => checkMutation.mutate()}
        disabled={checkMutation.isPending}
        activeOpacity={0.8}
      >
        <Ionicons name="refresh-outline" size={18} color="#fff" />
        <Text style={styles.checkBtnText}>
          {checkMutation.isPending ? "检查中..." : "检查更新"}
        </Text>
      </TouchableOpacity>

      {/* History */}
      {historyItems.length > 0 ? (
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>更新历史</Text>
          {historyItems.map(renderHistoryItem)}
        </View>
      ) : !historyLoading ? (
        <View style={styles.emptyBox}>
          <Ionicons
            name="time-outline"
            size={36}
            color={colors.textDisabled}
          />
          <Text style={styles.emptyText}>暂无更新历史</Text>
        </View>
      ) : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.xl, paddingBottom: 40 },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.xl,
    marginBottom: spacing.lg,
  },
  cardHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
    marginBottom: spacing.lg,
  },
  cardIcon: {
    width: 28,
    height: 28,
    borderRadius: radius.md,
    justifyContent: "center",
    alignItems: "center",
  },
  cardTitle: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
    flex: 1,
  },
  updateBadge: {
    backgroundColor: colors.warningBg,
    borderRadius: radius.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
  },
  updateText: {
    fontSize: fontSize.xs,
    fontWeight: "700",
    color: colors.warning,
  },
  infoRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    paddingVertical: spacing.sm,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  infoLabel: {
    fontSize: fontSize.sm,
    color: colors.textTertiary,
  },
  infoValue: {
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.textSecondary,
    fontFamily: "monospace",
  },
  linkRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
    marginTop: spacing.md,
  },
  linkText: {
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.primary,
  },
  checkBtn: {
    flexDirection: "row",
    height: 50,
    borderRadius: radius.lg,
    backgroundColor: colors.primary,
    justifyContent: "center",
    alignItems: "center",
    gap: spacing.sm,
    marginBottom: spacing.xl,
  },
  checkBtnDisabled: { opacity: 0.5 },
  checkBtnText: { fontSize: fontSize.lg, fontWeight: "700", color: "#fff" },
  section: { marginBottom: spacing.lg },
  sectionTitle: {
    fontSize: fontSize.xs,
    fontWeight: "700",
    color: colors.textDisabled,
    letterSpacing: 1,
    marginBottom: spacing.md,
  },
  historyCard: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.md,
  },
  historyHeader: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
  },
  historyVersion: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
    fontFamily: "monospace",
  },
  statusBadge: {
    borderRadius: radius.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
  },
  statusText: {
    fontSize: fontSize.xs,
    fontWeight: "700",
  },
  historyTime: {
    fontSize: fontSize.xs,
    color: colors.textTertiary,
    marginTop: spacing.xs,
  },
  historyError: {
    fontSize: fontSize.xs,
    color: colors.error,
    marginTop: spacing.xs,
  },
  emptyBox: {
    alignItems: "center",
    gap: spacing.md,
    marginTop: spacing.xxxl,
  },
  emptyText: { fontSize: fontSize.base, color: colors.textTertiary },
});
