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
} from "react-native";
import { router } from "expo-router";
import { useTranslation } from "react-i18next";
import { Ionicons } from "@expo/vector-icons";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../../lib/api-client";
import { spacing, radius, fontSize, type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";
import type { CreateVpsAssetRequest, VpsAsset, BillingCycle } from "../../types/api";
import { formatApiError } from "../../lib/format-api-error";

const CURRENCY_OPTIONS = ["CNY", "USD", "EUR", "GBP"] as const;

type TFn = (key: string) => string;

function buildBillingOptions(t: TFn): { label: string; value: BillingCycle }[] {
  return [
    { label: t("billing_cycle_monthly"), value: "monthly" },
    { label: t("billing_cycle_quarterly"), value: "quarterly" },
    { label: t("billing_cycle_semi_annual"), value: "semi_annual" },
    { label: t("billing_cycle_annual"), value: "annual" },
    { label: t("billing_cycle_biennial"), value: "biennial" },
    { label: t("billing_cycle_triennial"), value: "triennial" },
  ];
}

export default function CreateVpsAssetScreen() {
  const { t } = useTranslation("vps");
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const queryClient = useQueryClient();
  const BILLING_OPTIONS = useMemo(() => buildBillingOptions(t), [t]);

  const [name, setName] = useState("");
  const [provider, setProvider] = useState("");
  const [ip, setIp] = useState("");
  const [location, setLocation] = useState("");
  const [price, setPrice] = useState("");
  const [currency, setCurrency] = useState("CNY");
  const [billingCycle, setBillingCycle] = useState<BillingCycle>("monthly");
  const [expireAt, setExpireAt] = useState("");
  const [cpu, setCpu] = useState("");
  const [memory, setMemory] = useState("");
  const [disk, setDisk] = useState("");
  const [bandwidth, setBandwidth] = useState("");
  const [sshPort, setSshPort] = useState("22");
  const [sshUser, setSshUser] = useState("");
  const [sshPassword, setSshPassword] = useState("");
  const [sshPrivateKey, setSshPrivateKey] = useState("");
  const [os, setOs] = useState("");
  const [notes, setNotes] = useState("");

  const createMutation = useMutation({
    mutationFn: (data: CreateVpsAssetRequest) =>
      apiFetch<VpsAsset>("/api/vps-assets", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["vps-asset"] });
      Alert.alert(t("create_success"), t("create_success_message"), [
        { text: t("ok"), onPress: () => router.back() },
      ]);
    },
    onError: (err: any) => Alert.alert(t("create_failed"), formatApiError(err, t)),
  });

  const handleCreate = () => {
    if (!name.trim()) {
      Alert.alert(t("tip"), t("tip_name_required"));
      return;
    }
    if (!provider.trim()) {
      Alert.alert(t("tip"), t("tip_provider_required"));
      return;
    }
    if (!expireAt.trim()) {
      Alert.alert(t("tip"), t("tip_expire_at_required"));
      return;
    }

    const req: CreateVpsAssetRequest = {
      name: name.trim(),
      provider: provider.trim(),
      price: parseFloat(price) || 0,
      currency,
      billing_cycle: billingCycle,
      expire_at: expireAt.trim(),
    };
    if (ip.trim()) req.ip = ip.trim();
    if (location.trim()) req.location = location.trim();
    if (cpu.trim()) req.cpu = cpu.trim();
    if (memory.trim()) req.memory = memory.trim();
    if (disk.trim()) req.disk = disk.trim();
    if (bandwidth.trim()) req.bandwidth = bandwidth.trim();
    if (sshPort.trim()) req.ssh_port = parseInt(sshPort, 10) || 22;
    if (sshUser.trim()) req.ssh_user = sshUser.trim();
    if (sshPassword) req.ssh_password = sshPassword;
    if (sshPrivateKey.trim()) req.ssh_private_key = sshPrivateKey.trim();
    if (os.trim()) req.os = os.trim();
    if (notes.trim()) req.notes = notes.trim();

    createMutation.mutate(req);
  };

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <ScrollView contentContainerStyle={styles.content}>
        {/* Basic info */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.primarySoft }]}>
              <Ionicons name="information-circle-outline" size={16} color={colors.primary} />
            </View>
            <Text style={styles.cardTitle}>{t("basic_info")}</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("name")} <Text style={styles.required}>*</Text></Text>
            <TextInput
              style={styles.input}
              value={name}
              onChangeText={setName}
              placeholder={t("name_placeholder")}
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("provider")} <Text style={styles.required}>*</Text></Text>
            <TextInput
              style={styles.input}
              value={provider}
              onChangeText={setProvider}
              placeholder={t("provider_placeholder")}
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("ip_address")}</Text>
            <TextInput
              style={styles.input}
              value={ip}
              onChangeText={setIp}
              placeholder={t("ip_placeholder")}
              placeholderTextColor={colors.textDisabled}
              autoCapitalize="none"
              keyboardType="decimal-pad"
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("location")}</Text>
            <TextInput
              style={styles.input}
              value={location}
              onChangeText={setLocation}
              placeholder={t("location_placeholder")}
              placeholderTextColor={colors.textDisabled}
            />
          </View>
        </View>

        {/* Price & expiry */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.warningBg }]}>
              <Ionicons name="cash-outline" size={16} color={colors.warning} />
            </View>
            <Text style={styles.cardTitle}>{t("cost_and_expiry")}</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("price")}</Text>
            <TextInput
              style={styles.input}
              value={price}
              onChangeText={setPrice}
              placeholder={t("price_placeholder")}
              placeholderTextColor={colors.textDisabled}
              keyboardType="decimal-pad"
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("currency")}</Text>
            <View style={styles.chipRow}>
              {CURRENCY_OPTIONS.map((c) => (
                <TouchableOpacity
                  key={c}
                  style={[
                    styles.chip,
                    currency === c && styles.chipActive,
                  ]}
                  onPress={() => setCurrency(c)}
                  activeOpacity={0.7}
                >
                  <Text
                    style={[
                      styles.chipText,
                      currency === c && styles.chipTextActive,
                    ]}
                  >
                    {c}
                  </Text>
                </TouchableOpacity>
              ))}
            </View>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("billing_cycle")}</Text>
            <View style={styles.chipRow}>
              {BILLING_OPTIONS.map((opt) => (
                <TouchableOpacity
                  key={opt.value}
                  style={[
                    styles.chip,
                    billingCycle === opt.value && styles.chipActive,
                  ]}
                  onPress={() => setBillingCycle(opt.value)}
                  activeOpacity={0.7}
                >
                  <Text
                    style={[
                      styles.chipText,
                      billingCycle === opt.value && styles.chipTextActive,
                    ]}
                  >
                    {opt.label}
                  </Text>
                </TouchableOpacity>
              ))}
            </View>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("expire_at")} <Text style={styles.required}>*</Text></Text>
            <TextInput
              style={styles.input}
              value={expireAt}
              onChangeText={setExpireAt}
              placeholder={t("expire_at_placeholder")}
              placeholderTextColor={colors.textDisabled}
            />
          </View>
        </View>

        {/* Hardware */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.infoBg }]}>
              <Ionicons name="hardware-chip-outline" size={16} color={colors.info} />
            </View>
            <Text style={styles.cardTitle}>{t("hardware_config")}</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("cpu")}</Text>
            <TextInput
              style={styles.input}
              value={cpu}
              onChangeText={setCpu}
              placeholder={t("cpu_placeholder")}
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("memory")}</Text>
            <TextInput
              style={styles.input}
              value={memory}
              onChangeText={setMemory}
              placeholder={t("memory_placeholder")}
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("disk")}</Text>
            <TextInput
              style={styles.input}
              value={disk}
              onChangeText={setDisk}
              placeholder={t("disk_placeholder")}
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("bandwidth")}</Text>
            <TextInput
              style={styles.input}
              value={bandwidth}
              onChangeText={setBandwidth}
              placeholder={t("bandwidth_placeholder")}
              placeholderTextColor={colors.textDisabled}
            />
          </View>
        </View>

        {/* SSH */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.successBg }]}>
              <Ionicons name="terminal-outline" size={16} color={colors.success} />
            </View>
            <Text style={styles.cardTitle}>SSH</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("ssh_port")}</Text>
            <TextInput
              style={styles.input}
              value={sshPort}
              onChangeText={setSshPort}
              placeholder="22"
              placeholderTextColor={colors.textDisabled}
              keyboardType="number-pad"
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("ssh_user")}</Text>
            <TextInput
              style={styles.input}
              value={sshUser}
              onChangeText={setSshUser}
              placeholder="root"
              placeholderTextColor={colors.textDisabled}
              autoCapitalize="none"
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("ssh_password_optional")}</Text>
            <TextInput
              style={styles.input}
              value={sshPassword}
              onChangeText={setSshPassword}
              placeholder={t("ssh_password_placeholder")}
              placeholderTextColor={colors.textDisabled}
              secureTextEntry
              autoCapitalize="none"
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("ssh_private_key_optional")}</Text>
            <TextInput
              style={[styles.input, styles.textArea]}
              value={sshPrivateKey}
              onChangeText={setSshPrivateKey}
              placeholder="-----BEGIN RSA PRIVATE KEY-----..."
              placeholderTextColor={colors.textDisabled}
              multiline
              numberOfLines={4}
              autoCapitalize="none"
              autoCorrect={false}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("os")}</Text>
            <TextInput
              style={styles.input}
              value={os}
              onChangeText={setOs}
              placeholder={t("os_placeholder")}
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>{t("notes")}</Text>
            <TextInput
              style={[styles.input, styles.textArea]}
              value={notes}
              onChangeText={setNotes}
              placeholder={t("notes_placeholder")}
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
            {createMutation.isPending ? t("creating") : t("create_vps")}
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
  field: { gap: spacing.xs, marginBottom: spacing.md },
  label: {
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.textSecondary,
  },
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
  chipRow: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: spacing.sm,
  },
  chip: {
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    borderRadius: radius.md,
    backgroundColor: colors.elevated,
    borderWidth: 1,
    borderColor: colors.borderStrong,
  },
  chipActive: {
    backgroundColor: colors.primarySoft,
    borderColor: colors.primary,
  },
  chipText: {
    fontSize: fontSize.sm,
    fontWeight: "600",
    color: colors.textSecondary,
  },
  chipTextActive: {
    color: colors.primary,
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
