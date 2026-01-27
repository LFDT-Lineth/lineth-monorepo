import { execFile } from "child_process";
import * as fs from "fs";
import * as path from "path";
import { Open } from "unzipper";

import { getBuildConfig } from "./config";

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

const architectureResourceDirMapping: Record<string, string> = {
  darwin_arm64: "darwin-arm64",
  darwin_x86_64: "darwin-x64",
  linux_arm64: "linux-arm64",
  linux_amd64: "linux-x64",
  linux_x86_64: "linux-x64",
};

async function downloadReleaseAsset(nativeLibReleaseTag: string): Promise<string> {
  const assetReleaseUrl = `https://github.com/Consensys/linea-monorepo/releases/download/${nativeLibReleaseTag}/linea-${nativeLibReleaseTag}.zip`;
  const fileName = `${nativeLibReleaseTag}.zip`;
  const destPath = path.resolve("build", fileName);
  console.log(`Downloading ${fileName} from ${assetReleaseUrl} to ${destPath}`);
  return await downloadFileUsingCurl(assetReleaseUrl, destPath);
}

function getBinaryResourceFolder(libFile: string): string {
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

async function downloadReleaseAndExtractToResources(nativeLibReleaseTag: string, libName: string): Promise<void> {
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

  for (const file of files) {
    if (file.includes(libName) && (file.endsWith(".so") || file.endsWith(".dylib"))) {
      const destResourceDir = getBinaryResourceFolder(file);
      const destResourceFileName = getBinaryResourceFileName(file, libName);
      const destPath = path.resolve("src", "compressor", "lib", destResourceDir);

      fs.mkdirSync(destPath, { recursive: true });
      fs.copyFileSync(path.join(extractPath, file), path.join(destPath, destResourceFileName));
      console.log(`Copying ${file} to ${path.join(destPath, destResourceFileName)}`);
    }
  }
}

async function fetchLib(nativeLibReleaseTag: string, libName: string): Promise<void> {
  await downloadReleaseAndExtractToResources(nativeLibReleaseTag, libName);
}

async function main() {
  const { nativeLibReleaseTag } = getBuildConfig();
  await fetchLib(nativeLibReleaseTag, "blob_compressor");
}

main()
  .then(() => {
    process.exit(0);
  })
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
