import { execFile } from "child_process";
import * as fs from "fs";
import * as path from "path";
import { Open } from "unzipper";

import { getBuildConfig, SUPPORTED_TARGETS, Target } from "./config";

const LIB_RESOURCES_DIR = path.resolve("src", "compressor", "lib");

async function downloadFileUsingCurl(url: string, outputFilePath: string): Promise<string> {
  const outputDirectory = path.dirname(outputFilePath);

  // Ensure the output directory exists
  fs.mkdirSync(outputDirectory, { recursive: true });

  return new Promise((resolve, reject) => {
    execFile(
      "curl",
      ["-L", "-H", "Accept:application/octet-stream", "-o", outputFilePath, url],
      (error: unknown, _: unknown, stderr: unknown) => {
        if (error) {
          reject(new Error(`Failed to download file using curl: ${stderr}`));
        } else {
          resolve(outputFilePath);
        }
      },
    );
  });
}

const architectureResourceDirMapping: Record<string, Target> = {
  darwin_arm64: "darwin-arm64",
  darwin_x86_64: "darwin-x64",
  linux_arm64: "linux-arm64",
  linux_amd64: "linux-x64",
  linux_x86_64: "linux-x64",
};

async function downloadReleaseAsset(nativeLibReleaseTag: string): Promise<string> {
  const assetReleaseUrl = `https://github.com/LFDT-Lineth/lineth-monorepo/releases/download/${nativeLibReleaseTag}/linea-${nativeLibReleaseTag}.zip`;
  const fileName = `${nativeLibReleaseTag}.zip`;
  const destPath = path.resolve("build", fileName);
  console.log(`Downloading ${fileName} from ${assetReleaseUrl} to ${destPath}`);
  return await downloadFileUsingCurl(assetReleaseUrl, destPath);
}

function getBinaryResourceFolder(libFile: string): Target {
  const destResource = Object.entries(architectureResourceDirMapping).find(([key]) => libFile.includes(key));
  if (!destResource) {
    throw new Error(`No architecture found for ${libFile}`);
  }
  return destResource[1];
}

function getBinaryResourceFileName(libFile: string, libName: string): string {
  const versionPattern = /v\d+\.\d+\.\d+/;
  const match = libFile.match(versionPattern);
  const version = match ? match[0] : null;
  const extension = path.extname(libFile);
  return `${libName}_${version}${extension}`;
}

/**
 * Removes binaries left by previous builds (other targets or release tags),
 * so only the binaries selected by this build end up in `dist`.
 */
function removePreviousBinaries(): void {
  for (const target of SUPPORTED_TARGETS) {
    fs.rmSync(path.join(LIB_RESOURCES_DIR, target), { recursive: true, force: true });
  }
}

async function downloadReleaseAndExtractToResources(
  nativeLibReleaseTag: string,
  libName: string,
  targets: Target[],
): Promise<void> {
  const outputFile = await downloadReleaseAsset(nativeLibReleaseTag);

  if (!fs.existsSync(outputFile)) {
    throw new Error(`Output file ${outputFile} does not exist`);
  }

  const extractPath = path.resolve("build", nativeLibReleaseTag);

  const zipFile = await Open.file(outputFile);
  await zipFile.extract({ path: extractPath, concurrency: 5 });

  console.log("Extraction complete");
  const files = fs.readdirSync(extractPath);

  if (files.length === 0) {
    throw new Error("No files found in the extracted zip file.");
  }

  removePreviousBinaries();
  const copiedTargets = new Set<Target>();

  for (const file of files) {
    if (file.includes(libName) && (file.endsWith(".so") || file.endsWith(".dylib"))) {
      const destResourceDir = getBinaryResourceFolder(file);
      if (!targets.includes(destResourceDir)) {
        continue;
      }

      const destResourceFileName = getBinaryResourceFileName(file, libName);
      const destPath = path.join(LIB_RESOURCES_DIR, destResourceDir);

      fs.mkdirSync(destPath, { recursive: true });
      fs.copyFileSync(path.join(extractPath, file), path.join(destPath, destResourceFileName));
      copiedTargets.add(destResourceDir);
      console.log(`Copying ${file} to ${path.join(destPath, destResourceFileName)}`);
    }
  }

  const missingTargets = targets.filter((target) => !copiedTargets.has(target));
  if (missingTargets.length > 0) {
    throw new Error(`Release ${nativeLibReleaseTag} has no ${libName} binary for: ${missingTargets.join(", ")}`);
  }
}

async function fetchLib(nativeLibReleaseTag: string, libName: string, targets: Target[]): Promise<void> {
  await downloadReleaseAndExtractToResources(nativeLibReleaseTag, libName, targets);
}

async function main() {
  const { nativeLibReleaseTag, targets } = getBuildConfig();
  console.log(`Keeping native lib binaries for: ${targets.join(", ")}`);
  await fetchLib(nativeLibReleaseTag, "blob_compressor", targets);
}

main()
  .then(() => {
    process.exit(0);
  })
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
