import { createProof, ProofType } from "@chainsafe/persistent-merkle-tree";
import {
  fetchBeaconHeader,
  fetchBeaconState,
  fetchBeaconHeaderByParentRoot,
} from "@lidofinance/lsv-cli/dist/utils/fetch-cl.js";
import { createBeaconHeaderProof, createStateProof } from "@lidofinance/lsv-cli/dist/utils/proof/proofs.js";
import { ssz } from "@lodestar/types";
import { BaseError, ContractFunctionRevertedError, bytesToHex } from "viem";

import { parseChecksummedAddress } from "../../../../common/helpers/addressRegistry.js";
import { encodeData } from "../../../../common/helpers/encoding.js";
import { getTaskCliOrEnvValue } from "../../../../common/helpers/environmentHelper.js";
import { waitForSuccessfulReceipt } from "../../../../common/helpers/receipt.js";
import { getUiWalletClient, withSignerUi } from "../../../../hardhat-plugins/signer-ui/session.js";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { TaskArguments } from "hardhat/types/tasks";

const gIndexPendingPartialWithdrawals = 99n;

/** Validator fields read from the SSZ state view (the lsv-cli typing of this view resolves to `never`). */
type ValidatorFields = {
  pubkey: Uint8Array;
  effectiveBalance: number;
  activationEpoch: number;
  activationEligibilityEpoch: number;
};

function isValidatorFields(value: unknown): value is ValidatorFields {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  return (
    "pubkey" in value &&
    value.pubkey instanceof Uint8Array &&
    "effectiveBalance" in value &&
    "activationEpoch" in value &&
    "activationEligibilityEpoch" in value
  );
}

