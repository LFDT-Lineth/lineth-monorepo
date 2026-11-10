import { existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

import type { DeployCompanion, DeploymentParameters, ParameterContext } from "./types.js";
import type { IgnitionModule } from "@nomicfoundation/ignition-core";
import type { NetworkConnection } from "hardhat/types/network";

const COMPANION_SUFFIX = ".deploy.ts";

/** Ignition module files under `<ignition>/modules`: every `.ts` except `lib/` and `*.deploy.ts` companions. */
export function findModuleFiles(modulesDir: string): string[] {
  return readdirSync(modulesDir, { withFileTypes: true }).flatMap((entry) => {
    const fullPath = path.join(modulesDir, entry.name);
    if (entry.isDirectory()) {
      return entry.name === "lib" ? [] : findModuleFiles(fullPath);
    }
    return entry.name.endsWith(".ts") && !entry.name.endsWith(COMPANION_SUFFIX) ? [fullPath] : [];
  });
}

/** `LinethRollup` → `<ignition>/modules/rollup/LinethRollup.ts`; paths are returned resolved, unchanged. */
export function resolveModulePath(ignitionDir: string, moduleIdOrPath: string): string {
  if (moduleIdOrPath.includes("/") || moduleIdOrPath.includes("\\") || moduleIdOrPath.endsWith(".ts")) {
    return path.resolve(moduleIdOrPath);
  }
  const matches = findModuleFiles(path.join(ignitionDir, "modules")).filter(
    (file) => path.basename(file, ".ts") === moduleIdOrPath,
  );
  const [match] = matches;
  if (matches.length !== 1 || match === undefined) {
    throw new Error(
      matches.length === 0
        ? `No Ignition module named "${moduleIdOrPath}" under ${path.join(ignitionDir, "modules")}.`
        : `Module name "${moduleIdOrPath}" is ambiguous: ${matches.join(", ")}`,
    );
  }
  return match;
}

function isIgnitionModule(value: unknown): value is IgnitionModule {
  return typeof value === "object" && value !== null && "id" in value && "futures" in value && "submodules" in value;
}

/** Imports the module file the same way Ignition does, so both see the same module object (and recorded targets). */
export async function loadIgnitionModule(modulePath: string): Promise<IgnitionModule> {
  const imported: unknown = await import(pathToFileURL(modulePath).href);
  const candidate =
    typeof imported === "object" && imported !== null && "default" in imported ? imported.default : imported;
  if (!isIgnitionModule(candidate)) {
    throw new Error(`${modulePath} does not export an Ignition module (buildModule) as default.`);
  }
  return candidate;
}

/** The optional `<Module>.deploy.ts` next to the module file. */
export async function loadCompanion(modulePath: string): Promise<DeployCompanion> {
  const companionPath = modulePath.replace(/\.ts$/, COMPANION_SUFFIX);
  if (!existsSync(companionPath)) {
    return {};
  }
  const companion: DeployCompanion = await import(pathToFileURL(companionPath).href);
  return companion;
}

/** JSON replacer writing bigints the way Ignition's parameter reader revives them (`"123n"`). */
export function jsonBigint(_key: string, value: unknown): unknown {
  return typeof value === "bigint" ? `${value.toString()}n` : value;
}

function reviveBigint(_key: string, value: unknown): unknown {
  return typeof value === "string" && /^\d+n$/.test(value) ? BigInt(value.slice(0, -1)) : value;
}

/** Reads an Ignition parameters file (JSON, bigints as `"123n"`). */
export function readParametersFile(filePath: string): DeploymentParameters {
  const parsed: DeploymentParameters = JSON.parse(readFileSync(filePath, "utf8"), reviveBigint);
  return parsed;
}

/** Writes `parameters` into `filePath`, replacing only their top-level module entries (other modules are kept). */
export function writeParametersFile(filePath: string, parameters: DeploymentParameters): void {
  const existing = existsSync(filePath) ? readParametersFile(filePath) : {};
  mkdirSync(path.dirname(filePath), { recursive: true });
  writeFileSync(filePath, `${JSON.stringify({ ...existing, ...parameters }, jsonBigint, 2)}\n`);
}

/** What `<Module>.deploy.ts` parameter builders read about the target network. */
export async function createParameterContext(connection: NetworkConnection): Promise<ParameterContext> {
  const publicClient = await connection.viem.getPublicClient();
  return {
    networkName: connection.networkName,
    chainId: BigInt(await publicClient.getChainId()),
    getCode: (address) => publicClient.getCode({ address }),
  };
}
