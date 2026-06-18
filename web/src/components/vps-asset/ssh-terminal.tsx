import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useAuthStore } from "@/stores/auth-store";
import { useUpdateVpsAssetMutation } from "@/api/vps-asset";
import { useApiError } from "@/hooks/use-api-error";
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
  // Keep the latest t in a ref so the WS effect can read it without listing t
  // as a dependency (which could rebuild the socket on every render).
  const tRef = useRef(t);
  tRef.current = t;
  const token = useAuthStore((s) => s.token);
  // Callback ref → state: the terminal <div> lives inside Radix's portal and
  // is NOT yet committed when a deps-on-[open] effect first runs, so a plain
  // useRef would read null and the effect would bail forever (terminal stuck
  // on "connecting", no WebSocket ever opened). Tracking the node in state
  // re-runs the effect the moment the container actually mounts.
  const [container, setContainer] = useState<HTMLDivElement | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [status, setStatus] = useState<TermStatus>("connecting");
  const [errorText, setErrorText] = useState<string | null>(null);
  // Bumping this key tears down and recreates the whole session (reconnect).
  const [sessionKey, setSessionKey] = useState(0);

  const open = vps !== null;

  // Credential gate: if the asset has neither a stored password nor a private
  // key, the relay can't authenticate. Rather than fail, prompt for a password
  // inline, save it to the asset, then connect.
  const hasStoredCreds = !!(vps?.ssh_password || vps?.ssh_private_key);
  const [credsSaved, setCredsSaved] = useState(false);
  const credsReady = hasStoredCreds || credsSaved;
  const [pwInput, setPwInput] = useState("");
  const updateAsset = useUpdateVpsAssetMutation();
  const { handle: handleError } = useApiError();

  // Reset the inline-credential state whenever the dialog targets a new asset.
  useEffect(() => {
    setCredsSaved(false);
    setPwInput("");
  }, [vps?.id]);

  const saveCredentials = async () => {
    if (!vps || !pwInput) return;
    try {
      await updateAsset.mutateAsync({ id: vps.id, data: { ssh_password: pwInput } });
      setPwInput("");
      setCredsSaved(true); // → connect effect fires once the terminal mounts
    } catch (err) {
      handleError(err);
    }
  };

  useEffect(() => {
    if (!open || !vps || !container || !credsReady) return;
    if (!token) {
      setErrorText(tRef.current("vps-asset:terminal.no_token"));
      setStatus("error");
      return;
    }

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
    term.open(container);
    fit.fit();

    const ws = new WebSocket(buildWsUrl(vps.id, token));
    ws.binaryType = "arraybuffer";
    wsRef.current = ws;
    const encoder = new TextEncoder();

    // Watchdog: a WebSocket that never completes its upgrade stays in
    // CONNECTING with no onerror/onclose, leaving the UI stuck on "connecting"
    // forever. Surface an actionable error if onopen hasn't fired in time.
    let opened = false;
    const connectTimer = window.setTimeout(() => {
      if (opened) return;
      const msg = tRef.current("vps-asset:terminal.timeout");
      setErrorText(msg);
      term.writeln(`\r\n\x1b[31m${msg}\x1b[0m`);
      setStatus("error");
      ws.close();
    }, 15000);

    const sendResize = () => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
      }
    };

    ws.onopen = () => {
      opened = true;
      window.clearTimeout(connectTimer);
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
    ro.observe(container);

    return () => {
      window.clearTimeout(connectTimer);
      ro.disconnect();
      dataSub.dispose();
      ws.close();
      wsRef.current = null;
      term.dispose();
    };
  }, [open, vps, token, sessionKey, container, credsReady]);

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
            <StatusDot status={credsReady ? status : "connecting"} />
            {vps ? `${vps.ssh_user ?? ""}@${vps.ip ?? ""}` : ""}
            {credsReady && (
              <span className="text-[var(--color-text-tertiary)]">
                {status === "connecting" && t("vps-asset:terminal.connecting")}
                {status === "closed" && t("vps-asset:terminal.closed")}
                {status === "error" && (errorText ?? t("vps-asset:terminal.error"))}
              </span>
            )}
          </DialogTitle>
          {credsReady && (status === "closed" || status === "error") && (
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
        <DialogDescription className="sr-only">
          {t("vps-asset:terminal.aria_description")}
        </DialogDescription>
        {credsReady ? (
          <div
            ref={setContainer}
            className="min-h-0 flex-1 overflow-hidden rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-neutral-50)] p-2"
          />
        ) : (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void saveCredentials();
            }}
            className="flex min-h-0 flex-1 flex-col items-center justify-center gap-4 p-6"
          >
            <div className="max-w-md text-center">
              <p className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
                {t("vps-asset:terminal.no_creds_title")}
              </p>
              <p className="mt-1 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                {t("vps-asset:terminal.no_creds_desc")}
              </p>
            </div>
            <div className="flex w-full max-w-sm flex-col gap-2">
              <Input
                type="password"
                value={pwInput}
                onChange={(e) => setPwInput(e.target.value)}
                placeholder={t("vps-asset:form.ssh_password")}
                autoComplete="new-password"
                autoFocus
              />
              <Button type="submit" disabled={!pwInput || updateAsset.isPending}>
                {updateAsset.isPending
                  ? t("vps-asset:terminal.saving")
                  : t("vps-asset:terminal.save_and_connect")}
              </Button>
            </div>
          </form>
        )}
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
