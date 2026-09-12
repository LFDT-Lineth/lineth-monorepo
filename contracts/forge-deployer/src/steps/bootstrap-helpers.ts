// Pure helpers extracted from bootstrap.ts so they're importable/testable
// without triggering that file's top-level `main().catch(...)` entrypoint
// (bootstrap.ts, like the other step scripts, is always run standalone via
// process-runner.ts's spawn, never imported as a module).
import path from "node:path";

import { BootstrapItem } from "../bootstrap-manifest";

// Resolves a script item's path within `scriptsDir`, rejecting any `item.script`
// (e.g. containing `../`) that would resolve outside of it.
export function resolveBootstrapScriptPath(scriptsDir: string, itemId: string, itemScript: string): string {
  const root = path.resolve(scriptsDir);
  const scriptPath = path.resolve(root, itemScript);
  if (scriptPath !== root && !scriptPath.startsWith(root + path.sep)) {
    throw new Error(`bootstrap script item ${itemId} resolves outside BOOTSTRAP_SCRIPTS_DIR`);
  }
  return scriptPath;
}

// Funding (sign) items first so funded external accounts exist before any
// presigned tx that spends from them, then presigned, then scripts.
export function orderBootstrapItems(items: BootstrapItem[]): BootstrapItem[] {
  return [
    ...items.filter((item) => item.kind === "sign"),
    ...items.filter((item) => item.kind === "presigned"),
    ...items.filter((item) => item.kind === "script"),
  ];
}
