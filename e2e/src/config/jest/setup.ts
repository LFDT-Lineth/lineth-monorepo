import { createTestLogger } from "../logger";

global.logger = createTestLogger();
global.shouldSkipBundleTests = process.env.SHOULD_SKIP_BUNDLE_TESTS === "true";
