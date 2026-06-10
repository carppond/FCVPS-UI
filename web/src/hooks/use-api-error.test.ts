import { describe, expect, it } from "vitest";
import { formatApiError } from "./use-api-error";
import { ApiError } from "@/lib/api-client";

// Minimal t: resolves from a fixed table, returns the default value (or the
// key) when missing — mirroring i18next's defaultValue behaviour.
const TABLE: Record<string, string> = {
  "errors:AUTH_INVALID_PASSWORD": "密码错误",
  "errors:NOT_FOUND": "资源不存在",
  "errors:VALIDATION_FAILED": "参数校验失败",
  "errors:RATE_LIMITED": "请求过于频繁",
  "errors:INTERNAL": "服务器内部错误",
  "errors:INTERNAL_UNKNOWN": "发生未知错误",
  "errors:NET_TLS_CERT": "目标服务器 TLS 证书无效",
  "errors:NET_TIMEOUT": "连接目标服务器超时",
};
const t = (key: string, def?: string) => TABLE[key] ?? def ?? key;

describe("formatApiError", () => {
  it("strips the ERR_ prefix so locale keys resolve", () => {
    const err = new ApiError("ERR_AUTH_INVALID_PASSWORD", "invalid password", 401);
    expect(formatApiError(err, t)).toBe("密码错误");
  });

  it("falls back to the code family when the exact key is missing", () => {
    expect(
      formatApiError(new ApiError("ERR_NOT_FOUND_SUBSCRIPTION", "subscription not found", 404), t),
    ).toBe("资源不存在");
    expect(
      formatApiError(new ApiError("ERR_VALIDATION_REQUIRED_FIELD", "name required", 400), t),
    ).toBe("参数校验失败");
    expect(
      formatApiError(new ApiError("ERR_AUTH_RATE_LIMITED", "rate limited", 429), t),
    ).toBe("请求过于频繁");
  });

  it("never leaks raw network error dumps — classifies them instead", () => {
    const raw =
      'http get https://1.2.3.4:2096/sub/x: Get "https://1.2.3.4:2096/sub/x": tls: failed to verify certificate: x509: certificate has expired';
    const out = formatApiError(new ApiError("ERR_INTERNAL_UNKNOWN", raw, 500), t);
    expect(out).toBe("目标服务器 TLS 证书无效");
    expect(out).not.toContain("https://");
  });

  it("classifies timeouts", () => {
    const out = formatApiError(
      new ApiError("ERR_INTERNAL_UNKNOWN", "context deadline exceeded (Client.Timeout exceeded)", 500),
      t,
    );
    expect(out).toBe("连接目标服务器超时");
  });

  it("uses the generic message for unknown codes without leaking the message", () => {
    const out = formatApiError(new ApiError("ERR_SOMETHING_NEW", "scary internals", 500), t);
    expect(out).toBe("发生未知错误");
  });

  it("handles non-ApiError values", () => {
    expect(formatApiError(new Error("boom"), t)).toBe("发生未知错误");
    expect(formatApiError("nope", t)).toBe("发生未知错误");
  });
});
