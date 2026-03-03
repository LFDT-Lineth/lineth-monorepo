import { describe, expect, it, jest } from "@jest/globals";
import { mkdtempSync, readFileSync, writeFileSync } from "fs";
import { tmpdir } from "os";
import { join } from "path";

import { addToDenyList, removeFromDenyList, withDenyListAddresses } from "./deny-list";

type MockSequencerClient = {
  pluginsReloadPluginConfig: ReturnType<typeof jest.fn<() => Promise<unknown>>>;
};

function createTempDenyListFile(initialContent = ""): string {
  const directory = mkdtempSync(join(tmpdir(), "deny-list-test-"));
  const denyListPath = join(directory, "deny-list.txt");
  writeFileSync(denyListPath, initialContent);
  return denyListPath;
}

describe("deny-list helper", () => {
  it("should append lowercase addresses", () => {
    // Arrange
    const denyListPath = createTempDenyListFile("0xexisting\n");

    // Act
    addToDenyList(["0xAbC123", "0xDEF456"], denyListPath);

    // Assert
    expect(readFileSync(denyListPath, "utf-8")).toEqual("0xexisting\n0xabc123\n0xdef456\n");
  });

  it("should insert a separator newline when existing file has no trailing newline", () => {
    // Arrange
    const denyListPath = createTempDenyListFile("0xexisting");

    // Act
    addToDenyList(["0xAbC123"], denyListPath);

    // Assert
    expect(readFileSync(denyListPath, "utf-8")).toEqual("0xexisting\n0xabc123\n");
  });

  it("should remove only target addresses case-insensitively", () => {
    // Arrange
    const denyListPath = createTempDenyListFile("0xabc123\n0xkeepme\n0xDEF456\n");

    // Act
    removeFromDenyList(["0xAbC123"], denyListPath);

    // Assert
    expect(readFileSync(denyListPath, "utf-8")).toEqual("0xkeepme\n0xDEF456\n");
  });

  it("should reload before and after callback and restore deny-list content", async () => {
    // Arrange
    const denyListPath = createTempDenyListFile();
    const client: MockSequencerClient = {
      pluginsReloadPluginConfig: jest.fn().mockResolvedValue(undefined),
    };
    const run = jest.fn(async () => {
      expect(readFileSync(denyListPath, "utf-8")).toEqual("0xabc123\n");
    });

    // Act
    await withDenyListAddresses(client, ["0xabc123"], run, denyListPath);

    // Assert
    expect(run).toHaveBeenCalledTimes(1);
    expect(client.pluginsReloadPluginConfig).toHaveBeenCalledTimes(2);
    expect(readFileSync(denyListPath, "utf-8")).toEqual("");
  });

  it("should clean up and reload even when callback throws", async () => {
    // Arrange
    const denyListPath = createTempDenyListFile();
    const client: MockSequencerClient = {
      pluginsReloadPluginConfig: jest.fn().mockResolvedValue(undefined),
    };
    const runError = new Error("callback failed");

    // Act
    await expect(
      withDenyListAddresses(
        client,
        ["0xabc123"],
        async () => {
          throw runError;
        },
        denyListPath,
      ),
    ).rejects.toThrow("callback failed");

    // Assert
    expect(client.pluginsReloadPluginConfig).toHaveBeenCalledTimes(2);
    expect(readFileSync(denyListPath, "utf-8")).toEqual("");
  });
});
