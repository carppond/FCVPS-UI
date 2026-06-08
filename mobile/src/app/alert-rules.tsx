import { useMemo, useState } from "react";
import {
  View,
  Text,
  FlatList,
  TouchableOpacity,
  StyleSheet,
  ActivityIndicator,
  Modal,
  TextInput,
  ScrollView,
  Switch,
  Alert,
  RefreshControl,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useTranslation } from "react-i18next";
import {
  useAlertRulesQuery,
  useCreateAlertRule,
  useUpdateAlertRule,
  useDeleteAlertRule,
} from "../api/alert-rule";
import { useAgentsQuery } from "../api/agent";
import { spacing, radius, fontSize, type AppColors } from "../lib/theme";
import { useColors } from "../lib/useColors";
import type { AlertMetric, AlertRule } from "../types/api";

const METRICS: AlertMetric[] = ["cpu", "mem", "disk", "offline"];

interface FormState {
  id?: string;
  name: string;
  agentId: string;
  metric: AlertMetric;
  threshold: string;
  durationSec: string;
  cooldownSec: string;
  enabled: boolean;
}

function emptyForm(): FormState {
  return {
    name: "",
    agentId: "",
    metric: "cpu",
    threshold: "80",
    durationSec: "0",
    cooldownSec: "3600",
    enabled: true,
  };
}

