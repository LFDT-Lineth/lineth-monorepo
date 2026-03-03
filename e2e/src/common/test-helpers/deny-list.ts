import { appendFileSync, readFileSync, writeFileSync } from "fs";
import { resolve } from "path";

import { SequencerPluginName } from "../../config/clients/linea-rpc/sequencer-plugins";
import { toLowercaseLines } from "../utils/string";

import type { PluginsReloadPluginConfigParameters } from "../../config/clients/linea-rpc/plugins-reload-plugin-config";

const DEFAULT_DENY_LIST_PATH = resolve(__dirname, "../../../..", "docker/config/linea-besu-sequencer/deny-list.txt");
const NEWLINE = "\n";

type DenyListControlClient = {
  pluginsReloadPluginConfig: (args: PluginsReloadPluginConfigParameters) => Promise<unknown>;
};

export async function reloadDenyList(client: DenyListControlClient): Promise<void> {
  await client.pluginsReloadPluginConfig({
    pluginName: SequencerPluginName.TransactionPoolValidator,
  });
}

export function addToDenyList(addresses: readonly string[], denyListPath: string = DEFAULT_DENY_LIST_PATH): void {
  const existingContent = readFileSync(denyListPath, "utf-8");
  const prefix = existingContent.length > 0 && !existingContent.endsWith(NEWLINE) ? NEWLINE : "";
  const data = `${prefix}${toLowercaseLines(addresses).join(NEWLINE)}${NEWLINE}`;
  appendFileSync(denyListPath, data);
}

export function removeFromDenyList(addresses: readonly string[], denyListPath: string = DEFAULT_DENY_LIST_PATH): void {
  const current = readFileSync(denyListPath, "utf-8");
  const toRemove = new Set(toLowercaseLines(addresses));
  const remaining = current
    .split(NEWLINE)
    .filter(Boolean)
    .filter((address) => !toRemove.has(address.toLowerCase()));
  writeFileSync(denyListPath, remaining.length ? `${remaining.join(NEWLINE)}${NEWLINE}` : "");
}

export async function withDenyListAddresses(
  client: DenyListControlClient,
  addresses: readonly string[],
  run: () => Promise<void>,
  denyListPath: string = DEFAULT_DENY_LIST_PATH,
): Promise<void> {
  addToDenyList(addresses, denyListPath);
  await reloadDenyList(client);

  try {
    await run();
  } finally {
    removeFromDenyList(addresses, denyListPath);
    await reloadDenyList(client);
  }
}
