import { useState } from "react";
import { View, Text, TextInput, ScrollView, StyleSheet, TouchableOpacity, Alert } from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useUpdateAgent } from "../../api/agent";
import { colors, spacing, radius, fontSize } from "../../lib/theme";

export default function EditAgentScreen() {
  const { id, name: initialName } = useLocalSearchParams<{ id: string; name: string }>();
  const updateMutation = useUpdateAgent();
  const [name, setName] = useState(initialName ?? "");

  const handleSave = () => {
    if (!name.trim()) {
      Alert.alert("提示", "请输入探针名称");
      return;
    }
    updateMutation.mutate(
      { id: id!, data: { name: name.trim() } },
      {
        onSuccess: () => {
          Alert.alert("保存成功", "探针已更新", [
            { text: "好", onPress: () => router.back() },
          ]);
        },
        onError: (err: any) => Alert.alert("保存失败", err.message),
      },
    );
  };

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <View style={styles.card}>
        <View style={styles.cardHeader}>
          <View style={[styles.cardIcon, { backgroundColor: colors.primarySoft }]}>
            <Ionicons name="radio-outline" size={16} color={colors.primary} />
          </View>
          <Text style={styles.cardTitle}>编辑探针</Text>
        </View>
        <View style={styles.field}>
          <Text style={styles.label}>名称</Text>
          <TextInput
            style={styles.input}
            value={name}
            onChangeText={setName}
            placeholder="探针名称"
            placeholderTextColor={colors.textDisabled}
          />
        </View>
      </View>

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
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.xl, paddingBottom: 40 },
  card: {
    backgroundColor: colors.surface, borderRadius: radius.xl,
    borderWidth: 1, borderColor: colors.border, padding: spacing.xl, marginBottom: spacing.lg,
  },
  cardHeader: { flexDirection: "row", alignItems: "center", gap: spacing.sm, marginBottom: spacing.lg },
  cardIcon: { width: 28, height: 28, borderRadius: radius.md, justifyContent: "center", alignItems: "center" },
  cardTitle: { fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
  field: { gap: spacing.xs },
  label: { fontSize: fontSize.sm, fontWeight: "600", color: colors.textSecondary },
  input: {
    height: 48, borderRadius: radius.lg, borderWidth: 1,
    borderColor: colors.borderStrong, backgroundColor: colors.elevated,
    paddingHorizontal: spacing.lg, fontSize: fontSize.base, color: colors.textPrimary,
  },
  submitBtn: {
    height: 50, borderRadius: radius.lg, backgroundColor: colors.primary,
    justifyContent: "center", alignItems: "center",
  },
  submitBtnDisabled: { opacity: 0.5 },
  submitText: { fontSize: fontSize.lg, fontWeight: "700", color: "#fff" },
});
