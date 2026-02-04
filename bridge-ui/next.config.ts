import type { NextConfig } from "next";

export const isProd = process.env.NEXT_PUBLIC_ENVIRONMENT === "production";
const basePath = isProd ? "/hub/bridge" : "";

const nextConfig: NextConfig = {
  output: "standalone",
  reactStrictMode: true,
  basePath,
  env: {
    NEXT_PUBLIC_BASE_PATH: basePath,
  },
  experimental: {
    optimizePackageImports: ["@lifi/widget", "@layerswap/widget", "gsap", "motion"],
  },
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "s2.coinmarketcap.com",
        pathname: "/static/img/coins/**",
      },
      {
        protocol: "https",
        hostname: "assets.coingecko.com",
        pathname: "/coins/images/**",
      },
      {
        protocol: "https",
        hostname: "linea.build",
        pathname: "/icons/**",
      },
      {
        protocol: "https",
        hostname: "images.ctfassets.net",
        pathname: `/${process.env.CONTENTFUL_SPACE_ID}/**`,
      },
    ],
  },
  serverExternalPackages: ["pino-pretty", "lokijs", "encoding"],
  sassOptions: {
    loadPaths: ["src/scss"],
    prependData: `@use 'sass:math'; @use 'breakpoints' as *;`,
  },
  turbopack: {
    rules: {
      "*.svg": {
        loaders: ["@svgr/webpack"],
        as: "*.js",
      },
    },
  },
};

export default nextConfig;
