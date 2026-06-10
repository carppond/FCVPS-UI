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

  it("returns null for unrelated errors", () => {
    expect(classifySyncError("http get …: context deadline exceeded")).toBeNull();
    expect(classifySyncError("connection refused")).toBeNull();
    expect(classifySyncError("")).toBeNull();
    expect(classifySyncError(undefined)).toBeNull();
  });
});
