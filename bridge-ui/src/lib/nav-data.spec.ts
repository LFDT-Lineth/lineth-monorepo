import { test } from "@playwright/test";

import { type LinkBlock } from "@/types";

import { formatNavData } from "./nav-data";

const { describe, expect } = test;

function createLinkBlock(overrides: Partial<LinkBlock> = {}): LinkBlock {
  return {
    __id: "test-id",
    name: "Test",
    label: "Test",
    ...overrides,
  };
}

describe("formatNavData", () => {
  test("rewrites relative urls to linea.build", () => {
    const data = [createLinkBlock({ url: "/developers" })];

    const result = formatNavData(data);

    expect(result[0]?.url).toBe("https://linea.build/developers");
  });

  test("preserves safe absolute http and https urls", () => {
    const data = [
      createLinkBlock({ __id: "http-id", url: "http://example.com" }),
      createLinkBlock({ __id: "https-id", url: "https://example.com" }),
    ];

    const result = formatNavData(data);

    expect(result[0]?.url).toBe("http://example.com/");
    expect(result[1]?.url).toBe("https://example.com/");
  });

  test("drops unsafe urls recursively", () => {
    const data = [
      createLinkBlock({
        url: "javascript:alert(1)",
        submenusLeft: [createLinkBlock({ __id: "child-id", url: "data:text/html,<script>alert(1)</script>" })],
      }),
    ];

    const result = formatNavData(data);

    expect(result[0]?.url).toBeUndefined();
    expect(result[0]?.submenusLeft?.[0]?.url).toBeUndefined();
  });
});
