import { useState, useEffect } from "react";
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
  ActivityIndicator,
} from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useSubscriptionDetail, useUpdateSubscription } from "../../api/subscription";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { UpdateSubscriptionRequest } from "../../types/api";

export default function EditSubscriptionScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { data, isLoading } = useSubscriptionDetail(id ?? "");
  const updateMutation = useUpdateSubscription();

  const [name, setName] = useState("");
  const [sourceUrl, setSourceUrl] = useState("");
  const [remark, setRemark] = useState("");
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    if (data && !loaded) {
      setName(data.name ?? "");
      setSourceUrl(data.source_url ?? "");
      setRemark(data.remark ?? "");
      setLoaded(true);
    }
  }, [data, loaded]);

  const handleSave = () => {
    if (!name.trim()) {
      Alert.alert("提示", "请输入订阅名称");
      return;
    }
    const req: UpdateSubscriptionRequest = {
      name: name.trim(),
    };
    if (sourceUrl.trim()) {
      req.source_url = sourceUrl.trim();
    }
    req.remark = remark.trim() || undefined;
    updateMutation.mutate(
      { id: id!, data: req },
      {
        onSuccess: () => {
          Alert.alert("保存成功", "订阅已更新", [
            { text: "好", onPress: () => router.back() },
          ]);
        },
        onError: (err: any) => Alert.alert("保存失败", err.message),
      },
    );
  };

  if (isLoading) {
    return (
      <View style={styles.loadingContainer}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }

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
          style={[styles.submitBtn, updateMutation.isPending && styles.submitBtnDisabled]}
          onPress={handleSave}
          disabled={updateMutation.isPending}
          activeOpacity={0.8}
        >
          <Text style={styles.submitText}>
            {updateMutation.isPending ? "保存中..." : "保存修改"}
          </Text>
        </TouchableOpacity>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  loadingContainer: { flex: 1, justifyContent: "center", alignItems: "center", backgroundColor: colors.bg },
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
