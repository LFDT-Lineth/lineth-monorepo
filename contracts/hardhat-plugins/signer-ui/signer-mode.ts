export type SignerModeConflictDetails = {
  hardhatSignerUiEnabled?: boolean;
  deployerPrivateKeyConfigured?: boolean;
};

function hasTrimmedValue(value?: string): boolean {
  return (value?.trim().length ?? 0) > 0;
}

export function hasConfiguredDeployerPrivateKey(
  deployerPrivateKey: string | undefined = process.env.DEPLOYER_PRIVATE_KEY,
): boolean {
  return hasTrimmedValue(deployerPrivateKey);
}

export function isSignerUiEnabled(hardhatSignerUiEnabled: string | undefined = process.env.HARDHAT_SIGNER_UI): boolean {
  return hardhatSignerUiEnabled === "true";
}

export function getSignerModeConflictMessage(): string {
  return (
    "Invalid signer configuration: HARDHAT_SIGNER_UI=true and DEPLOYER_PRIVATE_KEY are both set. " +
    "Choose exactly one signing mode: either unset DEPLOYER_PRIVATE_KEY and keep HARDHAT_SIGNER_UI=true to sign in the browser, " +
    "or unset HARDHAT_SIGNER_UI and keep DEPLOYER_PRIVATE_KEY for private-key signing."
  );
}

export function assertExclusiveSignerMode(details: SignerModeConflictDetails = {}): void {
  const signerUiEnabled = details.hardhatSignerUiEnabled ?? isSignerUiEnabled();
  const privateKeyConfigured = details.deployerPrivateKeyConfigured ?? hasConfiguredDeployerPrivateKey();

  if (signerUiEnabled && privateKeyConfigured) {
    throw new Error(getSignerModeConflictMessage());
  }
}

type ResolveDeployerAccountsInputs = {
  hardhatSignerUiEnabled?: string;
  deployerPrivateKey?: string;
};

/**
 * `HARDHAT_SIGNER_UI=true` -> no local keys (browser wallet via signer-ui bridge).
 * If `DEPLOYER_PRIVATE_KEY` is unset -> `[]` so compile/clean can run without secrets.
 * If a key is set, it must be a valid non-zero hex scalar.
 */
export function resolveDeployerAccounts(inputs: ResolveDeployerAccountsInputs = {}): string[] {
  const hardhatSignerUiEnabled = inputs.hardhatSignerUiEnabled ?? process.env.HARDHAT_SIGNER_UI;
  const deployerPrivateKey = inputs.deployerPrivateKey ?? process.env.DEPLOYER_PRIVATE_KEY;
  const signerUiEnabled = isSignerUiEnabled(hardhatSignerUiEnabled);

  const signerModeConflictDetails = {
    hardhatSignerUiEnabled: signerUiEnabled,
    deployerPrivateKeyConfigured: hasConfiguredDeployerPrivateKey(deployerPrivateKey),
  };

  assertExclusiveSignerMode(signerModeConflictDetails);

  if (signerUiEnabled) {
    return [];
  }

  if (!hasConfiguredDeployerPrivateKey(deployerPrivateKey)) {
    return [];
  }

  const raw = deployerPrivateKey!.trim();
  const normalized = raw.startsWith("0x") ? raw : `0x${raw}`;
  let scalar: bigint;
  try {
    scalar = BigInt(normalized);
  } catch {
    throw new Error(
      "DEPLOYER_PRIVATE_KEY is not valid hex. Set a real key, or set HARDHAT_SIGNER_UI=true to sign via the browser.",
    );
  }

  if (scalar === 0n) {
    throw new Error(
      "DEPLOYER_PRIVATE_KEY cannot be zero. Set HARDHAT_SIGNER_UI=true for browser signing, or use a real key.",
    );
  }

  return [normalized];
}
