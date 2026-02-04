"use client";

import dynamic from "next/dynamic";

import { LayerswapSkeleton } from "./LayerswapSkeleton";

const LayerswapClientWrapper = dynamic(
  () => import("./LayerswapClientWrapper").then((mod) => mod.LayerswapClientWrapper),
  {
    ssr: false,
    loading: () => <LayerswapSkeleton />,
  },
);

export function Widget() {
  return <LayerswapClientWrapper />;
}
