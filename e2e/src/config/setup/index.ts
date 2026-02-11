import { createTestSetup } from "./test-setup-factory";

import type TestSetupCore from "./test-setup-core";

export type TestContext = TestSetupCore;

export function createTestContext(): TestContext {
  return createTestSetup();
}
