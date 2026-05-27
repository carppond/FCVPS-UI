import { useState, useCallback } from "react";
import {
  View,
  Text,
  FlatList,
  StyleSheet,
  RefreshControl,
  TouchableOpacity,
  Alert,
} from "react-native";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useNotificationChannelsQuery, useDeleteChannel } from "../api/notify";
import { colors, spacing, radius, fontSize } from "../lib/theme";
import type { NotificationChannel, ChannelKind } from "../types/api";

function channelIcon(kind: ChannelKind): keyof typeof Ionicons.glyphMap {
  switch (kind) {
    case "telegram":
      return "paper-plane-outline";
    case "email":
      return "mail-outline";
    case "discord":
      return "logo-discord";
    case "slack":
      return "chatbubbles-outline";
    case "bark":
      return "notifications-outline";
    case "gotify":
      return "megaphone-outline";
    case "webhook":
      return "code-slash-outline";
    case "serverchan":
      return "server-outline";
    case "pushdeer":
      return "push-outline";
    case "ifttt":
      return "git-network-outline";
    default:
      return "notifications-outline";
  }
}

function channelLabel(kind: ChannelKind): string {
  const map: Record<ChannelKind, string> = {
    telegram: "Telegram",
    email: "Email",
    discord: "Discord",
    slack: "Slack",
    bark: "Bark",
    gotify: "Gotify",
    webhook: "Webhook",
    serverchan: "Server酱",
    pushdeer: "PushDeer",
    ifttt: "IFTTT",
  };
  return map[kind] ?? kind;
}

export default function NotificationsScreen() {
  const { data, isLoading, refetch } = useNotificationChannelsQuery();
  const deleteMutation = useDeleteChannel();
  const [refreshing, setRefreshing] = useState(false);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const handleDelete = (item: NotificationChannel) => {
    Alert.alert("删除确认", `确定删除通知渠道「${item.name}」吗？`, [
      { text: "取消", style: "cancel" },
      {
        text: "删除",
        style: "destructive",
        onPress: () => {
          deleteMutation.mutate(item.id, {
            onSuccess: () => refetch(),
            onError: (err: any) => Alert.alert("删除失败", err.message),
          });
        },
      },
    ]);
  };

  const renderItem = ({ item }: { item: NotificationChannel }) => (
    <TouchableOpacity
      style={styles.card}
      onPress={() => router.push(`/notification/create`)}
      onLongPress={() => handleDelete(item)}
      activeOpacity={0.7}
    >
      <View style={styles.cardLeft}>
        <View
          style={[
            styles.iconBox,
            {
              backgroundColor: item.enabled
                ? colors.primarySoft
                : "rgba(255,255,255,0.04)",
            },
          ]}
        >
          <Ionicons
            name={channelIcon(item.kind)}
            size={18}
            color={item.enabled ? colors.primary : colors.textTertiary}
          />
        </View>
      </View>
      <View style={styles.cardBody}>
        <Text style={styles.cardName} numberOfLines={1}>
          {item.name}
        </Text>
        <View style={styles.badgeRow}>
          <View style={styles.kindBadge}>
            <Text style={styles.kindText}>{channelLabel(item.kind)}</Text>
          </View>
          <Text style={styles.eventCount}>
            {item.event_types.length} 个事件
          </Text>
        </View>
      </View>
      <View
        style={[
          styles.statusDot,
          { backgroundColor: item.enabled ? colors.success : colors.textDisabled },
        ]}
      />
    </TouchableOpacity>
  );

  return (
    <View style={styles.wrapper}>
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
        ListHeaderComponent={
          <TouchableOpacity
            style={styles.addBtn}
            onPress={() => router.push("/notification/create")}
            activeOpacity={0.7}
          >
            <Ionicons name="add-circle-outline" size={16} color={colors.primary} />
            <Text style={styles.addBtnText}>新建渠道</Text>
          </TouchableOpacity>
        }
        ListEmptyComponent={
          !isLoading ? (
            <View style={styles.emptyBox}>
              <Ionicons
                name="notifications-off-outline"
                size={48}
                color={colors.textDisabled}
              />
              <Text style={styles.emptyText}>暂无通知渠道</Text>
            </View>
          ) : null
        }
        renderItem={renderItem}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  wrapper: { flex: 1, backgroundColor: colors.bg },
  container: { flex: 1 },
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
    justifyContent: "center",
    alignItems: "center",
  },
  cardBody: { flex: 1, gap: 4 },
  cardName: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
  },
  badgeRow: { flexDirection: "row", alignItems: "center", gap: spacing.sm },
  kindBadge: {
    backgroundColor: colors.elevated,
    borderRadius: radius.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
  },
  kindText: {
    fontSize: fontSize.xs,
    fontWeight: "600",
    color: colors.textSecondary,
  },
  eventCount: { fontSize: fontSize.xs, color: colors.textTertiary },
  addBtn: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.sm,
    backgroundColor: colors.primarySoft,
    borderRadius: radius.lg,
    padding: spacing.md,
    marginBottom: spacing.lg,
  },
  addBtnText: {
    fontSize: fontSize.sm,
    fontWeight: "700",
    color: colors.primary,
  },
  statusDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
    marginLeft: spacing.md,
  },
});
