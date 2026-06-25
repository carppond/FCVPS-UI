import { describe, expect, it } from "vitest";
import { parseShortLinkTarget, displayShortUrl } from "./shortlink-target";

describe("parseShortLinkTarget", () => {
  it("parses a subscription share URL with token and target", () => {
    expect(
      parseShortLinkTarget(
        "https://hub.example.com/download/my-sub?token=abc123&target=clash",
      ),
    ).toEqual({ subscriptionName: "my-sub", client: "clash" });
  });

  it("decodes URL-encoded subscription names (CJK / emoji)", () => {
    expect(
      parseShortLinkTarget(
        "https://hub.example.com/download/%E6%9C%BA%E5%9C%BA%E8%AE%A2%E9%98%85?token=t",
      ),
    ).toEqual({ subscriptionName: "机场订阅", client: undefined });
  });

  it("tolerates a silent-mode prefix in the path", () => {
    expect(
      parseShortLinkTarget(
        "https://hub.example.com/_app/0123456789abcdef0123456789abcdef/download/sub?token=t&target=singbox",
      ),
    ).toEqual({ subscriptionName: "sub", client: "singbox" });
  });

  it("returns null for non-subscription URLs", () => {
    expect(parseShortLinkTarget("https://example.com/very/long/url")).toBeNull();
    expect(parseShortLinkTarget("")).toBeNull();
  });

  it("keeps the raw segment when percent-decoding fails", () => {
    expect(
      parseShortLinkTarget("https://h.example.com/download/bad%zz?token=t"),
    ).toEqual({ subscriptionName: "bad%zz", client: undefined });
  });
});

describe("displayShortUrl", () => {
  it("re-roots a short URL onto the current browser origin (keeps port)", () => {
    const out = displayShortUrl("https://vpn.example.com/s/11");
    expect(out).toBe(`${window.location.origin}/s/11`);
  });
  it("handles a host that already has a port in the source", () => {
    const out = displayShortUrl("https://vpn.example.com:9999/s/abcXYZ");
    expect(out).toBe(`${window.location.origin}/s/abcXYZ`);
  });
  it("returns input unchanged when it has no http(s) host prefix", () => {
    expect(displayShortUrl("/s/keep")).toBe(`${window.location.origin}/s/keep`);
  });
});
