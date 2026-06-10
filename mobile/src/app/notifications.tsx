import { useState, useCallback, useMemo } from "react";
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
import { useTranslation } from "react-i18next";
import { useNotificationChannelsQuery, useDeleteChannel } from "../api/notify";
import { spacing, radius, fontSize, type AppColors } from "../lib/theme";
import { useColors } from "../lib/useColors";
import type { NotificationChannel, ChannelKind } from "../types/api";
import { formatApiError } from "../lib/format-api-error";

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
    serverchan: "ServerChan",
    pushdeer: "PushDeer",
    ifttt: "IFTTT",
  };
  return map[kind] ?? kind;
}

export default function NotificationsScreen() {
  const { t } = useTranslation(["notify", "common"]);
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
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
    Alert.alert(
      t("common:delete_confirm_title"),
      t("delete_confirm_msg", { name: item.name }),
      [
        { text: t("common:cancel"), style: "cancel" },
        {
          text: t("common:delete"),
          style: "destructive",
          onPress: () => {
            deleteMutation.mutate(item.id, {
              onSuccess: () => refetch(),
              onError: (err: any) =>
                Alert.alert(t("common:delete_failed"), formatApiError(err, t)),
            });
          },
        },
      ],
    );
  };

  const handleEdit = (item: NotificationChannel) => {
    const eventTypesStr = item.event_types.join(",");
    router.push(
      `/notification/create?editId=${item.id}&editName=${encodeURIComponent(item.name)}&editKind=${item.kind}&editEnabled=${item.enabled}&editEventTypes=${encodeURIComponent(eventTypesStr)}&editConfig=${encodeURIComponent(JSON.stringify(item.config))}`,
    );
  };

  const renderItem = ({ item }: { item: NotificationChannel }) => (
    <TouchableOpacity
      style={styles.card}
      onPress={() => handleEdit(item)}
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
            {t("event_count", { count: item.event_types.length })}
          </Text>
        </View>
      </View>
      <Ionicons name="chevron-forward" size={16} color={colors.textDisabled} />
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
            <Text style={styles.addBtnText}>{t("create_title")}</Text>
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
              <Text style={styles.emptyText}>{t("empty")}</Text>
            </View>
          ) : null
        }
        renderItem={renderItem}
      />
    </View>
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
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
    marginLeft: spacing.sm,
  },
});
