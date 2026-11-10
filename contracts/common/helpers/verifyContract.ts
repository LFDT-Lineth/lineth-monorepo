import { delay } from "./general.js";
import { clearSignerUiWorkflowStatus, setSignerUiWorkflowStatus } from "./signerUiWorkflowStatus.js";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";

const VERIFY_TIMEOUT_MS = 90_000;
const VERIFY_PROPAGATION_DELAY_MS = 30_000;

type VerifyRequest = {
  address: string;
  /** Fully qualified name (`<source>:<contract>`), required when several artifacts share the bytecode. */
  contract?: string;
  constructorArgs?: unknown[];
  libraries?: Record<string, string>;
};

type VerifyResult = "succeeded" | "failed" | "timed_out";

/**
 * Runs Etherscan verification in-process. The verification itself cannot be cancelled, so on timeout the deploy
 * moves on and the pending request finishes (or fails) in the background.
 */
async function runVerifyWithTimeout(request: VerifyRequest, hre: HardhatRuntimeEnvironment): Promise<VerifyResult> {
  // Loaded lazily: `common/helpers/index.ts` re-exports this module into the Hardhat-free scripts.
  const { verifyContract } = await import("@nomicfoundation/hardhat-verify/verify");
  let timeoutHandle: ReturnType<typeof setTimeout> | undefined;
  const verification = verifyContract(
    {
      address: request.address,
      provider: "etherscan",
      ...(request.contract === undefined ? {} : { contract: request.contract }),
      ...(request.constructorArgs === undefined ? {} : { constructorArgs: request.constructorArgs }),
      ...(request.libraries === undefined ? {} : { libraries: request.libraries }),
    },
    hre,
  ).then(
    (verified): VerifyResult => (verified ? "succeeded" : "failed"),
    (error: unknown): VerifyResult => {
      console.log(`Error happened during verification: ${error instanceof Error ? error.message : String(error)}`);
      return "failed";
    },
  );
  const timeout = new Promise<VerifyResult>((resolveTimeout) => {
    timeoutHandle = setTimeout(() => resolveTimeout("timed_out"), VERIFY_TIMEOUT_MS);
  });

  try {
    return await Promise.race([verification, timeout]);
  } finally {
    clearTimeout(timeoutHandle);
  }
}

/** Best-effort: waits for explorer propagation, verifies, and never fails the calling deploy. */
async function verifyBestEffort(request: VerifyRequest, hre: HardhatRuntimeEnvironment): Promise<void> {
  const contractAddress = request.address;
  await setSignerUiWorkflowStatus(
    "waiting_for_contract_verification",
    `Waiting for contract verification for ${contractAddress}. Explorer propagation may take around 30 seconds.`,
  );
  console.log("Waiting 30 seconds for contract propagation...");
  await delay(VERIFY_PROPAGATION_DELAY_MS);
  console.log("Etherscan verification ongoing...");

  let result: VerifyResult | "errored";

  try {
    result = await runVerifyWithTimeout(request, hre);
    if (result === "timed_out") {
      console.log(
        `Verification timed out after ${VERIFY_TIMEOUT_MS / 1000}s. Continuing deploy; you can verify ${contractAddress} separately later.`,
      );
    } else if (result === "failed") {
      console.log(`Verification failed for ${contractAddress}.`);
      console.log(`Continuing deploy; you can verify ${contractAddress} separately later.`);
    }
  } catch (error) {
    result = "errored";
    const message = error instanceof Error ? error.message : String(error);
    console.log(`Verification failed for ${contractAddress}: ${message}`);
    console.log(`Continuing deploy; you can verify ${contractAddress} separately later.`);
  } finally {
    await clearSignerUiWorkflowStatus();
  }

  if (result === "succeeded") {
    console.log("Etherscan verification done.");
  } else {
    console.log("Etherscan verification not confirmed.");
  }
}

/** Verifies `contractAddress` on Etherscan when `VERIFY_CONTRACT=true`. */
export async function tryVerifyContract(
  hre: HardhatRuntimeEnvironment,
  contractAddress: string,
  contractForVerification?: string,
): Promise<void> {
  if (process.env.VERIFY_CONTRACT === "true") {
    await verifyBestEffort(
      {
        address: contractAddress,
        ...(contractForVerification === undefined ? {} : { contract: contractForVerification }),
      },
      hre,
    );
  }
}

/** Verifies `contractAddress` with explicit constructor args (and linked libraries) when `VERIFY_CONTRACT=true`. */
export async function tryVerifyContractWithConstructorArgs(
  hre: HardhatRuntimeEnvironment,
  contractAddress: string,
  contractForVerification: string,
  args: unknown[],
  libraries?: Record<string, string>,
): Promise<void> {
  if (process.env.VERIFY_CONTRACT === "true") {
    await verifyBestEffort(
      {
        address: contractAddress,
        contract: contractForVerification,
        constructorArgs: args,
        ...(libraries === undefined ? {} : { libraries }),
      },
      hre,
    );
  }
}
