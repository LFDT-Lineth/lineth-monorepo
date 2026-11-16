import { FutureType } from "@nomicfoundation/ignition-core";
import { decodeFunctionData } from "viem";

import type { SerializedTransactionRequest, UiTransactionDetails } from "../signer-ui/bridge.js";
import type { Future, IgnitionModule } from "@nomicfoundation/ignition-core";
import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { Abi } from "viem";

type KnownContract = { futureId: string; contractName: string; abi: Abi; bytecode?: string };

function allFutures(ignitionModule: IgnitionModule, seen = new Set<IgnitionModule>()): Future[] {
  if (seen.has(ignitionModule)) {
    return [];
  }
  seen.add(ignitionModule);
  return [...ignitionModule.futures, ...[...ignitionModule.submodules].flatMap((sub) => allFutures(sub, seen))];
}

async function knownContract(hre: HardhatRuntimeEnvironment, future: Future): Promise<KnownContract | undefined> {
  switch (future.type) {
    case FutureType.CONTRACT_DEPLOYMENT:
    case FutureType.LIBRARY_DEPLOYMENT:
      return {
        futureId: future.id,
        contractName: future.contractName,
        abi: future.artifact.abi,
        bytecode: future.artifact.bytecode,
      };
    case FutureType.NAMED_ARTIFACT_CONTRACT_DEPLOYMENT:
    case FutureType.NAMED_ARTIFACT_LIBRARY_DEPLOYMENT: {
      const { abi, bytecode } = await hre.artifacts.readArtifact(future.contractName);
      return { futureId: future.id, contractName: future.contractName, abi, bytecode };
    }
    case FutureType.CONTRACT_AT:
      return { futureId: future.id, contractName: future.contractName, abi: future.artifact.abi };
    case FutureType.NAMED_ARTIFACT_CONTRACT_AT:
      return {
        futureId: future.id,
        contractName: future.contractName,
        abi: (await hre.artifacts.readArtifact(future.contractName)).abi,
      };
    default:
      return undefined;
  }
}

/** Bytecode up to the first library placeholder (linked libraries change what follows). */
function unlinkedPrefix(bytecode: string): string {
  const placeholder = bytecode.indexOf("__$");
  return (placeholder === -1 ? bytecode : bytecode.slice(0, placeholder)).toLowerCase();
}

/**
 * Labels the transactions Ignition sends for `ignitionModule` in the signer UI: contract creations by the futures whose
 * bytecode they carry, calls by the function they invoke.
 */
export async function createTransactionLabeler(
  hre: HardhatRuntimeEnvironment,
  ignitionModule: IgnitionModule,
): Promise<(request: SerializedTransactionRequest) => UiTransactionDetails | undefined> {
  const known = (await Promise.all(allFutures(ignitionModule).map((future) => knownContract(hre, future)))).filter(
    (contract): contract is KnownContract => contract !== undefined,
  );
  return ({ to, data = "0x" }) => {
    const calldata = data.toLowerCase();
    if (to === undefined) {
      const created = known.filter(
        ({ bytecode }) =>
          bytecode !== undefined && bytecode.length > 2 && calldata.startsWith(unlinkedPrefix(bytecode)),
      );
      const [first] = created;
      return (
        first && { contractName: first.contractName, notes: `Deploys ${created.map((c) => c.futureId).join(" or ")}` }
      );
    }
    for (const { contractName, abi } of known) {
      try {
        const { functionName } = decodeFunctionData({ abi, data });
        return { contractName, notes: `Calls ${functionName}` };
      } catch {
        // Not a function of this contract.
      }
    }
    return undefined;
  };
}
