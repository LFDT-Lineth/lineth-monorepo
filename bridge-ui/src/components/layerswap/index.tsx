"use client";

import { config } from "@/config";
import styles from "./layerswap.module.scss";

const layerswapConfig: Record<string, string | string[]> = {
  clientId: config.layerswapApiKey,
};

function generateLayerswapUrl(config: Record<string, string | string[]>): string {
  const url = new URL("https://layerswap.io/app/");
  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(config)) {
    const val = Array.isArray(value) ? value.join(",") : value;
    params.set(key, val);
  }

  url.search = params.toString();
  return url.toString();
}

export default function LayerSwapWidget() {
  return (
    <iframe
      className={styles["layerswap-iframe"]}
      src={generateLayerswapUrl(layerswapConfig)}
      title="Linea LayerSwap Widget"
    />
  );
}
