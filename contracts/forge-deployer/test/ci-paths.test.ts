import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const REPOSITORY_ROOT = path.resolve(__dirname, "../../..");
const REQUIRED_PATHS = [
  "contracts/forge-deployer/**",
  "contracts/common/**",
  "contracts/local-deployments-artifacts/**",
  "contracts/package.json",
  "package.json",
  "pnpm-lock.yaml",
  "pnpm-workspace.yaml",
  "tsconfig.json",
  ".npmrc",
  "patches/**",
  "scripts/docker/**",
  ".github/actions/docker-build-publish/**",
  ".github/workflows/contracts-forge-deployer-*.yml",
  ".github/workflows/build-and-publish.yml",
  ".github/workflows/main.yml",
  ".nvmrc",
] as const;

function namedYamlBlock(file: string, name: string): string {
  const lines = readFileSync(path.join(REPOSITORY_ROOT, file), "utf8").split("\n");
  const start = lines.findIndex((line) => line.trim() === `${name}:`);
  assert.notEqual(start, -1, `${file} is missing ${name}:`);
  const indentation = lines[start]!.search(/\S/);
  const block = [lines[start]!];
  for (const line of lines.slice(start + 1)) {
    if (line.trim() && line.search(/\S/) <= indentation) break;
    block.push(line);
  }
  return block.join("\n");
}

function quotedLiteral(value: string): RegExp {
  const escaped = value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return new RegExp(`['"]${escaped}['"]`);
}

test("keeps the Forge deployer component and E2E build-input paths aligned", () => {
  const component = namedYamlBlock(".github/workflows/main.yml", "contract-deployer");
  const e2e = namedYamlBlock(".github/workflows/get-has-changes-requiring-e2e-testing.yml", "e2e-paths");

  for (const requiredPath of REQUIRED_PATHS) {
    assert.match(component, quotedLiteral(requiredPath), `component filter: ${requiredPath}`);
    assert.match(e2e, quotedLiteral(requiredPath), `E2E gate: ${requiredPath}`);
  }
});

test("passes the full checked-out source SHA to the Forge deployer workflow", () => {
  const deployer = namedYamlBlock(".github/workflows/build-and-publish.yml", "contract_deployer");

  assert.match(deployer, /image_tag:\s*\$\{\{\s*github\.event\.pull_request\.head\.sha\s*\|\|\s*github\.sha\s*\}\}/);
  assert.doesNotMatch(deployer, /image_tag:\s*\$\{\{\s*inputs\.commit_tag\s*\}\}/);
});
