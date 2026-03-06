import { test } from "@playwright/test";

import { buildExplorerUrl, sanitizeAbsoluteHttpUrl } from "./urls";

const { describe, expect } = test;

describe("sanitizeAbsoluteHttpUrl", () => {
  test("preserves safe http and https urls", () => {
    expect(sanitizeAbsoluteHttpUrl("https://lineascan.build")).toBe("https://lineascan.build/");
    expect(sanitizeAbsoluteHttpUrl("http://example.com")).toBe("http://example.com/");
  });

  test("drops unsafe or invalid urls", () => {
    expect(sanitizeAbsoluteHttpUrl("javascript:alert(1)")).toBeUndefined();
    expect(sanitizeAbsoluteHttpUrl("data:text/html,<script>alert(1)</script>")).toBeUndefined();
    expect(sanitizeAbsoluteHttpUrl("not-a-url")).toBeUndefined();
  });
});

describe("buildExplorerUrl", () => {
  test("builds safe explorer urls for addresses and tx hashes", () => {
    expect(buildExplorerUrl("https://lineascan.build", "address", "0xabc")).toBe(
      "https://lineascan.build/address/0xabc",
    );
    expect(buildExplorerUrl("https://etherscan.io/foo?bar=baz", "tx", "0xdef")).toBe("https://etherscan.io/tx/0xdef");
  });

  test("returns undefined when the base url is unsafe", () => {
    expect(buildExplorerUrl("javascript:alert(1)", "address", "0xabc")).toBeUndefined();
  });

  test("encodes untrusted path segments", () => {
    expect(buildExplorerUrl("https://lineascan.build", "tx", "0xabc/#frag")).toBe(
      "https://lineascan.build/tx/0xabc%2F%23frag",
    );
  });
});
