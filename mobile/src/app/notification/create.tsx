import { useState, useMemo } from "react";
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
  Switch,
} from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useQueryClient } from "@tanstack/react-query";
import { useCreateChannel, useUpdateChannel } from "../../api/notify";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";
import type {
  ChannelKind,
  EventType,
  ChannelConfig,
} from "../../types/api";

const CHANNEL_KINDS: { key: ChannelKind; label: string; icon: string }[] = [
  { key: "telegram", label: "Telegram", icon: "paper-plane-outline" },
  { key: "email", label: "Email", icon: "mail-outline" },
  { key: "bark", label: "Bark", icon: "notifications-outline" },
  { key: "webhook", label: "Webhook", icon: "code-slash-outline" },
  { key: "discord", label: "Discord", icon: "logo-discord" },
  { key: "slack", label: "Slack", icon: "chatbubbles-outline" },
  { key: "gotify", label: "Gotify", icon: "megaphone-outline" },
  { key: "serverchan", label: "ServerChan", icon: "server-outline" },
  { key: "pushdeer", label: "PushDeer", icon: "push-outline" },
  { key: "ifttt", label: "IFTTT", icon: "git-network-outline" },
];

const buildEventTypes = (
  t: TFunction,
): { key: EventType; label: string }[] => [
  { key: "node_offline", label: t("event_node_offline") },
  { key: "traffic_threshold", label: t("event_traffic_threshold") },
  { key: "subscription_sync_failed", label: t("event_subscription_sync_failed") },
  { key: "backup_completed", label: t("event_backup_completed") },
  { key: "login_anomaly", label: t("event_login_anomaly") },
  { key: "ota_available", label: t("event_ota_available") },
  { key: "script_alert", label: t("event_script_alert") },
  { key: "vps_expiry", label: t("event_vps_expiry") },
];

interface ConfigField {
  key: string;
  label: string;
  placeholder: string;
  secure?: boolean;
}

function getConfigFields(
  kind: ChannelKind,
  t: TFunction,
): ConfigField[] {
  switch (kind) {
    case "telegram":
      return [
        { key: "bot_token", label: t("field_bot_token"), placeholder: t("ph_bot_token"), secure: true },
        { key: "chat_id", label: t("field_chat_id"), placeholder: t("ph_chat_id") },
      ];
    case "email":
      return [
        { key: "smtp_host", label: t("field_smtp_host"), placeholder: "smtp.example.com" },
        { key: "smtp_port", label: t("field_smtp_port"), placeholder: "465" },
        { key: "smtp_user", label: t("field_smtp_user"), placeholder: "user@example.com" },
        { key: "smtp_password", label: t("field_smtp_password"), placeholder: t("ph_smtp_password"), secure: true },
        { key: "from", label: t("field_from"), placeholder: "noreply@example.com" },
        { key: "to", label: t("field_to"), placeholder: "user@example.com" },
      ];
    case "bark":
      return [
        { key: "device_key", label: t("field_device_key"), placeholder: t("ph_device_key") },
        { key: "server_url", label: t("field_server_url_optional"), placeholder: "https://api.day.app" },
      ];
    case "webhook":
      return [
        { key: "url", label: t("field_url"), placeholder: "https://example.com/webhook" },
      ];
    case "discord":
      return [
        { key: "webhook_url", label: t("field_webhook_url"), placeholder: "https://discord.com/api/webhooks/..." },
      ];
    case "slack":
      return [
        { key: "webhook_url", label: t("field_webhook_url"), placeholder: "https://hooks.slack.com/..." },
      ];
    case "gotify":
      return [
        { key: "server_url", label: t("field_server_url"), placeholder: "https://gotify.example.com" },
        { key: "app_token", label: t("field_app_token"), placeholder: t("ph_app_token"), secure: true },
      ];
    case "serverchan":
      return [
        { key: "send_key", label: t("field_send_key"), placeholder: t("ph_send_key"), secure: true },
      ];
    case "pushdeer":
      return [
        { key: "push_key", label: t("field_push_key"), placeholder: t("ph_push_key"), secure: true },
        { key: "server_url", label: t("field_server_url_optional"), placeholder: "https://api2.pushdeer.com" },
      ];
    case "ifttt":
      return [
        { key: "webhook_key", label: t("field_webhook_key"), placeholder: t("ph_webhook_key"), secure: true },
        { key: "event_name", label: t("field_event_name"), placeholder: t("ph_event_name") },
      ];
    default:
      return [];
  }
}

