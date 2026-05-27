import { useState } from "react";
import {
  View,
  Text,
  TextInput,
  ScrollView,
  StyleSheet,
  TouchableOpacity,
  Alert,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../../lib/api-client";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { CreateSubscriptionRequest, Subscription } from "../../types/api";

export default function CreateSubscriptionScreen() {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [sourceUrl, setSourceUrl] = useState("");
  const [remark, setRemark] = useState("");

  const createMutation = useMutation({
    mutationFn: (data: CreateSubscriptionRequest) =>
      apiFetch<Subscription>("/api/subscriptions", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["subscription"] });
      Alert.alert("创建成功", "订阅已添加", [
        { text: "好", onPress: () => router.back() },
      ]);
    },
    onError: (err: any) => Alert.alert("创建失败", err.message),
  });

  const handleCreate = () => {
    if (!name.trim()) {
      Alert.alert("提示", "请输入订阅名称");
      return;
    }
    const req: CreateSubscriptionRequest = {
      name: name.trim(),
      type: sourceUrl.trim() ? "url" : "manual",
    };
    if (sourceUrl.trim()) {
      req.source_url = sourceUrl.trim();
    }
    if (remark.trim()) {
      req.remark = remark.trim();
    }
    createMutation.mutate(req);
  };

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <ScrollView contentContainerStyle={styles.content}>
        {/* Name */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.primarySoft }]}>
              <Ionicons name="book-outline" size={16} color={colors.primary} />
            </View>
            <Text style={styles.cardTitle}>基本信息</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>名称 <Text style={styles.required}>*</Text></Text>
            <TextInput
              style={styles.input}
              value={name}
              onChangeText={setName}
              placeholder="如：我的订阅"
              placeholderTextColor={colors.textDisabled}
            />
          </View>
        </View>

        {/* Source URL */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.infoBg }]}>
              <Ionicons name="link-outline" size={16} color={colors.info} />
            </View>
            <Text style={styles.cardTitle}>订阅源</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>URL</Text>
            <TextInput
              style={styles.input}
              value={sourceUrl}
              onChangeText={setSourceUrl}
              placeholder="https://example.com/subscribe?token=xxx"
              placeholderTextColor={colors.textDisabled}
              autoCapitalize="none"
              autoCorrect={false}
              keyboardType="url"
            />
            <Text style={styles.hint}>留空则创建手动订阅，可稍后添加节点</Text>
          </View>
        </View>

        {/* Remark */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: "rgba(255,255,255,0.04)" }]}>
              <Ionicons name="chatbubble-outline" size={16} color={colors.textTertiary} />
            </View>
            <Text style={styles.cardTitle}>备注</Text>
          </View>
          <View style={styles.field}>
            <TextInput
              style={[styles.input, styles.textArea]}
              value={remark}
              onChangeText={setRemark}
              placeholder="可选备注"
              placeholderTextColor={colors.textDisabled}
              multiline
              numberOfLines={3}
              textAlignVertical="top"
            />
          </View>
        </View>

        {/* Submit */}
        <TouchableOpacity
          style={[styles.submitBtn, createMutation.isPending && styles.submitBtnDisabled]}
          onPress={handleCreate}
          disabled={createMutation.isPending}
          activeOpacity={0.8}
        >
          <Text style={styles.submitText}>
            {createMutation.isPending ? "创建中..." : "创建订阅"}
          </Text>
        </TouchableOpacity>
      </ScrollView>
    </KeyboardAvoidingView>
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
  cardHeader: { flexDirection: "row", alignItems: "center", gap: spacing.sm, marginBottom: spacing.lg },
  cardIcon: { width: 28, height: 28, borderRadius: radius.md, justifyContent: "center", alignItems: "center" },
  cardTitle: { fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
  field: { gap: spacing.xs },
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
  textArea: { height: 80, paddingTop: spacing.md },
  hint: { fontSize: fontSize.xs, color: colors.textDisabled, marginTop: 2 },
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
