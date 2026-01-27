import path from "path";
import { fileURLToPath } from "url";

/**
 * Gets the directory path of the current module, compatible with both ES modules and CommonJS.
 * Uses import.meta.url in ES modules and __dirname in CommonJS.
 * tsup will transform this appropriately for each output format.
 *
 * @returns {string} The directory path of the current module.
 */
export function getModuleDir(): string {
  // In CommonJS, __dirname is available (check first for CJS compatibility)
  if (typeof __dirname !== "undefined") {
    return __dirname;
  }
  // In ES modules, use import.meta.url
  // tsup will handle the transformation: ESM builds use import.meta.url, CJS builds use __dirname
  // The check below will be evaluated at runtime in ESM builds
  try {
    // @ts-expect-error - import.meta.url is available in ESM but TypeScript complains for CJS
    const moduleUrl = import.meta.url;
    if (moduleUrl) {
      return path.dirname(fileURLToPath(moduleUrl));
    }
  } catch {
    // import.meta not available, fall through to process.cwd()
  }
  // Fallback to current working directory if neither is available
  return process.cwd();
}
