/**
 * Next.js Configuration
 *
 * Build modes:
 * - Default: Client-side only with IndexedDB storage (no server required)
 * - Static export: Set STATIC_EXPORT=true for fully static deployment
 * - Server mode: Set NEXT_PUBLIC_STORAGE_MODE=server (not implemented, placeholder)
 *
 * Environment variables:
 * - STATIC_EXPORT: Set to "true" to build a static export (for GitHub Pages, etc.)
 * - NEXT_PUBLIC_STORAGE_MODE: "client" (default) or "server" (placeholder)
 */

const isStaticExport = process.env.STATIC_EXPORT === "true";

/** @type {import('next').NextConfig} */
const nextConfig = {
  // Use static export for client-only deployments, standalone for server deployments
  output: isStaticExport ? "export" : "standalone",
  reactStrictMode: true,
  sassOptions: {
    prependData: `@use 'sass:math'; @use 'src/scss/breakpoints' as *;`,
  },
  // Transpile workspace packages
  transpilePackages: [
    "@consensys/linea-contract-integrity-verifier",
    "@consensys/linea-contract-integrity-verifier-ethers",
    "@consensys/linea-contract-integrity-verifier-viem",
  ],
  // External packages for server components (only used in non-static builds)
  ...(isStaticExport ? {} : { serverExternalPackages: ["ethers", "viem"] }),
  // Webpack configuration for browser builds
  webpack: (config, { isServer }) => {
    // For client-side bundles, stub out Node.js built-in modules
    // This is needed because verifier-core imports fs at the module level
    // even though browser code doesn't use those functions
    if (!isServer) {
      config.resolve.fallback = {
        ...config.resolve.fallback,
        fs: false,
        path: false,
        os: false,
      };
    }
    return config;
  },
};

export default nextConfig;
