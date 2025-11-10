import { getSettings } from "@layerswap/widget";
import { LayerswapClientWrapper } from "./LayerswapClientWrapper";
import { config } from "@/config";

export async function Widget() {
  const settings = await getSettings(config.layerswapApiKey);

  return <LayerswapClientWrapper settings={settings} />;
}
