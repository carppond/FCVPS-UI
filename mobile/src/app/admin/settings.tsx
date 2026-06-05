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
import { useTranslation } from "react-i18next";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../../lib/api-client";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";

type SectionKey = "account" | "traffic" | "agent" | "notify";

interface SettingField {
  key: string;
  section: SectionKey;
}

const FIELDS: SettingField[] = [
  { key: "session_ttl_seconds", section: "account" },
  { key: "default_locale", section: "account" },
  { key: "monthly_reset_day", section: "traffic" },
  { key: "monthly_traffic_limit", section: "traffic" },
  { key: "agent_heartbeat_interval", section: "agent" },
  { key: "notification_debounce", section: "notify" },
];

const SECTIONS: SectionKey[] = ["account", "traffic", "agent", "notify"];

function sectionIcon(section: SectionKey): keyof typeof Ionicons.glyphMap {
  switch (section) {
    case "account":
      return "person-outline";
    case "traffic":
      return "analytics-outline";
    case "agent":
      return "pulse-outline";
    case "notify":
      return "notifications-outline";
    default:
      return "settings-outline";
  }
}

export default function AdminSettingsScreen() {
  const { t } = useTranslation(["settings", "common"]);
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
      Alert.alert(t("common:save_success"), t("admin_settings_save_success_msg"));
    },
    onError: (err: any) => Alert.alert(t("common:save_failed"), err.message),
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
              <Text style={styles.cardTitle}>
                {t(`admin_settings_section_${section}`)}
              </Text>
            </View>
            {sectionFields.map((field) => (
              <View key={field.key} style={styles.field}>
                <Text style={styles.label}>
                  {t(`admin_settings_field_${field.key}_label`)}
                </Text>
                <Text style={styles.hint}>
                  {t(`admin_settings_field_${field.key}_hint`)}
                </Text>
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
          {saveMutation.isPending
            ? t("common:saving")
            : t("admin_settings_save_btn")}
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
