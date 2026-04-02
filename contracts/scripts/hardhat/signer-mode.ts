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
