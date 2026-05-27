import { useState, useCallback } from "react";
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
import * as Clipboard from "expo-clipboard";
import { useShortLinksQuery, useCreateShortLink } from "../api/shortlink";
import { colors, spacing, radius, fontSize } from "../lib/theme";
import type { ShortLink } from "../types/api";

function formatDate(ts?: number): string {
  if (!ts) return "永久";
  return new Date(ts * 1000).toLocaleDateString("zh-CN");
}

export default function ShortLinksScreen() {
  const { data, isLoading, refetch } = useShortLinksQuery();
  const createMutation = useCreateShortLink();
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
    Alert.alert("已复制", url);
  };

  const handleCreate = () => {
    if (!targetUrl.trim()) {
      Alert.alert("提示", "请输入目标 URL");
      return;
    }
    const body: { target_url: string; expires_at?: number } = {
      target_url: targetUrl.trim(),
    };
    if (expiresAt.trim()) {
      const ts = Math.floor(new Date(expiresAt.trim()).getTime() / 1000);
      if (isNaN(ts)) {
        Alert.alert("提示", "过期时间格式无效，请使用 YYYY-MM-DD");
        return;
      }
      body.expires_at = ts;
    }
    createMutation.mutate(body, {
      onSuccess: () => {
        setModalVisible(false);
        setTargetUrl("");
        setExpiresAt("");
        Alert.alert("创建成功", "短链已添加");
      },
      onError: (err: any) => Alert.alert("创建失败", err.message),
    });
  };

  const renderItem = ({ item }: { item: ShortLink }) => (
    <View style={styles.card}>
      <View style={styles.cardBody}>
        <Text style={styles.shortUrl} numberOfLines={1}>
          {item.short_url}
        </Text>
        <Text style={styles.targetUrl} numberOfLines={1}>
          {item.target_url}
        </Text>
        <View style={styles.metaRow}>
          <Text style={styles.metaText}>
            创建: {formatDate(item.created_at)}
          </Text>
          <Text style={styles.metaText}>
            过期: {formatDate(item.expires_at)}
          </Text>
        </View>
      </View>
      <TouchableOpacity
        style={styles.copyBtn}
        onPress={() => copyUrl(item.short_url)}
        activeOpacity={0.6}
      >
        <Ionicons name="copy-outline" size={16} color={colors.primary} />
      </TouchableOpacity>
    </View>
  );

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
              <Text style={styles.emptyText}>暂无短链</Text>
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
              <Text style={styles.modalTitle}>新建短链</Text>
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
                目标 URL <Text style={styles.required}>*</Text>
              </Text>
              <TextInput
                style={styles.input}
                value={targetUrl}
                onChangeText={setTargetUrl}
                placeholder="https://example.com/long-url"
                placeholderTextColor={colors.textDisabled}
                autoCapitalize="none"
                autoCorrect={false}
                keyboardType="url"
              />
            </View>

            <View style={styles.field}>
              <Text style={styles.label}>过期时间</Text>
              <TextInput
                style={styles.input}
                value={expiresAt}
                onChangeText={setExpiresAt}
                placeholder="YYYY-MM-DD（留空为永久）"
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
                {createMutation.isPending ? "创建中..." : "创建短链"}
              </Text>
            </TouchableOpacity>
          </View>
        </KeyboardAvoidingView>
      </Modal>
    </View>
  );
}

const styles = StyleSheet.create({
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
  copyBtn: {
    width: 36,
    height: 36,
    borderRadius: radius.md,
    backgroundColor: colors.primarySoft,
    justifyContent: "center",
    alignItems: "center",
    marginLeft: spacing.md,
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
