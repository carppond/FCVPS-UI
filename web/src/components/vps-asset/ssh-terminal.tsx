import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/stores/auth-store";
import { prefixedPath } from "@/lib/silent-prefix";
import type { VpsAsset } from "@/types/api";

type TermStatus = "connecting" | "connected" | "closed" | "error";

/** Resolve a CSS custom property so the xterm theme follows the app tokens. */
function cssVar(name: string, fallback: string): string {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}

function buildWsUrl(assetId: string, token: string): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  const path = prefixedPath(`/api/vps-assets/${assetId}/ssh`);
  return `${proto}//${window.location.host}${path}?token=${encodeURIComponent(token)}`;
}

/**
 * SshTerminalDialog — interactive SSH shell to a VPS asset, relayed through
 * the hub (`GET /api/vps-assets/{id}/ssh` WebSocket). Credentials stay
 * server-side; this component only moves keystrokes and PTY bytes.
 *
 * Wire protocol: binary frames = raw bytes both ways; client text frames =
 * {"type":"resize",cols,rows}; server text frames = human-readable error.
 */
export function SshTerminalDialog({
  vps,
  onClose,
}: {
  vps: VpsAsset | null;
  onClose: () => void;
}) {
  const { t } = useTranslation(["vps-asset"]);
  const token = useAuthStore((s) => s.token);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [status, setStatus] = useState<TermStatus>("connecting");
  const [errorText, setErrorText] = useState<string | null>(null);
  // Bumping this key tears down and recreates the whole session (reconnect).
  const [sessionKey, setSessionKey] = useState(0);

  const open = vps !== null;

  useEffect(() => {
    if (!open || !vps || !token || !containerRef.current) return;

    setStatus("connecting");
    setErrorText(null);

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily:
        "ui-monospace, SFMono-Regular, Menlo, Monaco, 'Cascadia Mono', monospace",
      theme: {
        background: cssVar("--color-neutral-50", "#0a0a0b"),
        foreground: cssVar("--color-neutral-900", "#e4e6eb"),
        cursor: cssVar("--color-primary", "#ff6363"),
        selectionBackground: cssVar("--color-primary-soft", "rgba(255,99,99,0.18)"),
      },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(containerRef.current);
    fit.fit();

    const ws = new WebSocket(buildWsUrl(vps.id, token));
    ws.binaryType = "arraybuffer";
    wsRef.current = ws;
    const encoder = new TextEncoder();

    const sendResize = () => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
      }
    };

    ws.onopen = () => {
      setStatus("connected");
      sendResize();
      term.focus();
    };
    ws.onmessage = (ev) => {
      if (typeof ev.data === "string") {
        // Server-side relay error (e.g. auth/dial failure) — render + flag.
        setErrorText(ev.data);
        term.writeln(`\r\n\x1b[31m${ev.data}\x1b[0m`);
        setStatus("error");
        return;
      }
      term.write(new Uint8Array(ev.data as ArrayBuffer));
    };
    ws.onclose = () => {
      setStatus((prev) => (prev === "error" ? prev : "closed"));
    };
    ws.onerror = () => {
      setStatus("error");
    };

    const dataSub = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(encoder.encode(data));
      }
    });

    const ro = new ResizeObserver(() => {
      fit.fit();
      sendResize();
    });
    ro.observe(containerRef.current);

    return () => {
      ro.disconnect();
      dataSub.dispose();
      ws.close();
      wsRef.current = null;
      term.dispose();
    };
  }, [open, vps, token, sessionKey]);

  const handleOpenChange = useCallback(
    (next: boolean) => {
      if (!next) onClose();
    },
    [onClose],
  );

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="flex h-[80vh] max-w-5xl flex-col gap-3 p-4">
        <DialogHeader className="flex-row items-center justify-between space-y-0">
          <DialogTitle className="flex items-center gap-2 font-mono text-sm">
            <StatusDot status={status} />
            {vps ? `${vps.ssh_user ?? ""}@${vps.ip ?? ""}` : ""}
            <span className="text-[var(--color-text-tertiary)]">
              {status === "connecting" && t("vps-asset:terminal.connecting")}
              {status === "closed" && t("vps-asset:terminal.closed")}
              {status === "error" && (errorText ?? t("vps-asset:terminal.error"))}
            </span>
          </DialogTitle>
          {(status === "closed" || status === "error") && (
            <Button
              size="sm"
              variant="outline"
              className="mr-6"
              onClick={() => setSessionKey((k) => k + 1)}
            >
              {t("vps-asset:terminal.reconnect")}
            </Button>
          )}
        </DialogHeader>
        <div
          ref={containerRef}
          className="min-h-0 flex-1 overflow-hidden rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-neutral-50)] p-2"
        />
      </DialogContent>
    </Dialog>
  );
}

function StatusDot({ status }: { status: TermStatus }) {
  const color =
    status === "connected"
      ? "bg-[var(--color-success)]"
      : status === "connecting"
        ? "bg-[var(--color-warning)]"
        : "bg-[var(--color-error)]";
  return <span className={`inline-block h-2 w-2 rounded-full ${color}`} />;
}
