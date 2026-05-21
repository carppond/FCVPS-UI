import { createServer, type IncomingMessage, type Server } from "node:http";
import { AddressInfo } from "node:net";

/**
 * Lightweight HTTP mock server for E2E.
 *
 * Goals:
 *  - Zero extra deps (no msw / nock). Pure Node `http` module.
 *  - Captures every request body so specs can assert on payload shape.
 *  - Per-path responders so a single instance can stand in for the various
 *    external endpoints we hit during E2E (subscription remote, webhook
 *    receiver, telegram callback, nezha-style heartbeat).
 *
 * Lifecycle:
 *
 *    const mock = await startMockServer();
 *    mock.on("POST", "/hook", (req, body) => ({ status: 200, body: "ok" }));
 *    // ... run test ...
 *    expect(mock.captured("POST", "/hook")).toHaveLength(1);
 *    await mock.stop();
 */

export interface CapturedRequest {
  method: string;
  path: string;
  headers: Record<string, string | string[] | undefined>;
  body: string;
  receivedAt: number;
}

export interface MockResponse {
  status?: number;
  headers?: Record<string, string>;
  body?: string;
}

export type Responder = (
  req: IncomingMessage,
  body: string,
) => MockResponse | Promise<MockResponse>;

export interface MockServerHandle {
  /** Base URL including scheme, host, and bound port. */
  readonly url: string;
  /** Bound port (handy if a spec needs to feed it into config). */
  readonly port: number;
  /** Register a per-method-path responder. Overrides any previous one. */
  on: (method: string, path: string, responder: Responder) => void;
  /** Set a fallback responder when no method/path match. Defaults to 404. */
  setFallback: (responder: Responder) => void;
  /** All captured requests, optionally filtered by method/path. */
  captured: (method?: string, path?: string) => CapturedRequest[];
  /** Drop every captured request — handy between sub-tests. */
  reset: () => void;
  /** Gracefully close the server. */
  stop: () => Promise<void>;
}

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    req.on("data", (c: Buffer) => chunks.push(c));
    req.on("end", () => resolve(Buffer.concat(chunks).toString("utf8")));
    req.on("error", reject);
  });
}

/**
 * Sample Clash YAML returned for `GET /subscription` by default. Two nodes
 * (one vmess, one trojan) is enough to validate parser counts in 02-subscription.
 */
export const SAMPLE_CLASH_YAML = `proxies:
  - name: "hk-vmess"
    type: vmess
    server: hk.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
    tls: true
  - name: "jp-trojan"
    type: trojan
    server: jp.example.com
    port: 443
    password: s3cret
    sni: jp.example.com
proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - hk-vmess
      - jp-trojan
rules:
  - MATCH,PROXY
`;

export async function startMockServer(
  initialPort = 0,
): Promise<MockServerHandle> {
  const responders = new Map<string, Responder>();
  const captured: CapturedRequest[] = [];
  let fallback: Responder = () => ({ status: 404, body: "not found" });

  const keyOf = (method: string, path: string) =>
    `${method.toUpperCase()} ${path}`;

  const server: Server = createServer((req, res) => {
    (async () => {
      const body = await readBody(req);
      const method = (req.method || "GET").toUpperCase();
      const url = req.url || "/";
      // Drop query string for the captured path so specs can index on the
      // route alone, but keep the raw URL on `req.url` for responders that
      // care about params.
      const pathOnly = url.split("?")[0];
      captured.push({
        method,
        path: pathOnly,
        headers: req.headers,
        body,
        receivedAt: Date.now(),
      });
      const responder = responders.get(keyOf(method, pathOnly)) || fallback;
      let result: MockResponse;
      try {
        result = await responder(req, body);
      } catch (err) {
        result = {
          status: 500,
          body: `mock responder threw: ${String(err)}`,
        };
      }
      res.writeHead(result.status ?? 200, {
        "Content-Type": "application/json",
        ...(result.headers ?? {}),
      });
      res.end(result.body ?? "");
    })().catch((err) => {
      // Last-resort error path — should not happen because readBody covers
      // the body side, but defend against unexpected dispatcher failures.
      res.writeHead(500);
      res.end(`internal mock error: ${String(err)}`);
    });
  });

  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(initialPort, "127.0.0.1", () => {
      server.removeListener("error", reject);
      resolve();
    });
  });

  const addr = server.address() as AddressInfo;
  const port = addr.port;

  // Register a small default route for the most common subscription fetch.
  responders.set(keyOf("GET", "/subscription"), () => ({
    status: 200,
    headers: { "Content-Type": "text/yaml" },
    body: SAMPLE_CLASH_YAML,
  }));

  return {
    url: `http://127.0.0.1:${port}`,
    port,
    on(method, path, responder) {
      responders.set(keyOf(method, path), responder);
    },
    setFallback(responder) {
      fallback = responder;
    },
    captured(method, path) {
      return captured.filter((c) => {
        if (method && c.method !== method.toUpperCase()) return false;
        if (path && c.path !== path) return false;
        return true;
      });
    },
    reset() {
      captured.length = 0;
    },
    async stop() {
      await new Promise<void>((resolve, reject) =>
        server.close((err) => (err ? reject(err) : resolve())),
      );
    },
  };
}