export default function AlertRulesScreen() {
  const { t } = useTranslation("alert");
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);

  const rulesQ = useAlertRulesQuery();
  const agentsQ = useAgentsQuery();
  const createMutation = useCreateAlertRule();
  const updateMutation = useUpdateAlertRule();
  const deleteMutation = useDeleteAlertRule();

  const rules = rulesQ.data?.items ?? [];
  const agents = agentsQ.data?.items ?? [];

  const [modalOpen, setModalOpen] = useState(false);
  const [form, setForm] = useState<FormState>(emptyForm);

  const metricLabel = (m: AlertMetric) => t(`metric_${m}`);
  const agentName = (id: string) => agents.find((a) => a.id === id)?.name ?? id;

  function openCreate() {
    setForm(emptyForm());
    setModalOpen(true);
  }

  function openEdit(rule: AlertRule) {
    setForm({
      id: rule.id,
      name: rule.name,
      agentId: rule.agent_id ?? "",
      metric: rule.metric,
      threshold: String(rule.threshold),
      durationSec: String(rule.duration_sec),
      cooldownSec: String(rule.cooldown_sec),
      enabled: rule.enabled,
    });
    setModalOpen(true);
  }

  async function handleSave() {
    if (!form.name.trim()) {
      Alert.alert(t("required_name"));
      return;
    }
    const base = {
      name: form.name.trim(),
      metric: form.metric,
      threshold: form.metric === "offline" ? 0 : Number(form.threshold) || 0,
      duration_sec: Number(form.durationSec) || 0,
      cooldown_sec: Number(form.cooldownSec) || 3600,
      enabled: form.enabled,
    };
    try {
      if (form.id) {
        await updateMutation.mutateAsync({
          id: form.id,
          data: { ...base, agent_id: form.agentId || null },
        });
      } else {
        await createMutation.mutateAsync({ ...base, agent_id: form.agentId || undefined });
      }
      setModalOpen(false);
    } catch (err: any) {
      Alert.alert(t("load_failed"), err?.message ?? "");
    }
  }

  function confirmDelete(rule: AlertRule) {
    Alert.alert(t("delete_confirm"), rule.name, [
      { text: t("cancel"), style: "cancel" },
      {
        text: t("delete"),
        style: "destructive",
        onPress: () => deleteMutation.mutate(rule.id),
      },
    ]);
  }

  function conditionText(rule: AlertRule): string {
    const dur = rule.duration_sec > 0 ? ` · ${Math.round(rule.duration_sec / 60)}m` : "";
    if (rule.metric === "offline") return `${metricLabel("offline")}${dur}`;
    return `${metricLabel(rule.metric)} ≥ ${rule.threshold}%${dur}`;
  }

  const saving = createMutation.isPending || updateMutation.isPending;

  return (
    <View style={styles.container}>
      <FlatList
        data={rules}
        keyExtractor={(r) => r.id}
        contentContainerStyle={styles.list}
        refreshControl={
          <RefreshControl refreshing={rulesQ.isFetching} onRefresh={() => rulesQ.refetch()} tintColor={colors.primary} />
        }
        ListEmptyComponent={
          rulesQ.isLoading ? (
            <ActivityIndicator style={{ marginTop: 40 }} color={colors.primary} />
          ) : (
            <View style={styles.empty}>
              <Ionicons name="notifications-off-outline" size={40} color={colors.textDisabled} />
              <Text style={styles.emptyTitle}>{t("empty")}</Text>
              <Text style={styles.emptyHint}>{t("empty_hint")}</Text>
            </View>
          )
        }
        renderItem={({ item }) => (
          <TouchableOpacity style={styles.card} onPress={() => openEdit(item)} activeOpacity={0.7}>
            <View style={{ flex: 1 }}>
              <Text style={styles.cardName}>{item.name}</Text>
              <Text style={styles.cardMeta}>
                {item.agent_id ? agentName(item.agent_id) : t("agent_all")} · {conditionText(item)}
              </Text>
            </View>
            <View style={[styles.statusBadge, { backgroundColor: item.enabled ? colors.successBg : colors.surfaceHover }]}>
              <Text style={[styles.statusText, { color: item.enabled ? colors.success : colors.textTertiary }]}>
                {item.enabled ? t("status_on") : t("status_off")}
              </Text>
            </View>
            <TouchableOpacity onPress={() => confirmDelete(item)} style={styles.delBtn} hitSlop={8}>
              <Ionicons name="trash-outline" size={18} color={colors.error} />
            </TouchableOpacity>
          </TouchableOpacity>
        )}
      />

      <TouchableOpacity style={styles.fab} onPress={openCreate} activeOpacity={0.85}>
        <Ionicons name="add" size={26} color="#fff" />
      </TouchableOpacity>

      <Modal visible={modalOpen} animationType="slide" transparent onRequestClose={() => setModalOpen(false)}>
        <View style={styles.modalBackdrop}>
          <View style={styles.modalSheet}>
            <View style={styles.modalHeader}>
              <Text style={styles.modalTitle}>{form.id ? t("edit") : t("create")}</Text>
              <TouchableOpacity onPress={() => setModalOpen(false)} hitSlop={8}>
                <Ionicons name="close" size={22} color={colors.textSecondary} />
              </TouchableOpacity>
            </View>
            <ScrollView contentContainerStyle={{ gap: spacing.lg, paddingBottom: spacing.xl }}>
              <View style={styles.field}>
                <Text style={styles.label}>{t("name")}</Text>
                <TextInput
                  style={styles.input}
                  value={form.name}
                  onChangeText={(v) => setForm({ ...form, name: v })}
                  placeholder={t("name_ph")}
                  placeholderTextColor={colors.textDisabled}
                />
              </View>

              <View style={styles.field}>
                <Text style={styles.label}>{t("agent")}</Text>
                <View style={styles.chipRow}>
                  <Chip
                    label={t("agent_all")}
                    active={form.agentId === ""}
                    onPress={() => setForm({ ...form, agentId: "" })}
                    colors={colors}
                  />
                  {agents.map((a) => (
                    <Chip
                      key={a.id}
                      label={a.name}
                      active={form.agentId === a.id}
                      onPress={() => setForm({ ...form, agentId: a.id })}
                      colors={colors}
                    />
                  ))}
                </View>
              </View>

              <View style={styles.field}>
                <Text style={styles.label}>{t("metric")}</Text>
                <View style={styles.chipRow}>
                  {METRICS.map((m) => (
                    <Chip
                      key={m}
                      label={metricLabel(m)}
                      active={form.metric === m}
                      onPress={() => setForm({ ...form, metric: m })}
                      colors={colors}
                    />
                  ))}
                </View>
              </View>

              {form.metric !== "offline" && (
                <View style={styles.field}>
                  <Text style={styles.label}>{t("threshold")}</Text>
                  <TextInput
                    style={styles.input}
                    value={form.threshold}
                    onChangeText={(v) => setForm({ ...form, threshold: v })}
                    keyboardType="number-pad"
                    placeholderTextColor={colors.textDisabled}
                  />
                </View>
              )}

              <View style={styles.rowFields}>
                <View style={[styles.field, { flex: 1 }]}>
                  <Text style={styles.label}>{t("duration")}</Text>
                  <TextInput
                    style={styles.input}
                    value={form.durationSec}
                    onChangeText={(v) => setForm({ ...form, durationSec: v })}
                    keyboardType="number-pad"
                    placeholderTextColor={colors.textDisabled}
                  />
                </View>
                <View style={[styles.field, { flex: 1 }]}>
                  <Text style={styles.label}>{t("cooldown")}</Text>
                  <TextInput
                    style={styles.input}
                    value={form.cooldownSec}
                    onChangeText={(v) => setForm({ ...form, cooldownSec: v })}
                    keyboardType="number-pad"
                    placeholderTextColor={colors.textDisabled}
                  />
                </View>
              </View>

              <View style={styles.switchRow}>
                <Text style={styles.label}>{t("enabled")}</Text>
                <Switch
                  value={form.enabled}
                  onValueChange={(v) => setForm({ ...form, enabled: v })}
                  trackColor={{ false: colors.border, true: colors.primarySoft }}
                  thumbColor={form.enabled ? colors.primary : colors.textDisabled}
                />
              </View>

              <TouchableOpacity
                style={[styles.saveBtn, saving && { opacity: 0.5 }]}
                onPress={handleSave}
                disabled={saving}
                activeOpacity={0.85}
              >
                <Text style={styles.saveText}>{saving ? t("saving") : t("save")}</Text>
              </TouchableOpacity>
            </ScrollView>
          </View>
        </View>
      </Modal>
    </View>
  );
}

