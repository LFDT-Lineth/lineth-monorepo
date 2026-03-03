import type { Account, Chain, Client, Transport } from "viem";

export type PluginsReloadPluginConfigParameters = {
  pluginName: string;
};

export type PluginsReloadPluginConfigReturnType = unknown;

export type PluginsReloadPluginConfigRpc = {
  Method: "plugins_reloadPluginConfig";
  Parameters: [PluginsReloadPluginConfigParameters["pluginName"]];
  ReturnType: PluginsReloadPluginConfigReturnType;
};

export async function pluginsReloadPluginConfig(
  client: Client<Transport, Chain | undefined, Account | undefined, [PluginsReloadPluginConfigRpc]>,
  params: PluginsReloadPluginConfigParameters,
): Promise<PluginsReloadPluginConfigReturnType> {
  return client.request({
    method: "plugins_reloadPluginConfig",
    params: [params.pluginName],
  });
}
