import { existsSync, readdirSync } from "fs";
import os from "os";
import path from "path";

/**
 * Mapping of OS platforms to their respective compressor library file extensions.
 * @type {Record<string, string>}
 */
const OS_COMPRESSOR_LIB_EXTENSION_MAPPING: Record<string, string> = {
  darwin: ".dylib",
  linux: ".so",
  win32: ".exe",
};

/**
 * Gets the path to the compressor library based on the current OS platform and architecture.
 *
 * @returns {string} The absolute path to the compressor library file.
 * @throws {Error} Throws an error if the platform is unsupported, the directory does not exist, or no matching library file is found.
 */
export function getCompressorLibPath(): string {
  const platform = os.platform();
  const arch = os.arch();

  const directory = `${platform}-${arch}`;

  const fileExtension = OS_COMPRESSOR_LIB_EXTENSION_MAPPING[platform];

  if (!fileExtension) {
    throw new Error(`Unsupported platform: ${platform}`);
  }

  const dirPath = path.join(__dirname, "lib", directory);

  if (!existsSync(dirPath)) {
    throw new Error(`Directory does not exist: ${dirPath}`);
  }

  const files = readdirSync(dirPath);
  const libFile = files.find((file) => file.startsWith("blob_compressor") && file.endsWith(fileExtension));

  if (!libFile) {
    throw new Error(`No matching library file found in directory: ${dirPath}`);
  }

  return path.resolve(dirPath, libFile);
}
