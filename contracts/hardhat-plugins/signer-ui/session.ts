import { isSignerUiEnabled, handleSignerUiNetworkRequest, runSignerUiScope } from "./bridge.js";

import type { SignerUiScopeOptions } from "./bridge.js";
import type { NetworkHooks } from "hardhat/types/hooks";
import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";

/**
 * Runs `fn` with browser signing when `--signer-ui` (or `HARDHAT_SIGNER_UI=true`) is set; otherwise just runs it.
 * The signer-UI network handler is registered on `hre.hooks` for this call only, so it covers every connection the
 * task uses (including ones created by Ignition or OpenZeppelin Upgrades) and nothing outside it. The session closes
 * when `fn` settles; nested calls share it.
 */
export async function withSignerUi<T>(
  hre: HardhatRuntimeEnvironment,
  label: string,
  fn: () => Promise<T>,
  options?: SignerUiScopeOptions,
): Promise<T> {
  if (!hre.globalOptions.signerUi) {
    return fn();
  }
  if (isSignerUiEnabled()) {
    return runSignerUiScope(label, fn, options);
  }

  const handlers: Partial<NetworkHooks> = { onRequest: handleSignerUiNetworkRequest };
  hre.hooks.registerHandlers("network", handlers);
  try {
    return await runSignerUiScope(label, fn, options);
  } finally {
    hre.hooks.unregisterHandlers("network", handlers);
  }
}

export { getUiWalletClient, setUiTransactionContext, warnIfUsingPrivateKeySigning } from "./bridge.js";
