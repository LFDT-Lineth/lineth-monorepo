"use client";

import { config } from "@/config";

import styles from "./onramper.module.scss";

const onRamperConfig: Record<string, string | string[]> = {
  apiKey: config.onRamperApiKey,
  mode: ["buy"],
  defaultCrypto: "eth_linea",
  onlyCryptoNetworks: "linea",
  onlyCryptos: ["eth_linea", "usdc_linea", "usdt_linea", "weth_linea"],
  primaryColor: "6119ef",
  secondaryColor: "ffffff",
  primaryTextColor: "121212",
  secondaryTextColor: "525252",
  containerColor: "ffffff",
  cardColor: "f8f7f2",
  primaryBtnTextColor: "ffffff",
  borderRadius: "0.625rem",
  widgetBorderRadius: "0.625rem",
};

function generateOnramperUrl(config: Record<string, string | string[]>): string {
  const url = new URL("https://buy.onramper.com");
  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(config)) {
    const val = Array.isArray(value) ? value.join(",") : value;
    params.set(key, val);
  }

  url.search = params.toString();
  return url.toString();
}

export default function OnRamperWidget() {
  return (
    <iframe
      className={styles["onramper-iframe"]}
      src={generateOnramperUrl(onRamperConfig)}
      title="Linea Onramper Widget"
      allow="accelerometer; autoplay; camera; gyroscope; payment; microphone"
    />
  );
}