/*
  *******************************************************************************************
  Performs YieldManager::unstakePermissionless

  This is for the current version of smart contracts in the repo.

  -------------------------------------------------------------------------------------------
  Example (Hoodi):
  -------------------------------------------------------------------------------------------
  DEPLOYER_PRIVATE_KEY=0000000000000000000000000000000000000000000000000000000000000002 \
  CUSTOM_RPC_URL=https://0xrpc.io/hoodi \
  pnpm exec hardhat unstakePermissionless \
    --yield-manager <address> \
    --yield-provider <address> \
    --validator-index <uint64> \
    --slot <uint64> \
    --beacon-rpc-url <string> \
    --network custom
  -------------------------------------------------------------------------------------------

  Env var alternatives (used if CLI params omitted):
    YIELD_MANAGER_ADDRESS
    YIELD_PROVIDER_ADDRESS
    VALIDATOR_INDEX
    SLOT
    BEACON_RPC_URL
  
  Note: The contract calculates the unstake amount internally based on the withdrawal reserve deficit.
  The pendingPartialWithdrawalsWitness proof needs to be provided manually in the code.
  *******************************************************************************************
*/
export default async function unstakePermissionless(taskArgs: TaskArguments, hre: HardhatRuntimeEnvironment) {
  const connection = await hre.network.getOrCreate();
  return withSignerUi(hre, "task:unstakePermissionless", async () => {
    const { viem } = connection;
    const wallet = await getUiWalletClient(connection);
    const publicClient = await viem.getPublicClient();
    const signerAddress = wallet.account.address;

    // --- Resolve inputs from CLI or ENV ---
    const yieldManagerInput = getTaskCliOrEnvValue(taskArgs, "yieldManager", "YIELD_MANAGER_ADDRESS");
    const yieldProviderInput = getTaskCliOrEnvValue(taskArgs, "yieldProvider", "YIELD_PROVIDER_ADDRESS");
    const validatorIndexRaw = getTaskCliOrEnvValue(taskArgs, "validatorIndex", "VALIDATOR_INDEX");
    const slotRaw = getTaskCliOrEnvValue(taskArgs, "slot", "SLOT");
    const beaconRpcUrl = getTaskCliOrEnvValue(taskArgs, "beaconRpcUrl", "BEACON_RPC_URL");

    // --- Basic required fields check (adjust as needed) ---
    const missing: string[] = [];
    if (!yieldManagerInput) missing.push("yieldManager / YIELD_MANAGER_ADDRESS");
    if (!yieldProviderInput) missing.push("yieldProvider / YIELD_PROVIDER_ADDRESS");
    if (!validatorIndexRaw) missing.push("validatorIndex / VALIDATOR_INDEX");
    if (!slotRaw) missing.push("slot / SLOT");
    if (!beaconRpcUrl) missing.push("beaconRpcUrl / BEACON_RPC_URL");
    if (
      missing.length ||
      !yieldManagerInput ||
      !yieldProviderInput ||
      !validatorIndexRaw ||
      !slotRaw ||
      !beaconRpcUrl
    ) {
      throw new Error(`Missing required params/envs: ${missing.join(", ")}`);
    }
    const yieldManagerAddress = parseChecksummedAddress(yieldManagerInput, "--yield-manager / YIELD_MANAGER_ADDRESS");
    const yieldProvider = parseChecksummedAddress(yieldProviderInput, "--yield-provider / YIELD_PROVIDER_ADDRESS");

    // --- Parse numeric params ---
    const validatorIndex = parseInt(validatorIndexRaw, 10);
    const slot = parseInt(slotRaw, 10);

    // --- Log params ---
    console.log("Params:");
    console.log("  yieldManager:", yieldManagerAddress);
    console.log("  yieldProvider:", yieldProvider);
    console.log("  validatorIndex:", validatorIndex.toString());
    console.log("  slot:", slot.toString());
    console.log("  beaconRpcUrl:", beaconRpcUrl);

    // Get beacon state
    const beaconHeaderJson = await fetchBeaconHeader(slot, beaconRpcUrl);
    const beaconHeader = beaconHeaderJson.data.header.message;
    console.log("beaconHeader:", beaconHeader);
    const { stateBodyBytes, forkName } = await fetchBeaconState(slot, beaconRpcUrl);
    console.log("forkName:", forkName);

    // Validator Container Proof
    const { proof: beaconHeaderProof, root: beaconHeaderRoot } = await createBeaconHeaderProof(beaconHeader);
    const {
      proof: validatorStateProof,
      validator: validatorView,
      view: stateView,
    } = await createStateProof(validatorIndex, stateBodyBytes, forkName);
    const validator: unknown = validatorView;
    if (!isValidatorFields(validator)) {
      throw new Error(`Unexpected validator view shape for validator index ${validatorIndex}`);
    }
    const validatorContainerProofConcat = [...validatorStateProof.witnesses, ...beaconHeaderProof.witnesses];
    const validatorContainerProofHex = validatorContainerProofConcat.map((w) => bytesToHex(w));

    // Pending Partial Withdrawals Proof
    console.log("Using forkName for SSZ schema access:", forkName);
    console.log("Available SSZ forks:", Object.keys(ssz));
    const stateViewDU = ssz[forkName].BeaconState.getViewDU(stateView.node);
    if (!("pendingPartialWithdrawals" in stateViewDU)) {
      throw new Error(`Fork ${forkName} has no pendingPartialWithdrawals (requires Electra or later)`);
    }
    const { pendingPartialWithdrawals } = stateViewDU;
    const pendingPartialWithdrawalsProof = createProof(stateView.node, {
      type: ProofType.single,
      gindex: gIndexPendingPartialWithdrawals,
    });
    if (pendingPartialWithdrawalsProof.type !== ProofType.single) {
      throw new Error("Expected a single proof for pendingPartialWithdrawals");
    }
    const pendingPartialWithdrawalsProofConcat = [
      ...pendingPartialWithdrawalsProof.witnesses,
      ...beaconHeaderProof.witnesses,
    ];
    const pendingPartialWithdrawalsProofHex = pendingPartialWithdrawalsProofConcat.map((w) => bytesToHex(w));

    // Fill out Validator Witness fields
    const headerByParentJson = await fetchBeaconHeaderByParentRoot(beaconHeaderRoot, beaconRpcUrl);
    if (!headerByParentJson.data || !Array.isArray(headerByParentJson.data) || headerByParentJson.data.length === 0) {
      throw new Error(
        `No beacon headers found for parent root ${beaconHeaderRoot} at ${beaconRpcUrl}. ` +
          `This may occur during chain reorganization, timing issues, or if the parent root doesn't exist. ` +
          `Please verify the parent root and try again.`,
      );
    }
    const headerByParentSlot = headerByParentJson.data[0].header.message.slot;
    const headerByParentTimestamp = stateView.genesisTime + headerByParentSlot * 12;

    // Encode withdrawal params - only pubkeys and refundRecipient (no amounts array)
    const withdrawalParams = encodeData(["bytes", "address"], [bytesToHex(validator.pubkey), signerAddress]);

    // Build BeaconProofWitness structure
    // struct ValidatorContainerWitness {
    //   bytes32[] proof;
    //   uint64 effectiveBalance;
    //   uint64 activationEpoch;
    //   uint64 activationEligibilityEpoch;
    // }
    const validatorContainerWitness = {
      proof: validatorContainerProofHex,
      effectiveBalance: BigInt(validator.effectiveBalance),
      activationEpoch: BigInt(validator.activationEpoch),
      activationEligibilityEpoch: BigInt(validator.activationEligibilityEpoch),
    };

    // struct PendingPartialWithdrawalsWitness {
    //   bytes32[] proof;
    //   PendingPartialWithdrawal[] pendingPartialWithdrawals;
    // }
    const pendingPartialWithdrawalsWitness = {
      proof: pendingPartialWithdrawalsProofHex,
      pendingPartialWithdrawals: pendingPartialWithdrawals.toValue(),
    };

    // struct BeaconProofWitness {
    //   uint64 childBlockTimestamp;
    //   uint64 proposerIndex;
    //   ValidatorContainerWitness validatorContainerWitness;
    //   PendingPartialWithdrawalsWitness pendingPartialWithdrawalsWitness;
    // }
    const beaconProofWitness = {
      childBlockTimestamp: BigInt(headerByParentTimestamp),
      proposerIndex: BigInt(beaconHeader.proposer_index),
      validatorContainerWitness: validatorContainerWitness,
      pendingPartialWithdrawalsWitness: pendingPartialWithdrawalsWitness,
    };

    console.log("beaconProofWitness:", beaconProofWitness);

    const VALIDATOR_CONTAINER_WITNESS_TYPE =
      "tuple(bytes32[] proof, uint64 effectiveBalance, uint64 activationEpoch, uint64 activationEligibilityEpoch)";
    // PendingPartialWithdrawal struct: (uint64 validatorIndex, uint64 amount, uint64 withdrawableEpoch)
    const PENDING_PARTIAL_WITHDRAWALS_WITNESS_TYPE =
      "tuple(bytes32[] proof, tuple(uint64 validatorIndex, uint64 amount, uint64 withdrawableEpoch)[] pendingPartialWithdrawals)";
    const BEACON_PROOF_WITNESS_TYPE = `tuple(uint64 childBlockTimestamp, uint64 proposerIndex, ${VALIDATOR_CONTAINER_WITNESS_TYPE} validatorContainerWitness, ${PENDING_PARTIAL_WITHDRAWALS_WITNESS_TYPE} pendingPartialWithdrawalsWitness)`;

    const withdrawalParamsProof = encodeData([BEACON_PROOF_WITNESS_TYPE], [beaconProofWitness]);

    // Define minimal ABI for YieldManager
    const YIELD_MANAGER_ABI = [
      {
        inputs: [
          { internalType: "address", name: "_yieldProvider", type: "address" },
          { internalType: "uint64", name: "_validatorIndex", type: "uint64" },
          { internalType: "uint64", name: "_slot", type: "uint64" },
          { internalType: "bytes", name: "_withdrawalParams", type: "bytes" },
          { internalType: "bytes", name: "_withdrawalParamsProof", type: "bytes" },
        ],
        name: "unstakePermissionless",
        outputs: [],
        stateMutability: "payable",
        type: "function",
      },
    ] as const;

    // Call function
    console.log("Calling unstakePermissionless...");
    try {
      const hash = await wallet.writeContract({
        address: yieldManagerAddress,
        abi: YIELD_MANAGER_ABI,
        functionName: "unstakePermissionless",
        args: [yieldProvider, BigInt(validatorIndex), BigInt(slot), withdrawalParams, withdrawalParamsProof],
        value: 1n,
      });
      console.log("Transaction sent, hash:", hash);
      const receipt = await waitForSuccessfulReceipt(publicClient, hash);
      console.log("Transaction receipt:", receipt);
      console.log("Transaction successful!");
    } catch (error: unknown) {
      const revert =
        error instanceof BaseError ? error.walk((cause) => cause instanceof ContractFunctionRevertedError) : null;
      if (revert instanceof ContractFunctionRevertedError) {
        console.error("Revert data:", revert.raw ?? revert.data ?? revert.reason);
      }
      throw error;
    }
  });
}