function Chip({
  label,
  active,
  onPress,
  colors,
}: {
  label: string;
  active: boolean;
  onPress: () => void;
  colors: AppColors;
}) {
  return (
    <TouchableOpacity
      onPress={onPress}
      activeOpacity={0.7}
      style={{
        paddingHorizontal: spacing.md,
        paddingVertical: spacing.xs,
        borderRadius: radius.lg,
        borderWidth: 1,
        borderColor: active ? colors.primary : colors.border,
        backgroundColor: active ? colors.primarySoft : colors.elevated,
      }}
    >
      <Text style={{ fontSize: fontSize.sm, color: active ? colors.primary : colors.textSecondary, fontWeight: active ? "700" : "400" }}>
        {label}
      </Text>
    </TouchableOpacity>
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.bg },
    list: { padding: spacing.lg, gap: spacing.md, flexGrow: 1 },
    empty: { alignItems: "center", justifyContent: "center", paddingTop: 80, gap: spacing.sm },
    emptyTitle: { fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
    emptyHint: { fontSize: fontSize.sm, color: colors.textTertiary, textAlign: "center", paddingHorizontal: spacing.xl },
    card: {
      flexDirection: "row",
      alignItems: "center",
      gap: spacing.md,
      backgroundColor: colors.surface,
      borderRadius: radius.xl,
      borderWidth: 1,
      borderColor: colors.border,
      padding: spacing.lg,
    },
    cardName: { fontSize: fontSize.base, fontWeight: "700", color: colors.textPrimary },
    cardMeta: { fontSize: fontSize.sm, color: colors.textTertiary, marginTop: 2 },
    statusBadge: { paddingHorizontal: spacing.sm, paddingVertical: 2, borderRadius: radius.sm },
    statusText: { fontSize: fontSize.xs, fontWeight: "700" },
    delBtn: { padding: spacing.xs },
    fab: {
      position: "absolute",
      right: spacing.xl,
      bottom: spacing.xl,
      width: 56,
      height: 56,
      borderRadius: 28,
      backgroundColor: colors.primary,
      alignItems: "center",
      justifyContent: "center",
    },
    modalBackdrop: { flex: 1, backgroundColor: "rgba(0,0,0,0.4)", justifyContent: "flex-end" },
    modalSheet: {
      backgroundColor: colors.bg,
      borderTopLeftRadius: radius.xl,
      borderTopRightRadius: radius.xl,
      padding: spacing.xl,
      maxHeight: "88%",
    },
    modalHeader: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", marginBottom: spacing.lg },
    modalTitle: { fontSize: fontSize.lg, fontWeight: "800", color: colors.textPrimary },
    field: { gap: spacing.xs },
    rowFields: { flexDirection: "row", gap: spacing.md },
    label: { fontSize: fontSize.sm, fontWeight: "600", color: colors.textSecondary },
    input: {
      height: 46,
      borderRadius: radius.lg,
      borderWidth: 1,
      borderColor: colors.borderStrong,
      backgroundColor: colors.elevated,
      paddingHorizontal: spacing.lg,
      fontSize: fontSize.base,
      color: colors.textPrimary,
    },
    chipRow: { flexDirection: "row", flexWrap: "wrap", gap: spacing.sm },
    switchRow: { flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
    saveBtn: {
      height: 50,
      borderRadius: radius.lg,
      backgroundColor: colors.primary,
      alignItems: "center",
      justifyContent: "center",
      marginTop: spacing.sm,
    },
    saveText: { fontSize: fontSize.lg, fontWeight: "700", color: "#fff" },
  });
