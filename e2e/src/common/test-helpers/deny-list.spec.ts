import { describe, expect, it, jest } from "@jest/globals";
import { mkdtempSync, readFileSync, writeFileSync } from "fs";
import { tmpdir } from "os";
import { join } from "path";

import { addToDenyList, removeFromDenyList, withDenyListAddresses } from "./deny-list";

type MockSequencerClient = {
  pluginsReloadPluginConfig: ReturnType<typeof jest.fn<() => Promise<unknown>>>;
};

function createMockSequencerClient(): MockSequencerClient {
  return {
    pluginsReloadPluginConfig: jest.fn(async (): Promise<unknown> => undefined),
  };
}

function createDeferred<T = void>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });

  return { promise, resolve };
}

function createTempDenyListFile(initialContent = ""): string {
  const directory = mkdtempSync(join(tmpdir(), "deny-list-test-"));
  const denyListPath = join(directory, "deny-list.txt");
  writeFileSync(denyListPath, initialContent);
  return denyListPath;
}

describe("deny-list helper", () => {
  it("should append lowercase addresses from the in-memory state", async () => {
    // Arrange
    const denyListPath = createTempDenyListFile("0xexisting\n");
    const client = createMockSequencerClient();

    // Act
    await addToDenyList(client, ["0xAbC123", "0xDEF456"], denyListPath);

    // Assert
    expect(readFileSync(denyListPath, "utf-8")).toEqual("0xexisting\n0xabc123\n0xdef456\n");
    expect(client.pluginsReloadPluginConfig).toHaveBeenCalledTimes(2);
  });

  it("should preserve existing file entries without requiring a trailing newline", async () => {
    // Arrange
    const denyListPath = createTempDenyListFile("0xexisting");
    const client = createMockSequencerClient();

    // Act
    await addToDenyList(client, ["0xAbC123"], denyListPath);

    // Assert
    expect(readFileSync(denyListPath, "utf-8")).toEqual("0xexisting\n0xabc123\n");
    expect(client.pluginsReloadPluginConfig).toHaveBeenCalledTimes(2);
  });

  it("should remove only dynamically added target addresses case-insensitively", async () => {
    // Arrange
    const denyListPath = createTempDenyListFile("0xkeepme\n");
    const client = createMockSequencerClient();
    await addToDenyList(client, ["0xAbC123", "0xDEF456"], denyListPath);

    // Act
    await removeFromDenyList(client, ["0xAbC123"], denyListPath);

    // Assert
    expect(readFileSync(denyListPath, "utf-8")).toEqual("0xkeepme\n0xdef456\n");
    expect(client.pluginsReloadPluginConfig).toHaveBeenCalledTimes(4);
  });

  it("should reload before and after callback and restore deny-list content", async () => {
    // Arrange
    const denyListPath = createTempDenyListFile();
    const client = createMockSequencerClient();
    const run = jest.fn(async () => {
      expect(readFileSync(denyListPath, "utf-8")).toEqual("0xabc123\n");
    });

    // Act
    await withDenyListAddresses(client, ["0xabc123"], run, denyListPath);

    // Assert
    expect(run).toHaveBeenCalledTimes(1);
    expect(client.pluginsReloadPluginConfig).toHaveBeenCalledTimes(4);
    expect(readFileSync(denyListPath, "utf-8")).toEqual("");
  });

  it("should clean up and reload even when callback throws", async () => {
    // Arrange
    const denyListPath = createTempDenyListFile();
    const client = createMockSequencerClient();
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
    expect(client.pluginsReloadPluginConfig).toHaveBeenCalledTimes(4);
    expect(readFileSync(denyListPath, "utf-8")).toEqual("");
  });

  it("should serialize deny-list sessions until the first callback completes", async () => {
    // Arrange
    const denyListPath = createTempDenyListFile();
    const client = createMockSequencerClient();
    const firstRunEntered = createDeferred();
    const releaseFirstRun = createDeferred();
    const secondRunEntered = createDeferred();
    const releaseSecondRun = createDeferred();
    const events: string[] = [];

    const first = withDenyListAddresses(
      client,
      ["0xabc123"],
      async () => {
        events.push("first:start");
        expect(readFileSync(denyListPath, "utf-8")).toEqual("0xabc123\n");
        firstRunEntered.resolve();
        await releaseFirstRun.promise;
        events.push("first:end");
      },
      denyListPath,
    );

    await firstRunEntered.promise;

    const second = withDenyListAddresses(
      client,
      ["0xdef456"],
      async () => {
        events.push("second:start");
        expect(readFileSync(denyListPath, "utf-8")).toEqual("0xdef456\n");
        secondRunEntered.resolve();
        await releaseSecondRun.promise;
        events.push("second:end");
      },
      denyListPath,
    );

    await Promise.resolve();
    await Promise.resolve();

    // Assert the second session cannot start until the first callback exits.
    expect(events).toEqual(["first:start"]);
    expect(readFileSync(denyListPath, "utf-8")).toEqual("0xabc123\n");

    // Act
    releaseFirstRun.resolve();
    await first;
    await secondRunEntered.promise;

    // Assert the second session begins only after the first session fully cleans up.
    expect(events).toEqual(["first:start", "first:end", "second:start"]);
    expect(readFileSync(denyListPath, "utf-8")).toEqual("0xdef456\n");

    // Act
    releaseSecondRun.resolve();
    await second;

    // Assert
    expect(events).toEqual(["first:start", "first:end", "second:start", "second:end"]);
    expect(client.pluginsReloadPluginConfig).toHaveBeenCalledTimes(8);
    expect(readFileSync(denyListPath, "utf-8")).toEqual("");
  });

  it("should block standalone add mutations while a deny-list session callback is active", async () => {
    // Arrange
    const denyListPath = createTempDenyListFile();
    const client = createMockSequencerClient();
    const runEntered = createDeferred();
    const releaseRun = createDeferred();
    let addCompleted = false;

    const session = withDenyListAddresses(
      client,
      ["0xabc123"],
      async () => {
        runEntered.resolve();
        await releaseRun.promise;
      },
      denyListPath,
    );

    await runEntered.promise;

    const pendingAdd = addToDenyList(client, ["0xdef456"], denyListPath).then(() => {
      addCompleted = true;
    });

    await Promise.resolve();
    await Promise.resolve();

    // Assert the standalone mutation is queued behind the active deny-list session.
    expect(addCompleted).toBe(false);
    expect(readFileSync(denyListPath, "utf-8")).toEqual("0xabc123\n");

    // Act
    releaseRun.resolve();
    await session;
    await pendingAdd;

    // Assert
    expect(addCompleted).toBe(true);
    expect(client.pluginsReloadPluginConfig).toHaveBeenCalledTimes(6);
    expect(readFileSync(denyListPath, "utf-8")).toEqual("0xdef456\n");
  });
});
