// Shared environment/.env parsing helpers for the quickstart internal TypeScript.

import * as fs from "node:fs";

export type EnvMap = Record<string, string | undefined>;

export function readDotEnvContents(contents: string): Record<string, string> {
  const result: Record<string, string> = {};
  for (const line of contents.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) {
      continue;
    }
    const index = trimmed.indexOf("=");
    if (index === -1) {
      continue;
    }
    const key = trimmed.slice(0, index).trim();
    let value = trimmed.slice(index + 1).trim();
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    result[key] = value;
  }
  return result;
}

export function readDotEnvFile(envPath: string): Record<string, string> {
  if (!fs.existsSync(envPath)) {
    throw new Error(`${envPath} is missing; copy .env.example to .env first`);
  }
  return readDotEnvContents(fs.readFileSync(envPath, "utf8"));
}

export function envValue(name: string, env: EnvMap, fallback = ""): string {
  const raw = env[name];
  return raw === undefined || raw === "" ? fallback : raw;
}

export function requiredEnvValue(name: string, env: EnvMap): string {
  const value = envValue(name, env);
  if (!value) {
    throw new Error(`${name} must be set in .env`);
  }
  return value;
}

/** Required value read directly from process.env (for CLI scripts not using an EnvMap). */
export function requiredProcessEnv(name: string): string {
  const value = process.env[name];
  if (value === undefined || value === "") {
    throw new Error(`${name} must be set`);
  }
  return value;
}

export function envNumber(name: string, env: EnvMap, fallback: number): number {
  const raw = envValue(name, env, fallback.toString());
  if (!/^[0-9]+$/.test(raw)) {
    throw new Error(`${name} must be an integer value`);
  }
  return Number(raw);
}

export function parseDecimalWei(name: string, raw: string): bigint {
  if (!/^[0-9]+$/.test(raw)) {
    throw new Error(`${name} must be an integer wei value`);
  }
  return BigInt(raw);
}

export function parseBoolean(name: string, raw: string): boolean {
  if (raw === "true") {
    return true;
  }
  if (raw === "false") {
    return false;
  }
  throw new Error(`${name} must be true or false (got '${raw}')`);
}
