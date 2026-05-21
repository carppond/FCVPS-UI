/**
 * Unit tests for silent-mode URL prefix extraction.
 *
 * The hub auto-generates a 32-hex `silent_mode_prefix` at boot and requires
 * every non-whitelisted request to enter via `/_app/<prefix>/...`. The SPA
 * picks the prefix out of `window.location.pathname` on boot, caches it in
 * localStorage, and rewrites the URL to a canonical route so TanStack Router
 * can match it.
 *
 * Regression target: pre-fix, NO frontend code ever called setPrefix(), so
 * users had to manually delete `system_settings.silent_mode_prefix` on every
 * startup. These tests lock the extraction down so that bug can't return.
 */
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  extractSilentPrefixFromURL,
  getPrefix,
  parseSilentPrefix,
  setPrefix,
} from "@/lib/silent-prefix";

const HEX32_A = "a".repeat(32);
const HEX32_MIXED = "0123456789abcdef0123456789abcdef";

describe("parseSilentPrefix", () => {
  it("extracts a 32-hex prefix followed by a route", () => {
    expect(parseSilentPrefix(`/_app/${HEX32_A}/login`)).toEqual({
      prefix: `/_app/${HEX32_A}`,
      strippedPath: "/login",
    });
  });

  it("extracts when the path is exactly /_app/<prefix> (no trailing)", () => {
    expect(parseSilentPrefix(`/_app/${HEX32_MIXED}`)).toEqual({
      prefix: `/_app/${HEX32_MIXED}`,
      strippedPath: "/",
    });
  });

  it("extracts and preserves nested path segments", () => {
    expect(parseSilentPrefix(`/_app/${HEX32_A}/nodes/123/edit`)).toEqual({
      prefix: `/_app/${HEX32_A}`,
      strippedPath: "/nodes/123/edit",
    });
  });

  it("returns null when prefix is shorter than 32 hex chars", () => {
    expect(parseSilentPrefix("/_app/short/login")).toBeNull();
  });

  it("returns null when prefix contains non-hex chars", () => {
    expect(
      parseSilentPrefix(`/_app/${"g".repeat(32)}/login`),
    ).toBeNull();
  });

  it("returns null when /_app prefix is absent", () => {
    expect(parseSilentPrefix("/login")).toBeNull();
    expect(parseSilentPrefix("/")).toBeNull();
    expect(parseSilentPrefix("/dashboard")).toBeNull();
  });

  it("returns null when the prefix segment is too long (33+ hex)", () => {
    // The regex anchors the next char as either '/' or end-of-string, so a
    // 33-hex chunk should not be misread as a 32-hex prefix with junk.
    expect(parseSilentPrefix(`/_app/${"a".repeat(33)}/login`)).toBeNull();
  });
});

describe("extractSilentPrefixFromURL", () => {
  beforeEach(() => {
    localStorage.clear();
    window.history.replaceState(null, "", "/");
  });

  afterEach(() => {
    localStorage.clear();
    window.history.replaceState(null, "", "/");
  });

  it("persists prefix and rewrites URL when entry is /_app/<hex>/login", () => {
    window.history.replaceState(null, "", `/_app/${HEX32_A}/login?foo=1#x`);

    const extracted = extractSilentPrefixFromURL();

    expect(extracted).toBe(true);
    expect(getPrefix()).toBe(`/_app/${HEX32_A}`);
    expect(window.location.pathname).toBe("/login");
    expect(window.location.search).toBe("?foo=1");
    expect(window.location.hash).toBe("#x");
  });

  it("rewrites bare /_app/<hex> to /", () => {
    window.history.replaceState(null, "", `/_app/${HEX32_MIXED}`);

    const extracted = extractSilentPrefixFromURL();

    expect(extracted).toBe(true);
    expect(getPrefix()).toBe(`/_app/${HEX32_MIXED}`);
    expect(window.location.pathname).toBe("/");
  });

  it("does nothing when no prefix is present in the URL", () => {
    window.history.replaceState(null, "", "/login");
    setPrefix(""); // start clean

    const extracted = extractSilentPrefixFromURL();

    expect(extracted).toBe(false);
    expect(getPrefix()).toBe("");
    expect(window.location.pathname).toBe("/login");
  });

  it("does nothing when the prefix segment is malformed (too short)", () => {
    window.history.replaceState(null, "", "/_app/short/login");

    const extracted = extractSilentPrefixFromURL();

    expect(extracted).toBe(false);
    expect(getPrefix()).toBe("");
    expect(window.location.pathname).toBe("/_app/short/login");
  });

  it("does not overwrite localStorage when the same prefix is re-extracted", () => {
    const prefix = `/_app/${HEX32_A}`;
    setPrefix(prefix);
    window.history.replaceState(null, "", `${prefix}/dashboard`);

    const extracted = extractSilentPrefixFromURL();

    expect(extracted).toBe(true);
    expect(getPrefix()).toBe(prefix);
    expect(window.location.pathname).toBe("/dashboard");
  });
});
