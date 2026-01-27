import { getSettings } from "@layerswap/widget";

import { config } from "@/config";

import { LayerswapClientWrapper } from "./LayerswapClientWrapper";

export async function Widget() {
  const settings = await getSettings(config.layerswapApiKey);

  return <LayerswapClientWrapper settings={settings} />;
}
