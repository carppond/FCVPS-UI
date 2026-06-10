import { describe, expect, it } from "vitest";
import { classifySyncError } from "./sync-error";

describe("classifySyncError", () => {
  it("detects expired certificates", () => {
    expect(
      classifySyncError(
        'http get https://1.2.3.4:2096/sub/x: Get "https://1.2.3.4:2096/sub/x": tls: failed to verify certificate: x509: certificate has expired or is not yet valid: current time 2026-06-09T23:53:35-07:00 is after 2026-06-09T02:27:41Z',
      ),
    ).toBe("tls_cert");
  });

  it("detects self-signed certificates", () => {
    expect(
      classifySyncError("x509: certificate signed by unknown authority"),
    ).toBe("tls_cert");
  });

  it("detects SAN mismatches", () => {
    expect(
      classifySyncError(
        "x509: cannot validate certificate for 1.2.3.4 because it doesn't contain any IP SANs",
      ),
    ).toBe("tls_cert");
  });

  it("detects timeouts / refused / dns failures", () => {
    expect(classifySyncError("context deadline exceeded (Client.Timeout exceeded)")).toBe("timeout");
    expect(classifySyncError("read tcp 10.0.0.1:443: i/o timeout")).toBe("timeout");
    expect(classifySyncError("dial tcp 1.2.3.4:443: connect: connection refused")).toBe("refused");
    expect(classifySyncError("lookup sub.example.com: no such host")).toBe("dns");
  });

  it("returns null for unrelated errors", () => {
    expect(classifySyncError("yaml: unmarshal error at line 3")).toBeNull();
    expect(classifySyncError("")).toBeNull();
    expect(classifySyncError(undefined)).toBeNull();
  });
});
