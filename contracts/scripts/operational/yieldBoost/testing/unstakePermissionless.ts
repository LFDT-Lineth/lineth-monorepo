import { task } from "hardhat/config";
import { getTaskCliOrEnvValue } from "../../../../common/helpers/environmentHelper";
import {
  fetchBeaconHeader,
  fetchBeaconState,
  fetchBeaconHeaderByParentRoot,
} from "@lidofinance/lsv-cli/dist/utils/fetchCL";
import { createBeaconHeaderProof, createStateProof } from "@lidofinance/lsv-cli/dist/utils/proof/proofs.js";
import { hexlify, AbiCoder } from "ethers";
import * as path from "path";
import { pathToFileURL } from "url";
import * as fs from "fs";

const gIndexPendingPartialWithdrawals = 99n;

/*
  *******************************************************************************************
  Performs YieldManager::unstakePermissionless

  This is for the current version of smart contracts in the repo.

  -------------------------------------------------------------------------------------------
  Example (Hoodi):
  -------------------------------------------------------------------------------------------
  DEPLOYER_PRIVATE_KEY=0000000000000000000000000000000000000000000000000000000000000002 \
  CUSTOM_RPC_URL=https://0xrpc.io/hoodi \
  npx hardhat unstakePermissionless \
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
task("unstakePermissionless", "Performs YieldManager::unstakePermissionless (current contract version)")
  .addOptionalParam("yieldManager")
  .addOptionalParam("yieldProvider")
  .addOptionalParam("validatorIndex")
  .addOptionalParam("slot")
  .addOptionalParam("beaconRpcUrl")
  .setAction(async (taskArgs, hre) => {
    // Get signer
    const { ethers } = hre;
    const [signer] = await ethers.getSigners();

    // --- Resolve inputs from CLI or ENV ---
    const yieldManagerAddress = getTaskCliOrEnvValue(taskArgs, "yieldManager", "YIELD_MANAGER_ADDRESS");
    const yieldProvider = getTaskCliOrEnvValue(taskArgs, "yieldProvider", "YIELD_PROVIDER_ADDRESS");
    const validatorIndexRaw = getTaskCliOrEnvValue(taskArgs, "validatorIndex", "VALIDATOR_INDEX");
    const slotRaw = getTaskCliOrEnvValue(taskArgs, "slot", "SLOT");
    const beaconRpcUrl = getTaskCliOrEnvValue(taskArgs, "beaconRpcUrl", "BEACON_RPC_URL");

    // --- Basic required fields check (adjust as needed) ---
    const missing: string[] = [];
    if (!yieldManagerAddress) missing.push("yieldManager / YIELD_MANAGER_ADDRESS");
    if (!yieldProvider) missing.push("yieldProvider / YIELD_PROVIDER_ADDRESS");
    if (!validatorIndexRaw) missing.push("validatorIndex / VALIDATOR_INDEX");
    if (!slotRaw) missing.push("slot / SLOT");
    if (!beaconRpcUrl) missing.push("beaconRpcUrl / BEACON_RPC_URL");
    if (missing.length) {
      throw new Error(`Missing required params/envs: ${missing.join(", ")}`);
    }

    // --- Parse numeric params ---
    const validatorIndex = parseInt(validatorIndexRaw!, 10);
    const slot = parseInt(slotRaw!, 10);

    // --- Log params ---
    console.log("Params:");
    console.log("  yieldManager:", yieldManagerAddress);
    console.log("  yieldProvider:", yieldProvider);
    console.log("  validatorIndex:", validatorIndex.toString());
    console.log("  slot:", slot.toString());
    console.log("  beaconRpcUrl:", beaconRpcUrl);

    // Resolve path relative to contracts directory (where script is run from)
    const persistentMerkleTreePath = path.resolve(
      process.cwd(),
      "node_modules/@chainsafe/persistent-merkle-tree/lib/proof/index.js",
    );
    // Verify file exists before attempting import
    if (!fs.existsSync(persistentMerkleTreePath)) {
      throw new Error(`Module not found at: ${persistentMerkleTreePath}. Current working directory: ${process.cwd()}`);
    }
    // Convert path to file:// URL using Node's pathToFileURL for proper formatting
    // Use Function constructor to prevent ts-node from statically analyzing the import
    const fileUrl = pathToFileURL(persistentMerkleTreePath).href;
    const importModule = new Function("url", "return import(url)");
    const proofModule = await importModule(fileUrl);
    const { createProof, ProofType } = proofModule;

    // Import @lodestar/types using the same dynamic import pattern
    const lodestarTypes = await importModule("@lodestar/types");

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
      validator,
      view: stateView,
    } = await createStateProof(validatorIndex, stateBodyBytes, forkName);
    const validatorContainerProofConcat = [...validatorStateProof.witnesses, ...beaconHeaderProof.witnesses];
    const validatorContainerProofHex = validatorContainerProofConcat.map((w) => hexlify(w));

    // Pending Partial Withdrawals Proof
    console.log("Using forkName for SSZ schema access:", forkName);
    console.log("Available SSZ forks:", Object.keys(lodestarTypes.ssz));
    // const stateViewDU = lodestarTypes.ssz.fulu.BeaconState.getViewDU(stateView.node);
    const stateViewDU = lodestarTypes.ssz[forkName].BeaconState.getViewDU(stateView.node);
    const { pendingPartialWithdrawals } = stateViewDU;
    const pendingPartialWithdrawalsProof = createProof(stateView.node, {
      type: ProofType.single,
      gindex: gIndexPendingPartialWithdrawals,
    }) as { witnesses: Uint8Array[] };
    const pendingPartialWithdrawalsProofConcat = [
      ...pendingPartialWithdrawalsProof.witnesses,
      ...beaconHeaderProof.witnesses,
    ];
    const pendingPartialWithdrawalsProofHex = pendingPartialWithdrawalsProofConcat.map((w) => hexlify(w));

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
    const withdrawalParams = AbiCoder.defaultAbiCoder().encode(
      ["bytes", "address"],
      [hexlify(validator.pubkey), signer.address],
    );

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

    const withdrawalParamsProof = AbiCoder.defaultAbiCoder().encode([BEACON_PROOF_WITNESS_TYPE], [beaconProofWitness]);

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
    ];

    // Create contract instance
    const yieldManagerContract = await ethers.getContractAt(YIELD_MANAGER_ABI, yieldManagerAddress!, signer);

    // Call function
    console.log("Calling unstakePermissionless...");
    try {
      const tx = await yieldManagerContract.unstakePermissionless(
        yieldProvider!,
        validatorIndex,
        slot,
        withdrawalParams,
        withdrawalParamsProof,
        { value: 1n },
      );
      console.log("Transaction sent, hash:", tx.hash);
      const receipt = await tx.wait();
      console.log("Transaction receipt:", receipt);
      console.log("Transaction successful!");
    } catch (error: unknown) {
      // Extract error data
      if (error && typeof error === "object") {
        const errorObj = error as Record<string, unknown>;
        if (errorObj.data) {
          console.error("Error data:", errorObj.data);
        } else if (errorObj.error && typeof errorObj.error === "object") {
          const nestedError = errorObj.error as Record<string, unknown>;
          if (nestedError.data) {
            console.error("Error data:", nestedError.data);
          }
        }
      }
      throw error;
    }
  });
