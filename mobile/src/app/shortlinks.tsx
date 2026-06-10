import { useState, useCallback, useMemo } from "react";
import {
  View,
  Text,
  FlatList,
  StyleSheet,
  TouchableOpacity,
  RefreshControl,
  Alert,
  Modal,
  TextInput,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import * as Clipboard from "expo-clipboard";
import { useShortLinksQuery, useCreateShortLink, useDeleteShortLink } from "../api/shortlink";
import { parseShortLinkTarget } from "../lib/shortlink-target";
import { spacing, radius, fontSize, type AppColors } from "../lib/theme";
import { useColors } from "../lib/useColors";
import type { ShortLink } from "../types/api";
import { formatApiError } from "../lib/format-api-error";

function formatDate(ts: number | undefined, t: TFunction): string {
  if (!ts) return t("shortlink_permanent");
  return new Date(ts * 1000).toLocaleDateString("zh-CN");
}

export default function ShortLinksScreen() {
  const { t } = useTranslation(["rules", "common"]);
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const { data, isLoading, refetch } = useShortLinksQuery();
  const createMutation = useCreateShortLink();
  const deleteMutation = useDeleteShortLink();
  const [refreshing, setRefreshing] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [targetUrl, setTargetUrl] = useState("");
  const [expiresAt, setExpiresAt] = useState("");

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data ?? [];

  const copyUrl = async (url: string) => {
    await Clipboard.setStringAsync(url);
    Alert.alert(t("common:copied"), url);
  };

  const handleCreate = () => {
    if (!targetUrl.trim()) {
      Alert.alert(t("common:tip"), t("shortlink_target_required"));
      return;
    }
    const body: { target_url: string; expires_at?: number } = {
      target_url: targetUrl.trim(),
    };
    if (expiresAt.trim()) {
      const ts = Math.floor(new Date(expiresAt.trim()).getTime() / 1000);
      if (isNaN(ts)) {
        Alert.alert(t("common:tip"), t("shortlink_expires_invalid"));
        return;
      }
      body.expires_at = ts;
    }
    createMutation.mutate(body, {
      onSuccess: () => {
        setModalVisible(false);
        setTargetUrl("");
        setExpiresAt("");
        Alert.alert(t("shortlink_create_success"), t("shortlink_created_one"));
      },
      onError: (err: any) => Alert.alert(t("common:create_failed"), formatApiError(err, t)),
    });
  };

  const handleDelete = (item: ShortLink) => {
    Alert.alert(t("common:delete_confirm_title"), t("shortlink_delete_confirm", { url: item.short_url }), [
      { text: t("common:cancel"), style: "cancel" },
      {
        text: t("common:delete"),
        style: "destructive",
        onPress: () => {
          deleteMutation.mutate(
            { fileCode: item.file_code, userCode: item.user_code },
            {
              onSuccess: () => Alert.alert(t("shortlink_deleted"), t("shortlink_deleted_one")),
              onError: (err: any) => Alert.alert(t("common:delete_failed"), formatApiError(err, t)),
            },
          );
        },
      },
    ]);
  };

  const renderItem = ({ item }: { item: ShortLink }) => {
    const target = parseShortLinkTarget(item.target_url);
    return (
    <View style={styles.card}>
      <View style={styles.cardBody}>
        <Text style={styles.shortUrl} numberOfLines={1}>
          {item.short_url}
        </Text>
        {target ? (
          <View style={styles.subBadge}>
            <Text style={styles.subBadgeText} numberOfLines={1}>
              {t("shortlink_subscription", { name: target.subscriptionName })}
              {target.client ? ` · ${target.client}` : ""}
            </Text>
          </View>
        ) : null}
        <Text style={styles.targetUrl} numberOfLines={1}>
          {item.target_url}
        </Text>
        <View style={styles.metaRow}>
          <Text style={styles.metaText}>
            {t("shortlink_meta_created", { date: formatDate(item.created_at, t) })}
          </Text>
          <Text style={styles.metaText}>
            {t("shortlink_meta_expires", { date: formatDate(item.expires_at, t) })}
          </Text>
        </View>
      </View>
      <View style={styles.cardBtnGroup}>
        <TouchableOpacity
          style={styles.copyBtn}
          onPress={() => copyUrl(item.short_url)}
          activeOpacity={0.6}
        >
          <Ionicons name="copy-outline" size={16} color={colors.primary} />
        </TouchableOpacity>
        <TouchableOpacity
          style={styles.deleteBtn}
          onPress={() => handleDelete(item)}
          activeOpacity={0.6}
        >
          <Ionicons name="trash-outline" size={16} color={colors.error} />
        </TouchableOpacity>
      </View>
    </View>
    );
  };

  return (
    <View style={styles.container}>
      <FlatList
        contentContainerStyle={items.length === 0 ? styles.empty : styles.list}
        data={items}
        keyExtractor={(item) => item.file_code}
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
                name="link-outline"
                size={48}
                color={colors.textDisabled}
              />
              <Text style={styles.emptyText}>{t("shortlink_empty")}</Text>
            </View>
          ) : null
        }
        renderItem={renderItem}
      />

      {/* Floating add button */}
      <TouchableOpacity
        style={styles.fab}
        onPress={() => setModalVisible(true)}
        activeOpacity={0.8}
      >
        <Ionicons name="add" size={28} color="#fff" />
      </TouchableOpacity>

      {/* Create modal */}
      <Modal
        visible={modalVisible}
        animationType="slide"
        transparent
        onRequestClose={() => setModalVisible(false)}
      >
        <KeyboardAvoidingView
          style={styles.modalOverlay}
          behavior={Platform.OS === "ios" ? "padding" : "height"}
        >
          <View style={styles.modalContent}>
            <View style={styles.modalHeader}>
              <Text style={styles.modalTitle}>{t("shortlink_create_modal_title")}</Text>
              <TouchableOpacity
                onPress={() => setModalVisible(false)}
                activeOpacity={0.6}
              >
                <Ionicons
                  name="close"
                  size={24}
                  color={colors.textTertiary}
                />
              </TouchableOpacity>
            </View>

            <View style={styles.field}>
              <Text style={styles.label}>
                {t("shortlink_label_target")} <Text style={styles.required}>*</Text>
              </Text>
              <TextInput
                style={styles.input}
                value={targetUrl}
                onChangeText={setTargetUrl}
                placeholder={t("shortlink_target_placeholder")}
                placeholderTextColor={colors.textDisabled}
                autoCapitalize="none"
                autoCorrect={false}
                keyboardType="url"
              />
            </View>

            <View style={styles.field}>
              <Text style={styles.label}>{t("shortlink_label_expires")}</Text>
              <TextInput
                style={styles.input}
                value={expiresAt}
                onChangeText={setExpiresAt}
                placeholder={t("shortlink_expires_placeholder")}
                placeholderTextColor={colors.textDisabled}
              />
            </View>

            <TouchableOpacity
              style={[
                styles.submitBtn,
                createMutation.isPending && styles.submitBtnDisabled,
              ]}
              onPress={handleCreate}
              disabled={createMutation.isPending}
              activeOpacity={0.8}
            >
              <Text style={styles.submitText}>
                {createMutation.isPending ? t("common:creating") : t("shortlink_submit")}
              </Text>
            </TouchableOpacity>
          </View>
        </KeyboardAvoidingView>
      </Modal>
    </View>
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
  cardBody: { flex: 1, gap: 4 },
  shortUrl: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.primary,
    fontFamily: "monospace",
  },
  subBadge: {
    alignSelf: "flex-start",
    backgroundColor: colors.primarySoft,
    borderRadius: radius.sm,
    paddingHorizontal: 6,
    paddingVertical: 2,
  },
  subBadgeText: {
    fontSize: fontSize.xs,
    fontWeight: "600",
    color: colors.primary,
  },
  targetUrl: {
    fontSize: fontSize.sm,
    color: colors.textSecondary,
  },
  metaRow: {
    flexDirection: "row",
    gap: spacing.lg,
    marginTop: spacing.xs,
  },
  metaText: { fontSize: fontSize.xs, color: colors.textTertiary },
  cardBtnGroup: {
    flexDirection: "row",
    gap: spacing.sm,
    marginLeft: spacing.md,
  },
  copyBtn: {
    width: 36,
    height: 36,
    borderRadius: radius.md,
    backgroundColor: colors.primarySoft,
    justifyContent: "center",
    alignItems: "center",
  },
  deleteBtn: {
    width: 36,
    height: 36,
    borderRadius: radius.md,
    backgroundColor: colors.errorBg,
    justifyContent: "center",
    alignItems: "center",
  },
  fab: {
    position: "absolute",
    right: spacing.xl,
    bottom: spacing.xxxl,
    width: 56,
    height: 56,
    borderRadius: 28,
    backgroundColor: colors.primary,
    justifyContent: "center",
    alignItems: "center",
    elevation: 4,
    shadowColor: "#000",
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.3,
    shadowRadius: 4,
  },
  modalOverlay: {
    flex: 1,
    justifyContent: "flex-end",
    backgroundColor: "rgba(0,0,0,0.5)",
  },
  modalContent: {
    backgroundColor: colors.surface,
    borderTopLeftRadius: radius.xl,
    borderTopRightRadius: radius.xl,
    padding: spacing.xl,
    paddingBottom: 40,
  },
  modalHeader: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    marginBottom: spacing.xl,
  },
  modalTitle: {
    fontSize: fontSize.lg,
    fontWeight: "700",
    color: colors.textPrimary,
  },
  field: { gap: spacing.xs, marginBottom: spacing.lg },
  label: { fontSize: fontSize.sm, fontWeight: "600", color: colors.textSecondary },
  required: { color: colors.primary, fontSize: fontSize.xs },
  input: {
    height: 48,
    borderRadius: radius.lg,
    borderWidth: 1,
    borderColor: colors.borderStrong,
    backgroundColor: colors.elevated,
    paddingHorizontal: spacing.lg,
    fontSize: fontSize.base,
    color: colors.textPrimary,
  },
  submitBtn: {
    height: 50,
    borderRadius: radius.lg,
    backgroundColor: colors.primary,
    justifyContent: "center",
    alignItems: "center",
    marginTop: spacing.sm,
  },
  submitBtnDisabled: { opacity: 0.5 },
  submitText: { fontSize: fontSize.lg, fontWeight: "700", color: "#fff" },
});
