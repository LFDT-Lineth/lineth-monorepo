import { delay } from "./general";

export async function tryVerifyContract(contractAddress: string, contractForVerification?: string) {
  if (process.env.VERIFY_CONTRACT === "true") {
    console.log("Waiting 30 seconds for contract propagation...");
    await delay(30000);
    console.log("Etherscan verification ongoing...");
    // Verify contract
    try {
      // Use dynamic import to avoid loading hardhat during config initialization
      const { run } = await import("hardhat");
      const verifyArgs: Record<string, string> = {
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
      // Use dynamic import to avoid loading hardhat during config initialization
      const { run } = await import("hardhat");
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
