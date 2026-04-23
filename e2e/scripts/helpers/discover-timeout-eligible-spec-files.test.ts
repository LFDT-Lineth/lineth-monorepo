import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { discoverTimeoutEligibleSpecFiles } from "./discover-timeout-eligible-spec-files";

describe("discoverTimeoutEligibleSpecFiles", () => {
  it("returns top-level spec files except the ignored local specs", () => {
    // Arrange
    const projectDir = mkdtempSync(join(tmpdir(), "e2e-runtime-report-"));
    const srcDir = join(projectDir, "src");
    const nestedDir = join(srcDir, "common", "test-helpers");

    mkdirSync(nestedDir, { recursive: true });
    writeFileSync(join(srcDir, "l2.spec.ts"), "");
    writeFileSync(join(srcDir, "messaging.spec.ts"), "");
    writeFileSync(join(srcDir, "liveness.spec.ts"), "");
    writeFileSync(join(srcDir, "linea-besu-fleet.spec.ts"), "");
    writeFileSync(join(nestedDir, "deny-list.spec.ts"), "");

    // Act
    const result = discoverTimeoutEligibleSpecFiles(projectDir);

    // Assert
    expect(result).toEqual(["src/l2.spec.ts", "src/messaging.spec.ts"]);

    rmSync(projectDir, { recursive: true, force: true });
  });
});
