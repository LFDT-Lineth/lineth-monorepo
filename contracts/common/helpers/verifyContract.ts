import { spawn } from "node:child_process";
import { resolve } from "node:path";

import { delay } from "./general";
import { clearSignerUiWorkflowStatus, setSignerUiWorkflowStatus } from "./signerUiWorkflowStatus";
import { stringifyVerifyTaskArgs } from "../../scripts/hardhat/verify-task-args";

const VERIFY_TIMEOUT_MS = 90_000;
const VERIFY_PROPAGATION_DELAY_MS = 30_000;
const VERIFY_CHILD_SCRIPT = resolve(__dirname, "../../scripts/hardhat/run-verify-task.ts");

async function getCurrentHardhatNetworkName(): Promise<string> {
  const hreModule = await import("hardhat");
  const networkName =
    hreModule.network?.name ?? (hreModule.default as { network?: { name?: string } } | undefined)?.network?.name;

  if (!networkName) {
    throw new Error("Hardhat network name is not available; ensure verification runs under Hardhat.");
  }

  return networkName;
}

async function runVerifyTaskWithTimeout(
  task: string,
  args: Record<string, unknown>,
): Promise<"succeeded" | "failed" | "timed_out"> {
  const networkName = await getCurrentHardhatNetworkName();

  return await new Promise<"succeeded" | "failed" | "timed_out">((resolveRun) => {
    const child = spawn(
      "pnpm",
      ["exec", "hardhat", "run", "--no-compile", "--network", networkName, VERIFY_CHILD_SCRIPT],
      {
        cwd: resolve(__dirname, "../.."),
        env: {
          ...process.env,
          HARDHAT_VERIFY_TASK: task,
          HARDHAT_VERIFY_ARGS: stringifyVerifyTaskArgs(args),
        },
        stdio: "inherit",
      },
    );

    let settled = false;
    let timedOut = false;
    let killTimer: ReturnType<typeof setTimeout> | undefined;

    const finish = (result: "succeeded" | "failed" | "timed_out") => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timeoutHandle);
      clearTimeout(killTimer);
      child.off("exit", onExit);
      child.off("error", onError);
      resolveRun(result);
    };

    const onExit = (code: number | null, signal: NodeJS.Signals | null) => {
      if (timedOut) {
        finish("timed_out");
        return;
      }
      if (code === 0) {
        finish("succeeded");
        return;
      }
      console.log(`Verification process exited with code ${code ?? "null"}${signal ? ` (signal: ${signal})` : ""}.`);
      finish("failed");
    };

    const onError = (error: Error) => {
      console.log(`Error happened during verification: ${error}`);
      finish("failed");
    };

    const timeoutHandle = setTimeout(() => {
      timedOut = true;
      try {
        child.kill("SIGTERM");
      } catch {
        /* child may already be gone */
      }
      // No .unref(): SIGKILL must fire even if the parent event loop would otherwise exit.
      // finish("timed_out") is deferred to the exit handler so the child is reaped first.
      killTimer = setTimeout(() => {
        try {
          child.kill("SIGKILL");
        } catch {
          /* child may already be gone */
        }
      }, 1000);
    }, VERIFY_TIMEOUT_MS);

    child.on("exit", onExit);
    child.on("error", onError);
  });
}

async function verifyBestEffort(task: string, args: Record<string, unknown>, contractAddress: string): Promise<void> {
  await setSignerUiWorkflowStatus(
    "waiting_for_contract_verification",
    `Waiting for contract verification for ${contractAddress}. Explorer propagation may take around 30 seconds.`,
  );
  console.log("Waiting 30 seconds for contract propagation...");
  await delay(VERIFY_PROPAGATION_DELAY_MS);
  console.log("Etherscan verification ongoing...");

  let result: "succeeded" | "failed" | "timed_out" | "errored";

  try {
    result = await runVerifyTaskWithTimeout(task, args);
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

export async function tryVerifyContract(contractAddress: string, contractForVerification?: string) {
  if (process.env.VERIFY_CONTRACT === "true") {
    const verifyArgs: Record<string, unknown> = {
      address: contractAddress,
    };
    if (contractForVerification) {
      verifyArgs.contract = contractForVerification;
    }
    await verifyBestEffort("verify", verifyArgs, contractAddress);
  }
}

export async function tryVerifyContractWithConstructorArgs(
  contractAddress: string,
  contractForVerification: string,
  args: unknown[],
  libraries?: Record<string, string>,
) {
  if (process.env.VERIFY_CONTRACT === "true") {
    await verifyBestEffort(
      "verify:verify",
      {
        address: contractAddress,
        contract: contractForVerification,
        constructorArguments: args,
        libraries: libraries,
      },
      contractAddress,
    );
  }
}
