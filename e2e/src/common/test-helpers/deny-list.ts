import { closeSync, openSync, readFileSync, unlinkSync, writeFileSync } from "fs";
import { resolve } from "path";

import { SequencerPluginName } from "../../config/clients/linea-rpc/sequencer-plugins";
import { toLowercaseLines } from "../utils/string";

import type { PluginsReloadPluginConfigParameters } from "../../config/clients/linea-rpc/plugins-reload-plugin-config";

const DEFAULT_DENY_LIST_PATH = resolve(__dirname, "../../../..", "docker/config/linea-besu-sequencer/deny-list.txt");
const NEWLINE = "\n";
const FILE_LOCK_RETRY_MS = 25;
const FILE_LOCK_TIMEOUT_MS = 30_000;

let denyListOperationQueue: Promise<void> = Promise.resolve();

type DenyListControlClient = {
  pluginsReloadPluginConfig: (args: PluginsReloadPluginConfigParameters) => Promise<unknown>;
};

function withDenyListLock<T>(operation: () => Promise<T>): Promise<T> {
  const result = denyListOperationQueue.then(operation, operation);
  denyListOperationQueue = result.then(
    () => undefined,
    () => undefined,
  );
  return result;
}

function normalizeAddresses(addresses: readonly string[]): string[] {
  return [...new Set(toLowercaseLines(addresses).filter(Boolean))];
}

// Cross-worker file lock using O_EXCL: openSync('wx') atomically creates the lockfile or
// fails if it exists. Coordinates the read-modify-write of the deny-list across all Jest
// workers (and any other process that respects the same lockfile).
async function acquireFileLock(denyListPath: string): Promise<() => void> {
  const lockfile = `${denyListPath}.lock`;
  const start = Date.now();
  // eslint-disable-next-line no-constant-condition
  while (true) {
    try {
      const fd = openSync(lockfile, "wx");
      closeSync(fd);
      return () => {
        try {
          unlinkSync(lockfile);
        } catch {
          // already removed
        }
      };
    } catch {
      if (Date.now() - start > FILE_LOCK_TIMEOUT_MS) {
        throw new Error(`acquireFileLock: timed out waiting for ${lockfile}`);
      }
      await waitMs(FILE_LOCK_RETRY_MS);
    }
  }
}

function readFileAddresses(denyListPath: string): string[] {
  return normalizeAddresses(readFileSync(denyListPath, "utf-8").split(NEWLINE));
}

function writeFileAddresses(denyListPath: string, addresses: readonly string[]): void {
  writeFileSync(denyListPath, addresses.length > 0 ? `${addresses.join(NEWLINE)}${NEWLINE}` : "");
}

async function mutateDenyListFile(
  denyListPath: string,
  toAdd: readonly string[],
  toRemove: readonly string[],
): Promise<void> {
  if (toAdd.length === 0 && toRemove.length === 0) {
    return;
  }
  const release = await acquireFileLock(denyListPath);
  try {
    const current = new Set(readFileAddresses(denyListPath));
    for (const a of toAdd) current.add(a);
    for (const a of toRemove) current.delete(a);
    writeFileAddresses(denyListPath, [...current]);
  } finally {
    release();
  }
}

export async function reloadDenyList(client: DenyListControlClient): Promise<void> {
  await client.pluginsReloadPluginConfig({
    pluginName: SequencerPluginName.TransactionPoolValidator,
  });
  await client.pluginsReloadPluginConfig({
    pluginName: SequencerPluginName.TransactionSelector,
  });
}

async function addToDenyListUnlocked(
  client: DenyListControlClient,
  normalizedAddresses: readonly string[],
  denyListPath: string,
): Promise<void> {
  await mutateDenyListFile(denyListPath, normalizedAddresses, []);
  await waitMs(DENY_LIST_FILE_SYNC_WAIT_MS);
  await reloadDenyList(client);
}

async function removeFromDenyListUnlocked(
  client: DenyListControlClient,
  normalizedAddresses: readonly string[],
  denyListPath: string,
): Promise<void> {
  await mutateDenyListFile(denyListPath, [], normalizedAddresses);
  await waitMs(DENY_LIST_FILE_SYNC_WAIT_MS);
  await reloadDenyList(client);
}

export async function addToDenyList(
  client: DenyListControlClient,
  addresses: readonly string[],
  denyListPath: string = DEFAULT_DENY_LIST_PATH,
): Promise<void> {
  const normalizedAddresses = normalizeAddresses(addresses);
  if (normalizedAddresses.length === 0) {
    return;
  }

  await withDenyListLock(async () => addToDenyListUnlocked(client, normalizedAddresses, denyListPath));
}

export async function removeFromDenyList(
  client: DenyListControlClient,
  addresses: readonly string[],
  denyListPath: string = DEFAULT_DENY_LIST_PATH,
): Promise<void> {
  const normalizedAddresses = normalizeAddresses(addresses);
  if (normalizedAddresses.length === 0) {
    return;
  }

  await withDenyListLock(async () => removeFromDenyListUnlocked(client, normalizedAddresses, denyListPath));
}

const DENY_LIST_EFFECT_WAIT_MS = 3_000;
// Docker bind-mount on macOS (virtiofs/gRPC-FUSE) can delay host→container file visibility.
// Wait before reloading so the sequencer sees the updated file.
const DENY_LIST_FILE_SYNC_WAIT_MS = 500;

/**
 * Waits a fixed duration for the sequencer's deny list to take effect after an async reload.
 */
export async function waitForDenyListEffect(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, DENY_LIST_EFFECT_WAIT_MS));
}

function waitMs(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export async function withDenyListAddresses(
  client: DenyListControlClient,
  addresses: readonly string[],
  run: () => Promise<void>,
  denyListPath: string = DEFAULT_DENY_LIST_PATH,
): Promise<void> {
  const normalizedAddresses = normalizeAddresses(addresses);
  if (normalizedAddresses.length === 0) {
    await run();
    return;
  }

  await withDenyListLock(async () => {
    try {
      await addToDenyListUnlocked(client, normalizedAddresses, denyListPath);
      await waitForDenyListEffect();
      await run();
    } finally {
      await removeFromDenyListUnlocked(client, normalizedAddresses, denyListPath);
      await waitForDenyListEffect();
    }
  });
}
