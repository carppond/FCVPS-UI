/**
 * Builds the self-contained HTML document the SSH terminal WebView renders.
 *
 * The xterm.js runtime (JS + CSS + fit addon) is bundled as app assets
 * (assets/xterm/*.txt) and inlined here, so the terminal works without any
 * network access beyond the WebSocket to the hub itself — no CDN.
 *
 * Connection config is NOT baked into the HTML: the native side injects
 * `window.__SSH_CFG__ = {wsUrl, fontSize}` via
 * `injectedJavaScriptBeforeContentLoaded`, keeping the token out of the
 * document string.
 *
 * Bridge protocol to React Native (window.ReactNativeWebView.postMessage):
 *   {type:"status", status:"connected"|"closed"|"error"}
 *   {type:"error",  message:string}   — relay error text from the server
 */

interface TerminalAssets {
  xtermJs: string;
  fitJs: string;
  css: string;
}

export function buildTerminalHtml({ xtermJs, fitJs, css }: TerminalAssets): string {
  return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no, viewport-fit=cover" />
<style>
${css}
html, body { margin:0; padding:0; height:100%; background:#0a0a0b; }
#term { position:absolute; inset:0; padding:4px; }
</style>
</head>
<body>
<div id="term"></div>
<script>${xtermJs}</script>
<script>${fitJs}</script>
<script>
(function () {
  var cfg = window.__SSH_CFG__ || {};
  var post = function (o) {
    if (window.ReactNativeWebView) {
      window.ReactNativeWebView.postMessage(JSON.stringify(o));
    }
  };
  var term = new Terminal({
    cursorBlink: true,
    fontSize: cfg.fontSize || 13,
    fontFamily: "Menlo, Monaco, monospace",
    theme: {
      background: "#0a0a0b",
      foreground: "#e4e6eb",
      cursor: "#ff6363",
      selectionBackground: "rgba(255,99,99,0.25)"
    }
  });
  var fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.open(document.getElementById("term"));
  fit.fit();

  var ws = new WebSocket(cfg.wsUrl);
  ws.binaryType = "arraybuffer";
  var enc = new TextEncoder();
  var sendResize = function () {
    if (ws.readyState === 1) {
      ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
    }
  };
  ws.onopen = function () {
    post({ type: "status", status: "connected" });
    sendResize();
    term.focus();
  };
  ws.onmessage = function (ev) {
    if (typeof ev.data === "string") {
      post({ type: "error", message: ev.data });
      term.write("\\r\\n\\x1b[31m" + ev.data + "\\x1b[0m");
      return;
    }
    term.write(new Uint8Array(ev.data));
  };
  ws.onclose = function () { post({ type: "status", status: "closed" }); };
  ws.onerror = function () { post({ type: "status", status: "error" }); };
  term.onData(function (d) {
    if (ws.readyState === 1) ws.send(enc.encode(d));
  });
  var refit = function () { fit.fit(); sendResize(); };
  window.addEventListener("resize", refit);
  if (window.visualViewport) {
    window.visualViewport.addEventListener("resize", refit);
  }
})();
</script>
</body>
</html>`;
}

/** ws(s):// endpoint for the hub-relayed SSH session. */
export function buildSshWsUrl(serverUrl: string, assetId: string, token: string): string {
  const wsBase = serverUrl.replace(/^http/, "ws").replace(/\/+$/, "");
  return `${wsBase}/api/vps-assets/${assetId}/ssh?token=${encodeURIComponent(token)}`;
}