function parseEditConfig(configStr?: string): Record<string, string> {
  if (!configStr) return {};
  try {
    const obj = JSON.parse(configStr) as Record<string, unknown>;
    const result: Record<string, string> = {};
    for (const [key, value] of Object.entries(obj)) {
      if (Array.isArray(value)) {
        result[key] = value.join(",");
      } else if (value != null) {
        result[key] = String(value);
      }
    }
    return result;
  } catch {
    return {};
  }
}

export default function NotificationCreateScreen() {
  const { t } = useTranslation(["notify", "common"]);
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const eventTypes = useMemo(() => buildEventTypes(t), [t]);
  const params = useLocalSearchParams<{
    editId?: string;
    editName?: string;
    editKind?: string;
    editEnabled?: string;
    editEventTypes?: string;
    editConfig?: string;
  }>();

  const isEdit = !!params.editId;

  const queryClient = useQueryClient();
  const createMutation = useCreateChannel();
  const updateMutation = useUpdateChannel();

  const [name, setName] = useState(params.editName ?? "");
  const [kind, setKind] = useState<ChannelKind>((params.editKind as ChannelKind) ?? "telegram");
  const [configValues, setConfigValues] = useState<Record<string, string>>(
    parseEditConfig(params.editConfig),
  );
  const [selectedEvents, setSelectedEvents] = useState<Set<EventType>>(
    new Set(
      params.editEventTypes
        ? (params.editEventTypes.split(",") as EventType[])
        : [],
    ),
  );

  const configFields = useMemo(() => getConfigFields(kind, t), [kind, t]);

  const updateConfig = (key: string, value: string) => {
    setConfigValues((prev) => ({ ...prev, [key]: value }));
  };

  const toggleEvent = (event: EventType) => {
    setSelectedEvents((prev) => {
      const next = new Set(prev);
      if (next.has(event)) next.delete(event);
      else next.add(event);
      return next;
    });
  };

  const buildConfig = (): ChannelConfig => {
    const config: Record<string, unknown> = {};
    for (const field of configFields) {
      const val = configValues[field.key]?.trim() ?? "";
      if (val) {
        if (field.key === "smtp_port") {
          config[field.key] = parseInt(val, 10) || 465;
        } else if (field.key === "to") {
          config[field.key] = val.split(",").map((s) => s.trim());
        } else if (field.key === "smtp_tls") {
          config[field.key] = val === "true";
        } else {
          config[field.key] = val;
        }
      }
    }
    // email defaults
    if (kind === "email") {
      if (!config.smtp_tls) config.smtp_tls = true;
    }
    return config as unknown as ChannelConfig;
  };

  const handleSubmit = () => {
    if (!name.trim()) {
      Alert.alert(t("common:tip"), t("validate_name_required"));
      return;
    }
    if (selectedEvents.size === 0) {
      Alert.alert(t("common:tip"), t("validate_select_event"));
      return;
    }

    if (isEdit) {
      updateMutation.mutate(
        {
          id: params.editId!,
          data: {
            name: name.trim(),
            config: buildConfig(),
            event_types: Array.from(selectedEvents),
            enabled: params.editEnabled === "true",
          },
        },
        {
          onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["notify"] });
            Alert.alert(t("common:save_success"), t("saved_msg"), [
              { text: t("common:ok"), onPress: () => router.back() },
            ]);
          },
          onError: (err: any) =>
            Alert.alert(t("common:save_failed"), err.message),
        },
      );
      return;
    }

    createMutation.mutate(
      {
        kind,
        name: name.trim(),
        config: buildConfig(),
        event_types: Array.from(selectedEvents),
        enabled: true,
      },
      {
        onSuccess: () => {
          queryClient.invalidateQueries({ queryKey: ["notify"] });
          Alert.alert(t("common:success"), t("create_success_msg"), [
            { text: t("common:ok"), onPress: () => router.back() },
          ]);
        },
        onError: (err: any) =>
          Alert.alert(t("common:create_failed"), err.message),
      },
    );
  };

  const isPending = createMutation.isPending || updateMutation.isPending;

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <ScrollView contentContainerStyle={styles.content}>
        {/* Name */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View
              style={[styles.cardIcon, { backgroundColor: colors.primarySoft }]}
            >
              <Ionicons
                name="create-outline"
                size={16}
                color={colors.primary}
              />
            </View>
            <Text style={styles.cardTitle}>{t("name_title")}</Text>
          </View>
          <View style={styles.field}>
            <TextInput
              style={styles.input}
              value={name}
              onChangeText={setName}
              placeholder={t("name_placeholder")}
              placeholderTextColor={colors.textDisabled}
            />
          </View>
        </View>

        {/* Kind Selector */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.infoBg }]}>
              <Ionicons
                name="apps-outline"
                size={16}
                color={colors.info}
              />
            </View>
            <Text style={styles.cardTitle}>{t("kind_title")}</Text>
          </View>
          <View style={styles.kindGrid}>
            {CHANNEL_KINDS.map((ck) => (
              <TouchableOpacity
                key={ck.key}
                style={[
                  styles.kindChip,
                  kind === ck.key && styles.kindChipActive,
                  isEdit && kind !== ck.key && styles.kindChipEditDisabled,
                ]}
                onPress={() => {
                  if (isEdit) return; // Don't allow changing kind in edit mode
                  setKind(ck.key);
                  setConfigValues({});
                }}
                activeOpacity={isEdit ? 1 : 0.7}
              >
                <Ionicons
                  name={ck.icon as keyof typeof Ionicons.glyphMap}
                  size={16}
                  color={kind === ck.key ? colors.primary : colors.textTertiary}
                />
                <Text
                  style={[
                    styles.kindChipText,
                    kind === ck.key && styles.kindChipTextActive,
                  ]}
                >
                  {ck.label}
                </Text>
              </TouchableOpacity>
            ))}
          </View>
        </View>

        {/* Config Fields */}
        {configFields.length > 0 && (
          <View style={styles.card}>
            <View style={styles.cardHeader}>
              <View
                style={[
                  styles.cardIcon,
                  { backgroundColor: colors.warningBg },
                ]}
              >
                <Ionicons
                  name="settings-outline"
                  size={16}
                  color={colors.warning}
                />
              </View>
              <Text style={styles.cardTitle}>{t("config_title")}</Text>
            </View>
            {configFields.map((field) => (
              <View key={field.key} style={[styles.field, { marginBottom: spacing.md }]}>
                <Text style={styles.label}>{field.label}</Text>
                <TextInput
                  style={styles.input}
                  value={configValues[field.key] ?? ""}
                  onChangeText={(v) => updateConfig(field.key, v)}
                  placeholder={field.placeholder}
                  placeholderTextColor={colors.textDisabled}
                  secureTextEntry={field.secure}
                  autoCapitalize="none"
                  autoCorrect={false}
                />
              </View>
            ))}
          </View>
        )}

        {/* Event Types */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View
              style={[styles.cardIcon, { backgroundColor: colors.successBg }]}
            >
              <Ionicons
                name="flash-outline"
                size={16}
                color={colors.success}
              />
            </View>
            <Text style={styles.cardTitle}>{t("event_types_title")}</Text>
          </View>
          {eventTypes.map((et) => (
            <TouchableOpacity
              key={et.key}
              style={styles.eventRow}
              onPress={() => toggleEvent(et.key)}
              activeOpacity={0.7}
            >
              <Text style={styles.eventLabel}>{et.label}</Text>
              <Switch
                value={selectedEvents.has(et.key)}
                onValueChange={() => toggleEvent(et.key)}
                trackColor={{
                  false: colors.surfaceHover,
                  true: colors.primary,
                }}
                thumbColor="#fff"
              />
            </TouchableOpacity>
          ))}
        </View>

        {/* Submit */}
        <TouchableOpacity
          style={[
            styles.submitBtn,
            isPending && styles.submitBtnDisabled,
          ]}
          onPress={handleSubmit}
          disabled={isPending}
          activeOpacity={0.8}
        >
          <Text style={styles.submitText}>
            {isEdit
              ? isPending ? t("common:saving") : t("edit_btn")
              : isPending ? t("common:creating") : t("create_btn")}
          </Text>
        </TouchableOpacity>
      </ScrollView>
    </KeyboardAvoidingView>
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
  field: { gap: spacing.xs },
  label: {
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.textSecondary,
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
  kindGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: spacing.sm,
  },
  kindChip: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    borderRadius: radius.lg,
    backgroundColor: colors.elevated,
    borderWidth: 1,
    borderColor: colors.border,
  },
  kindChipActive: {
    backgroundColor: colors.primarySoft,
    borderColor: colors.primary,
  },
  kindChipEditDisabled: {
    opacity: 0.4,
  },
  kindChipText: {
    fontSize: fontSize.xs,
    fontWeight: "600",
    color: colors.textTertiary,
  },
  kindChipTextActive: {
    color: colors.primary,
    fontWeight: "700",
  },
  eventRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingVertical: spacing.sm,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  eventLabel: {
    fontSize: fontSize.base,
    color: colors.textSecondary,
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
