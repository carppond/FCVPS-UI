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
import { useVpsAssetDetail, useUpdateVpsAsset } from "../../api/vps-asset";
import { colors, spacing, radius, fontSize } from "../../lib/theme";
import type { UpdateVpsAssetRequest, BillingCycle } from "../../types/api";

const CURRENCY_OPTIONS = ["CNY", "USD", "EUR", "GBP"] as const;
const BILLING_OPTIONS: { label: string; value: BillingCycle }[] = [
  { label: "月付", value: "monthly" },
  { label: "季付", value: "quarterly" },
  { label: "半年付", value: "semi_annual" },
  { label: "年付", value: "annual" },
  { label: "两年付", value: "biennial" },
  { label: "三年付", value: "triennial" },
];

export default function EditVpsAssetScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { data, isLoading } = useVpsAssetDetail(id ?? "");
  const updateMutation = useUpdateVpsAsset();

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
  const [os, setOs] = useState("");
  const [notes, setNotes] = useState("");
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    if (data && !loaded) {
      setName(data.name ?? "");
      setProvider(data.provider ?? "");
      setIp(data.ip ?? "");
      setLocation(data.location ?? "");
      setPrice(String(data.price ?? 0));
      setCurrency(data.currency ?? "CNY");
      setBillingCycle(data.billing_cycle ?? "monthly");
      setExpireAt(data.expire_at ? data.expire_at.slice(0, 10) : "");
      setCpu(data.cpu ?? "");
      setMemory(data.memory ?? "");
      setDisk(data.disk ?? "");
      setBandwidth(data.bandwidth ?? "");
      setSshPort(String(data.ssh_port ?? 22));
      setSshUser(data.ssh_user ?? "");
      setOs(data.os ?? "");
      setNotes(data.notes ?? "");
      setLoaded(true);
    }
  }, [data, loaded]);

  const handleSave = () => {
    if (!name.trim()) {
      Alert.alert("提示", "请输入名称");
      return;
    }
    if (!provider.trim()) {
      Alert.alert("提示", "请输入提供商");
      return;
    }
    if (!expireAt.trim()) {
      Alert.alert("提示", "请输入到期时间");
      return;
    }

    const req: UpdateVpsAssetRequest = {
      name: name.trim(),
      provider: provider.trim(),
      price: parseFloat(price) || 0,
      currency,
      billing_cycle: billingCycle,
      expire_at: expireAt.trim(),
    };
    if (ip.trim()) req.ip = ip.trim();
    else req.ip = null;
    if (location.trim()) req.location = location.trim();
    else req.location = null;
    if (cpu.trim()) req.cpu = cpu.trim();
    else req.cpu = null;
    if (memory.trim()) req.memory = memory.trim();
    else req.memory = null;
    if (disk.trim()) req.disk = disk.trim();
    else req.disk = null;
    if (bandwidth.trim()) req.bandwidth = bandwidth.trim();
    else req.bandwidth = null;
    if (sshPort.trim()) req.ssh_port = parseInt(sshPort, 10) || 22;
    if (sshUser.trim()) req.ssh_user = sshUser.trim();
    else req.ssh_user = null;
    if (os.trim()) req.os = os.trim();
    else req.os = null;
    if (notes.trim()) req.notes = notes.trim();
    else req.notes = null;

    updateMutation.mutate(
      { id: id!, data: req },
      {
        onSuccess: () => {
          Alert.alert("保存成功", "VPS 资产已更新", [
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
        {/* Basic info */}
        <View style={styles.card}>
          <View style={styles.cardHeader}>
            <View style={[styles.cardIcon, { backgroundColor: colors.primarySoft }]}>
              <Ionicons name="information-circle-outline" size={16} color={colors.primary} />
            </View>
            <Text style={styles.cardTitle}>基本信息</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>名称 <Text style={styles.required}>*</Text></Text>
            <TextInput
              style={styles.input}
              value={name}
              onChangeText={setName}
              placeholder="如：香港 DMIT"
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>提供商 <Text style={styles.required}>*</Text></Text>
            <TextInput
              style={styles.input}
              value={provider}
              onChangeText={setProvider}
              placeholder="如：DMIT"
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>IP 地址</Text>
            <TextInput
              style={styles.input}
              value={ip}
              onChangeText={setIp}
              placeholder="如：1.2.3.4"
              placeholderTextColor={colors.textDisabled}
              autoCapitalize="none"
              keyboardType="decimal-pad"
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>地区</Text>
            <TextInput
              style={styles.input}
              value={location}
              onChangeText={setLocation}
              placeholder="如：Hong Kong"
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
            <Text style={styles.cardTitle}>费用与到期</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>价格</Text>
            <TextInput
              style={styles.input}
              value={price}
              onChangeText={setPrice}
              placeholder="0.00"
              placeholderTextColor={colors.textDisabled}
              keyboardType="decimal-pad"
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>币种</Text>
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
            <Text style={styles.label}>付费周期</Text>
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
            <Text style={styles.label}>到期时间 <Text style={styles.required}>*</Text></Text>
            <TextInput
              style={styles.input}
              value={expireAt}
              onChangeText={setExpireAt}
              placeholder="YYYY-MM-DD"
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
            <Text style={styles.cardTitle}>硬件配置</Text>
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>CPU</Text>
            <TextInput
              style={styles.input}
              value={cpu}
              onChangeText={setCpu}
              placeholder="如：1C"
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>内存</Text>
            <TextInput
              style={styles.input}
              value={memory}
              onChangeText={setMemory}
              placeholder="如：1G"
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>硬盘</Text>
            <TextInput
              style={styles.input}
              value={disk}
              onChangeText={setDisk}
              placeholder="如：20G SSD"
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>带宽</Text>
            <TextInput
              style={styles.input}
              value={bandwidth}
              onChangeText={setBandwidth}
              placeholder="如：1Gbps"
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
            <Text style={styles.label}>SSH 端口</Text>
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
            <Text style={styles.label}>SSH 用户</Text>
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
            <Text style={styles.label}>操作系统</Text>
            <TextInput
              style={styles.input}
              value={os}
              onChangeText={setOs}
              placeholder="如：Ubuntu 22.04"
              placeholderTextColor={colors.textDisabled}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>备注</Text>
            <TextInput
              style={[styles.input, styles.textArea]}
              value={notes}
              onChangeText={setNotes}
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
