import { useState, useEffect, useCallback, useMemo } from "react";
import {
  View,
  Text,
  ScrollView,
  StyleSheet,
  TextInput,
  TouchableOpacity,
  RefreshControl,
  Alert,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../../lib/api-client";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";

interface SettingField {
  key: string;
  label: string;
  hint: string;
  section: string;
}

const FIELDS: SettingField[] = [
  {
    key: "session_ttl_seconds",
    label: "会话有效期（秒）",
    hint: "登录 Token 过期时间",
    section: "账户",
  },
  {
    key: "default_locale",
    label: "默认语言",
    hint: "新用户默认语言，如 zh-CN",
    section: "账户",
  },
  {
    key: "monthly_reset_day",
    label: "月流量重置日",
    hint: "每月第几天重置流量，1-28",
    section: "流量",
  },
  {
    key: "monthly_traffic_limit",
    label: "月流量限额",
    hint: "单位字节，0 为不限",
    section: "流量",
  },
  {
    key: "agent_heartbeat_interval",
    label: "探针心跳间隔（秒）",
    hint: "Agent 上报间隔",
    section: "探针",
  },
  {
    key: "notification_debounce",
    label: "通知去抖（秒）",
    hint: "相同通知最小间隔",
    section: "通知",
  },
];

const SECTIONS = ["账户", "流量", "探针", "通知"];

function sectionIcon(section: string): keyof typeof Ionicons.glyphMap {
  switch (section) {
    case "账户":
      return "person-outline";
    case "流量":
      return "analytics-outline";
    case "探针":
      return "pulse-outline";
    case "通知":
      return "notifications-outline";
    default:
      return "settings-outline";
  }
}

export default function AdminSettingsScreen() {
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const queryClient = useQueryClient();
  const [values, setValues] = useState<Record<string, string>>({});
  const [refreshing, setRefreshing] = useState(false);

  const { data, refetch } = useQuery({
    queryKey: ["admin", "settings"],
    queryFn: () => apiFetch<Record<string, string>>("/api/admin/settings"),
  });

  useEffect(() => {
    if (data) {
      setValues(data);
    }
  }, [data]);

  const saveMutation = useMutation({
    mutationFn: (body: Record<string, string>) =>
      apiFetch<void>("/api/admin/settings", {
        method: "PUT",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "settings"] });
      Alert.alert("保存成功", "系统设置已更新");
    },
    onError: (err: any) => Alert.alert("保存失败", err.message),
  });

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const handleSave = () => {
    saveMutation.mutate(values);
  };

  const updateValue = (key: string, val: string) => {
    setValues((prev) => ({ ...prev, [key]: val }));
  };

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
      {SECTIONS.map((section) => {
        const sectionFields = FIELDS.filter((f) => f.section === section);
        return (
          <View key={section} style={styles.card}>
            <View style={styles.cardHeader}>
              <View
                style={[styles.cardIcon, { backgroundColor: colors.infoBg }]}
              >
                <Ionicons
                  name={sectionIcon(section)}
                  size={16}
                  color={colors.info}
                />
              </View>
              <Text style={styles.cardTitle}>{section}</Text>
            </View>
            {sectionFields.map((field) => (
              <View key={field.key} style={styles.field}>
                <Text style={styles.label}>{field.label}</Text>
                <Text style={styles.hint}>{field.hint}</Text>
                <TextInput
                  style={styles.input}
                  value={values[field.key] ?? ""}
                  onChangeText={(val) => updateValue(field.key, val)}
                  placeholder="--"
                  placeholderTextColor={colors.textDisabled}
                  autoCapitalize="none"
                  autoCorrect={false}
                />
              </View>
            ))}
          </View>
        );
      })}

      <TouchableOpacity
        style={[
          styles.saveBtn,
          saveMutation.isPending && styles.saveBtnDisabled,
        ]}
        onPress={handleSave}
        disabled={saveMutation.isPending}
        activeOpacity={0.8}
      >
        <Ionicons name="save-outline" size={18} color="#fff" />
        <Text style={styles.saveBtnText}>
          {saveMutation.isPending ? "保存中..." : "保存设置"}
        </Text>
      </TouchableOpacity>
    </ScrollView>
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
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
  },
  field: { marginBottom: spacing.lg },
  label: {
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.textSecondary,
    marginBottom: 2,
  },
  hint: {
    fontSize: fontSize.xs,
    color: colors.textTertiary,
    marginBottom: spacing.sm,
  },
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
  saveBtn: {
    flexDirection: "row",
    height: 50,
    borderRadius: radius.lg,
    backgroundColor: colors.primary,
    justifyContent: "center",
    alignItems: "center",
    gap: spacing.sm,
    marginTop: spacing.sm,
  },
  saveBtnDisabled: { opacity: 0.5 },
  saveBtnText: { fontSize: fontSize.lg, fontWeight: "700", color: "#fff" },
});
