import { delay } from "./general";

type HardhatRun = (task: string, args?: Record<string, unknown>) => Promise<unknown>;

async function getHardhatRun(): Promise<HardhatRun> {
  // Use dynamic import to avoid loading hardhat during config initialization
  const hreModule = await import("hardhat");
  const run: HardhatRun | undefined = hreModule.run ?? (hreModule.default as { run?: HardhatRun } | undefined)?.run;

  if (!run) {
    throw new Error("Hardhat runtime not available; ensure this helper runs under Hardhat.");
  }

  return run;
}

export async function tryVerifyContract(contractAddress: string, contractForVerification?: string) {
  if (process.env.VERIFY_CONTRACT === "true") {
    console.log("Waiting 30 seconds for contract propagation...");
    await delay(30000);
    console.log("Etherscan verification ongoing...");
    // Verify contract
    try {
      const run = await getHardhatRun();
      const verifyArgs: Record<string, unknown> = {
        address: contractAddress,
      };
      if (contractForVerification) {
        verifyArgs.contract = contractForVerification;
      }
      await run("verify", verifyArgs);
    } catch (err) {
      console.log(`Error happened during verification: ${err}`);
    }
    console.log("Etherscan verification done.");
  }
}

export async function tryVerifyContractWithConstructorArgs(
  contractAddress: string,
  contractForVerification: string,
  args: unknown[],
) {
  if (process.env.VERIFY_CONTRACT === "true") {
    console.log("Waiting 30 seconds for contract propagation...");
    await delay(30000);
    console.log("Etherscan verification ongoing...");

    // Verify contract
    try {
      const run = await getHardhatRun();
      await run("verify:verify", {
        address: contractAddress,
        contract: contractForVerification,
        constructorArguments: args,
      });
    } catch (err) {
      console.log(`Error happened during verification: ${err}`);
    }
    console.log("Etherscan verification done.");
  }
}
