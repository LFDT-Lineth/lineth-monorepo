import "@layerswap/widget/index.css";
import { Swap, LayerswapProvider, GetSettings, ThemeData } from "@layerswap/widget";
import CustomHooks from "./custom-hooks";

export async function Widget() {
  const settings = await GetSettings();
  return (
    <LayerswapProvider integrator="linea" themeData={themeData} settings={settings} apiKey={"YOUR_API_KEY"}>
      <CustomHooks>
        <Swap
          featuredNetwork={{
            initialDirection: "to",
            network: "LINEA_MAINNET",
            oppositeDirectionOverrides: "onlyExchanges",
          }}
        />
      </CustomHooks>
    </LayerswapProvider>
  );
}
// TODO: Implement theme colors
const themeData: ThemeData = {
  placeholderText: "134, 134, 134",
  actionButtonText: "255, 255, 255",
  buttonTextColor: "17, 17, 17",
  logo: "255, 0, 147",
  borderRadius: "large",
  primary: {
    DEFAULT: "97, 26, 239",
    "50": "215, 198, 251",
    "100": "202, 179, 250",
    "200": "176, 140, 247",
    "300": "150, 102, 244",
    "400": "123, 64, 242",
    "500": "97, 26, 239",
    "600": "74, 14, 195",
    "700": "54, 10, 143",
    "800": "34, 6, 90",
    "900": "14, 3, 38",
    text: "18, 18, 18",
    textMuted: "86, 97, 123",
  },
  secondary: {
    DEFAULT: "248, 247, 241",
    "50": "49, 60, 155",
    "100": "46, 59, 147",
    "200": "134, 134, 134",
    "300": "139, 139, 139",
    "400": "220, 219, 214",
    "500": "228, 227, 219",
    "600": "240, 240, 235",
    "700": "248, 247, 241",
    "800": "243, 244, 246",
    "900": "255, 255, 255",
    "950": "255, 255, 255",
    text: "82, 82, 82",
  },
};
