import RootLayout from "@/app/RootLayout";
import "@layerswap/widget/index.css";

export default function LayerswapLayout({ children }: { children: React.ReactNode }) {
  return <RootLayout>{children}</RootLayout>;
}
