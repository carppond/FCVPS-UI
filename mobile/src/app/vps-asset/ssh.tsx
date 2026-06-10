import { useState, useEffect, useMemo } from "react";
import { View, Text, StyleSheet, TouchableOpacity, ActivityIndicator } from "react-native";
import { useLocalSearchParams, router } from "expo-router";
import { useTranslation } from "react-i18next";
import { Ionicons } from "@expo/vector-icons";
import * as ScreenOrientation from "expo-screen-orientation";
import { WebView } from "react-native-webview";
import { Asset } from "expo-asset";
import { File } from "expo-file-system";
import { useVpsAssetDetail } from "../../api/vps-asset";
import { useAuthStore } from "../../stores/auth-store";
import { buildTerminalHtml, buildSshWsUrl } from "../../lib/terminal-html";

const TERM_BG = "#0a0a0c";
const TERM_FG = "#22c55e";
const TERM_ERR = "#ef4444";
const TERM_DIM = "#8a8a96";

type TermStatus = "connecting" | "connected" | "closed" | "error";

/** Load a bundled text asset (xterm dist files shipped as .txt). */
async function loadTextAsset(moduleId: number): Promise<string> {
  const asset = Asset.fromModule(moduleId);
  await asset.downloadAsync();
  if (!asset.localUri) throw new Error("asset has no localUri");
  return new File(asset.localUri).text();
}

/**
 * SSH terminal — a WebView running xterm.js, connected to the hub's SSH
 * relay (`/api/vps-assets/{id}/ssh`). Credentials never reach the device;
 * the hub dials the VPS server-side. Works in Expo Go (no native SSH module).
 */
export default function SSHTerminalScreen() {
  const { t } = useTranslation("vps");
  const { id } = useLocalSearchParams<{ id: string }>();
  const { data: vps, isLoading } = useVpsAssetDetail(id ?? "");
  const { token, serverUrl } = useAuthStore();
  const [html, setHtml] = useState<string | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [status, setStatus] = useState<TermStatus>("connecting");
  const [errorText, setErrorText] = useState<string | null>(null);
  // Bumping remounts the WebView → fresh WS session (reconnect).
  const [sessionKey, setSessionKey] = useState(0);

  // Landscape for a usable terminal width; restore portrait on leave.
  useEffect(() => {
    ScreenOrientation.lockAsync(ScreenOrientation.OrientationLock.LANDSCAPE);
    return () => {
      ScreenOrientation.lockAsync(ScreenOrientation.OrientationLock.PORTRAIT_UP);
    };
  }, []);

  // Compose the terminal document from bundled xterm assets (once).
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [xtermJs, fitJs, css] = await Promise.all([
          loadTextAsset(require("../../../assets/xterm/xterm.js.txt")),
          loadTextAsset(require("../../../assets/xterm/addon-fit.js.txt")),
          loadTextAsset(require("../../../assets/xterm/xterm.css.txt")),
        ]);
        if (!cancelled) setHtml(buildTerminalHtml({ xtermJs, fitJs, css }));
      } catch (err) {
        if (!cancelled) setLoadError(String(err));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const injectedConfig = useMemo(() => {
    if (!vps || !token) return null;
    const wsUrl = buildSshWsUrl(serverUrl, vps.id, token);
    return `window.__SSH_CFG__ = ${JSON.stringify({ wsUrl, fontSize: 13 })}; true;`;
  }, [vps, token, serverUrl]);

  const sshReady = !!vps?.ip && !!vps?.ssh_user;

  const statusLabel =
    status === "connecting"
      ? t("ssh_connecting")
      : status === "connected"
        ? ""
        : status === "closed"
          ? t("ssh_disconnected")
          : (errorText ?? t("ssh_error"));

  return (
    <View style={styles.container}>
      {/* Header */}
      <View style={styles.header}>
        <TouchableOpacity onPress={() => router.back()} style={styles.headerBtn}>
          <Ionicons name="close" size={20} color={TERM_FG} />
        </TouchableOpacity>
        <Text style={styles.headerTitle} numberOfLines={1}>
          {vps ? `${vps.name}  ${vps.ssh_user ?? ""}@${vps.ip ?? ""}` : ""}
        </Text>
        <Text
          style={[styles.statusText, (status === "closed" || status === "error") && styles.statusErr]}
          numberOfLines={1}
        >
          {statusLabel}
        </Text>
        {(status === "closed" || status === "error") && (
          <TouchableOpacity
            onPress={() => {
              setStatus("connecting");
              setErrorText(null);
              setSessionKey((k) => k + 1);
            }}
            style={styles.headerBtn}
          >
            <Text style={styles.reconnectText}>{t("ssh_reconnect")}</Text>
          </TouchableOpacity>
        )}
      </View>

      {/* Body */}
      {isLoading || (!html && !loadError) ? (
        <View style={styles.center}>
          <ActivityIndicator size="large" color={TERM_FG} />
        </View>
      ) : !vps || !sshReady ? (
        <View style={styles.center}>
          <Text style={styles.hintText}>{t("ssh_missing_info_message")}</Text>
        </View>
      ) : loadError ? (
        <View style={styles.center}>
          <Text style={styles.hintText}>{loadError}</Text>
        </View>
      ) : (
        <WebView
          key={sessionKey}
          style={styles.webview}
          source={{ html: html! }}
          originWhitelist={["*"]}
          javaScriptEnabled
          injectedJavaScriptBeforeContentLoaded={injectedConfig ?? "true;"}
          onMessage={(ev) => {
            try {
              const msg = JSON.parse(ev.nativeEvent.data) as {
                type: string;
                status?: TermStatus;
                message?: string;
              };
              if (msg.type === "status" && msg.status) {
                // Don't let the post-error close event mask the error state.
                setStatus((prev) => (prev === "error" && msg.status === "closed" ? prev : msg.status!));
              } else if (msg.type === "error" && msg.message) {
                setErrorText(msg.message);
                setStatus("error");
              }
            } catch {
              // Ignore non-JSON frames.
            }
          }}
          // Android: allow ws:// from the html-string page; iOS ATS already
          // permits arbitrary loads (app.json).
          mixedContentMode="always"
          keyboardDisplayRequiresUserAction={false}
          hideKeyboardAccessoryView
          bounces={false}
          setSupportMultipleWindows={false}
          allowsBackForwardNavigationGestures={false}
        />
      )}
    </View>
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
  statusText: { maxWidth: 200, fontSize: 11, color: TERM_DIM, fontFamily: "monospace" },
  statusErr: { color: TERM_ERR },
  reconnectText: { fontSize: 14, fontWeight: "700", color: TERM_FG },
  hintText: { paddingHorizontal: 32, fontSize: 13, color: TERM_DIM, textAlign: "center" },
  webview: { flex: 1, backgroundColor: TERM_BG },
});
