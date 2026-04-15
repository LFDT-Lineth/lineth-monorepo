import { readFileSync, writeFileSync } from "fs";
import { resolve } from "path";

import { SequencerPluginName } from "../../config/clients/linea-rpc/sequencer-plugins";
import { toLowercaseLines } from "../utils/string";

import type { PluginsReloadPluginConfigParameters } from "../../config/clients/linea-rpc/plugins-reload-plugin-config";

const DEFAULT_DENY_LIST_PATH = resolve(__dirname, "../../../..", "docker/config/linea-besu-sequencer/deny-list.txt");
const NEWLINE = "\n";
let denyListOperationQueue: Promise<void> = Promise.resolve();

type DenyListState = {
  baseAddresses: string[];
  dynamicAddressCounts: Map<string, number>;
};

const denyListStates = new Map<string, DenyListState>();

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

function parseDenyListContent(content: string): string[] {
  return normalizeAddresses(content.split(NEWLINE));
}

function getOrCreateDenyListState(denyListPath: string): DenyListState {
  const existingState = denyListStates.get(denyListPath);
  if (existingState) {
    return existingState;
  }

  const state: DenyListState = {
    baseAddresses: parseDenyListContent(readFileSync(denyListPath, "utf-8")),
    dynamicAddressCounts: new Map<string, number>(),
  };
  denyListStates.set(denyListPath, state);
  return state;
}

function getEffectiveDenyListAddresses(state: DenyListState): string[] {
  const addresses = [...state.baseAddresses];
  const existingAddresses = new Set(state.baseAddresses);

  for (const [address, count] of state.dynamicAddressCounts) {
    if (count > 0 && !existingAddresses.has(address)) {
      addresses.push(address);
    }
  }

  return addresses;
}

function writeDenyListState(denyListPath: string, state: DenyListState): void {
  const addresses = getEffectiveDenyListAddresses(state);
  writeFileSync(denyListPath, addresses.length > 0 ? `${addresses.join(NEWLINE)}${NEWLINE}` : "");
}

export async function reloadDenyList(client: DenyListControlClient): Promise<void> {
  await client.pluginsReloadPluginConfig({
    pluginName: SequencerPluginName.TransactionPoolValidator,
  });
}

async function addToDenyListUnlocked(
  client: DenyListControlClient,
  normalizedAddresses: readonly string[],
  denyListPath: string,
): Promise<void> {
  const state = getOrCreateDenyListState(denyListPath);

  for (const address of normalizedAddresses) {
    state.dynamicAddressCounts.set(address, (state.dynamicAddressCounts.get(address) ?? 0) + 1);
  }

  writeDenyListState(denyListPath, state);
  await reloadDenyList(client);
}

async function removeFromDenyListUnlocked(
  client: DenyListControlClient,
  normalizedAddresses: readonly string[],
  denyListPath: string,
): Promise<void> {
  const state = getOrCreateDenyListState(denyListPath);

  for (const address of normalizedAddresses) {
    const currentCount = state.dynamicAddressCounts.get(address) ?? 0;
    if (currentCount <= 1) {
      state.dynamicAddressCounts.delete(address);
      continue;
    }

    state.dynamicAddressCounts.set(address, currentCount - 1);
  }

  writeDenyListState(denyListPath, state);
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
      await run();
    } finally {
      await removeFromDenyListUnlocked(client, normalizedAddresses, denyListPath);
    }
  });
}
