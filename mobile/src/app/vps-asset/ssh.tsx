import { useState, useEffect, useRef } from "react";
import {
  View,
  Text,
  TextInput,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  Alert,
  KeyboardAvoidingView,
  Platform,
  ActivityIndicator,
} from "react-native";
import { useLocalSearchParams, router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import * as ScreenOrientation from "expo-screen-orientation";
// eslint-disable-next-line @typescript-eslint/no-var-requires
const SSHClient = require("react-native-ssh-sftp").default;
import { useVpsAssetDetail } from "../../api/vps-asset";

const TERM_BG = "#0a0a0c";
const TERM_FG = "#22c55e";
const TERM_ERR = "#ef4444";
const TERM_DIM = "#8a8a96";
const TERM_PROMPT = "#3b82f6";

interface LogLine {
  type: "in" | "out" | "err" | "sys";
  text: string;
}

export default function SSHTerminalScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { data: vps, isLoading } = useVpsAssetDetail(id ?? "");
  const [connecting, setConnecting] = useState(false);
  const [connected, setConnected] = useState(false);
  const [command, setCommand] = useState("");
  const [logs, setLogs] = useState<LogLine[]>([]);
  const clientRef = useRef<any>(null);
  const scrollRef = useRef<ScrollView | null>(null);

  // Lock landscape on mount, restore on unmount
  useEffect(() => {
    ScreenOrientation.lockAsync(ScreenOrientation.OrientationLock.LANDSCAPE);
    return () => {
      ScreenOrientation.lockAsync(ScreenOrientation.OrientationLock.PORTRAIT_UP);
      // Disconnect on leave
      if (clientRef.current) {
        try {
          clientRef.current.disconnect();
        } catch {}
        clientRef.current = null;
      }
    };
  }, []);

  const appendLog = (line: LogLine) => {
    setLogs((prev) => [...prev, line]);
    setTimeout(() => scrollRef.current?.scrollToEnd({ animated: true }), 50);
  };

  const connect = async () => {
    if (!vps?.ip || !vps?.ssh_user) {
      Alert.alert("缺少信息", "VPS 缺少 IP 或 SSH 用户名");
      return;
    }
    if (!vps.ssh_password && !vps.ssh_private_key) {
      Alert.alert("缺少凭据", "请在 VPS 编辑页面填写 SSH 密码或私钥");
      return;
    }
    setConnecting(true);
    appendLog({ type: "sys", text: `Connecting to ${vps.ssh_user}@${vps.ip}:${vps.ssh_port}...` });
    try {
      const client = vps.ssh_private_key
        ? await SSHClient.connectWithKey(vps.ip, vps.ssh_port, vps.ssh_user, vps.ssh_private_key, "")
        : await SSHClient.connectWithPassword(vps.ip, vps.ssh_port, vps.ssh_user, vps.ssh_password!);
      clientRef.current = client;
      setConnected(true);
      appendLog({ type: "sys", text: "✓ Connected" });
    } catch (err: any) {
      appendLog({ type: "err", text: `Connection failed: ${err.message || err}` });
    } finally {
      setConnecting(false);
    }
  };

  const execute = async () => {
    if (!command.trim() || !clientRef.current) return;
    const cmd = command.trim();
    setCommand("");
    appendLog({ type: "in", text: `$ ${cmd}` });
    try {
      const output = await clientRef.current.execute(cmd);
      if (output) {
        appendLog({ type: "out", text: String(output).trimEnd() });
      }
    } catch (err: any) {
      appendLog({ type: "err", text: err.message || String(err) });
    }
  };

  const disconnect = async () => {
    if (clientRef.current) {
      try {
        await clientRef.current.disconnect();
      } catch {}
      clientRef.current = null;
    }
    setConnected(false);
    appendLog({ type: "sys", text: "✗ Disconnected" });
  };

  if (isLoading || !vps) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color={TERM_FG} />
      </View>
    );
  }

  return (
    <KeyboardAvoidingView style={styles.container} behavior={Platform.OS === "ios" ? "padding" : undefined}>
      {/* Header */}
      <View style={styles.header}>
        <TouchableOpacity onPress={() => router.back()} style={styles.headerBtn}>
          <Ionicons name="close" size={20} color={TERM_FG} />
        </TouchableOpacity>
        <Text style={styles.headerTitle} numberOfLines={1}>
          {vps.name} {vps.ssh_user}@{vps.ip}
        </Text>
        {connected ? (
          <TouchableOpacity onPress={disconnect} style={styles.headerBtn}>
            <Text style={styles.disconnectText}>断开</Text>
          </TouchableOpacity>
        ) : (
          <TouchableOpacity
            onPress={connect}
            style={styles.headerBtn}
            disabled={connecting}
          >
            <Text style={styles.connectText}>{connecting ? "..." : "连接"}</Text>
          </TouchableOpacity>
        )}
      </View>

      {/* Terminal */}
      <ScrollView
        ref={scrollRef}
        style={styles.terminal}
        contentContainerStyle={styles.terminalContent}
      >
        {logs.length === 0 && (
          <Text style={[styles.logLine, { color: TERM_DIM }]}>
            点击右上角"连接"开始 SSH 会话
          </Text>
        )}
        {logs.map((l, i) => (
          <Text
            key={i}
            style={[
              styles.logLine,
              l.type === "in" && { color: TERM_PROMPT },
              l.type === "err" && { color: TERM_ERR },
              l.type === "sys" && { color: TERM_DIM },
            ]}
            selectable
          >
            {l.text}
          </Text>
        ))}
      </ScrollView>

      {/* Input */}
      <View style={styles.inputRow}>
        <Text style={styles.prompt}>$</Text>
        <TextInput
          style={styles.input}
          value={command}
          onChangeText={setCommand}
          placeholder={connected ? "输入命令..." : "未连接"}
          placeholderTextColor={TERM_DIM}
          autoCapitalize="none"
          autoCorrect={false}
          editable={connected}
          onSubmitEditing={execute}
          returnKeyType="send"
          blurOnSubmit={false}
        />
        <TouchableOpacity
          style={[styles.sendBtn, !connected && styles.sendBtnDisabled]}
          onPress={execute}
          disabled={!connected || !command.trim()}
        >
          <Ionicons name="send" size={18} color={connected ? TERM_FG : TERM_DIM} />
        </TouchableOpacity>
      </View>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: TERM_BG },
  center: { flex: 1, justifyContent: "center", alignItems: "center", backgroundColor: TERM_BG },
  header: {
    flexDirection: "row", alignItems: "center", gap: 12,
    paddingHorizontal: 16, paddingVertical: 12,
    borderBottomWidth: 1, borderBottomColor: "rgba(255,255,255,0.08)",
  },
  headerBtn: { padding: 4 },
  headerTitle: { flex: 1, fontSize: 14, fontWeight: "600", color: TERM_FG, fontFamily: "monospace" },
  connectText: { fontSize: 14, fontWeight: "700", color: TERM_FG },
  disconnectText: { fontSize: 14, fontWeight: "700", color: TERM_ERR },
  terminal: { flex: 1 },
  terminalContent: { padding: 12 },
  logLine: { fontFamily: "monospace", fontSize: 12, color: TERM_FG, lineHeight: 18 },
  inputRow: {
    flexDirection: "row", alignItems: "center", gap: 8,
    paddingHorizontal: 12, paddingVertical: 10,
    borderTopWidth: 1, borderTopColor: "rgba(255,255,255,0.08)",
    backgroundColor: "#0f0f11",
  },
  prompt: { color: TERM_PROMPT, fontFamily: "monospace", fontSize: 14, fontWeight: "700" },
  input: {
    flex: 1, fontFamily: "monospace", fontSize: 13, color: TERM_FG,
    padding: 6, height: 36,
  },
  sendBtn: { padding: 8 },
  sendBtnDisabled: { opacity: 0.3 },
});
