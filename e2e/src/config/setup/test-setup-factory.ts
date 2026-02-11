import TestSetupCore from "./test-setup-core";
import { Config } from "../schema/config-schema";
import devConfig from "../schema/dev";
import localConfig from "../schema/local";
import sepoliaConfig from "../schema/sepolia";

export type TestEnv = "local" | "dev" | "sepolia";

const CONFIG_MAP: Record<TestEnv, Config> = {
  local: localConfig,
  dev: devConfig,
  sepolia: sepoliaConfig,
};

function resolveTestEnv(): TestEnv {
  const env = process.env.TEST_ENV ?? "local";

  if (!(env in CONFIG_MAP)) {
    throw new Error(`Unknown TEST_ENV "${env}". Expected one of: ${Object.keys(CONFIG_MAP).join(", ")}`);
  }

  return env as TestEnv;
}

export function createTestSetup(): TestSetupCore {
  const env = resolveTestEnv();
  return new TestSetupCore(CONFIG_MAP[env], env);
}
