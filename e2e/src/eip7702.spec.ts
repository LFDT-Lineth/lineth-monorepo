import { beforeAll, describe, expect, it } from "@jest/globals";
import { encodeFunctionData, getAddress, isAddress } from "viem";

import {
  addToDenyList,
  expectBlockedTransaction,
  reloadDenyList,
  removeFromDenyList,
  withDenyListAddresses,
} from "./common/test-helpers";
import { estimateLineaGas, expectSuccessfulTransaction, sendTransactionWithRetry } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";
import { TestEIP7702DelegationAbi, TestEIP7702DelegationAbiBytecode } from "./generated";

const context = createTestContext();
const l2AccountManager = context.getL2AccountManager();

enum DelegationScenarioType {
  DenylistedAuthority,
  DenylistedContract,
}

type DelegationScenario = {
  denyListAddress: `0x${string}`;
  sendDelegatedInitializeTx: () => Promise<`0x${string}`>;
};

type CreateDelegationScenarioParams = {
  scenarioType: DelegationScenarioType;
  contractAddress: `0x${string}`;
};

// deny-list.txt is a shared file modified by this suite. Denylist tests below
// MUST NOT use it.concurrent() because removeFromDenyList uses read-modify-write.
describe("EIP-7702 test suite", () => {
  const l2PublicClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });
  const sequencerClient = context.l2PublicClient({ type: L2RpcEndpoint.Sequencer });
  const initializeData = encodeFunctionData({
    abi: TestEIP7702DelegationAbi,
    functionName: "initialize",
  });

  let targetContractAddress: `0x${string}`;

  async function deployDelegationContract(deployer: { address: `0x${string}` }): Promise<`0x${string}`> {
    const deployerWalletClient = context.l2WalletClient({ account: deployer });
    const deployNonce = await l2PublicClient.getTransactionCount({ address: deployer.address });

    const deployEstimate = await estimateLineaGas(l2PublicClient, {
      account: deployer,
      data: TestEIP7702DelegationAbiBytecode,
    });

    const { receipt: deployReceipt } = await sendTransactionWithRetry(l2PublicClient, (fees) =>
      deployerWalletClient.sendTransaction({
        data: TestEIP7702DelegationAbiBytecode,
        nonce: deployNonce,
        ...deployEstimate,
        ...fees,
      }),
    );

    expect(deployReceipt.status).toEqual("success");
    expect(deployReceipt.contractAddress).toBeDefined();
    expect(isAddress(deployReceipt.contractAddress as `0x${string}`)).toBe(true);

    const deployedContractAddress = getAddress(deployReceipt.contractAddress as `0x${string}`);
    logger.debug(`TestEIP7702Delegation deployed. address=${deployedContractAddress}`);
    return deployedContractAddress;
  }

  async function createDelegationScenario(params: CreateDelegationScenarioParams): Promise<DelegationScenario> {
    const { scenarioType, contractAddress } = params;
    const isDenylistedAuthorityCase = scenarioType === DelegationScenarioType.DenylistedAuthority;
    const [authority] = await l2AccountManager.generateAccounts(1);
    const [sponsor] = isDenylistedAuthorityCase ? await l2AccountManager.generateAccounts(1) : [authority];
    const authorityWalletClient = context.l2WalletClient({ account: authority });
    const sponsorWalletClient = context.l2WalletClient({ type: L2RpcEndpoint.Sequencer, account: sponsor });
    const authorization = await authorityWalletClient.signAuthorization({
      contractAddress,
      // Self-sponsored tx if target address is denylisted
      executor: isDenylistedAuthorityCase ? undefined : "self",
    });
    const { maxFeePerGas, maxPriorityFeePerGas } = await estimateLineaGas(l2PublicClient, {
      account: sponsor,
      to: contractAddress,
      data: initializeData,
    });

    return {
      denyListAddress: isDenylistedAuthorityCase ? authority.address : contractAddress,
      sendDelegatedInitializeTx: async () => {
        const nonce = await sequencerClient.getTransactionCount({ address: sponsor.address });

        return sponsorWalletClient.sendTransaction({
          authorizationList: [authorization],
          to: authority.address,
          data: initializeData,
          nonce,
          gas: 100_000n,
          maxFeePerGas,
          maxPriorityFeePerGas,
        });
      },
    };
  }

  beforeAll(async () => {
    const [deployer] = await l2AccountManager.generateAccounts(1);
    targetContractAddress = await deployDelegationContract(deployer);
  }, 120_000);

  it.concurrent("should execute EIP-7702 (Set Code) transactions", async () => {
    const [deployer] = await l2AccountManager.generateAccounts(1);
    // Keep this deployment test-local to avoid state coupling with shared beforeAll contract in concurrent execution.
    const testTargetContractAddress = await deployDelegationContract(deployer);
    // Not a denylist test, but reuse createDelegationScenario fn
    const scenario = await createDelegationScenario({
      scenarioType: DelegationScenarioType.DenylistedContract,
      contractAddress: testTargetContractAddress,
    });

    await expectSuccessfulTransaction(l2PublicClient, scenario.sendDelegatedInitializeTx());
  });

  it("should block EIP-7702 tx when authorization_list authority is denylisted", async () => {
    const scenario = await createDelegationScenario({
      scenarioType: DelegationScenarioType.DenylistedAuthority,
      contractAddress: targetContractAddress,
    });

    await withDenyListAddresses(sequencerClient, [scenario.denyListAddress], async () => {
      logger.debug(`Authority address added to deny list. address=${scenario.denyListAddress}`);
      const sendTransactionPromise = scenario.sendDelegatedInitializeTx();

      await expectBlockedTransaction(sendTransactionPromise);
      logger.debug("EIP-7702 transaction correctly rejected for denied authority.");
    });

    logger.debug("Authority address removed from deny list.");
  }, 120_000);

  it("should block EIP-7702 tx when authorization_list delegates to denylisted contract", async () => {
    const scenario = await createDelegationScenario({
      scenarioType: DelegationScenarioType.DenylistedContract,
      contractAddress: targetContractAddress,
    });
    await withDenyListAddresses(sequencerClient, [scenario.denyListAddress], async () => {
      logger.debug(`Contract address added to deny list. address=${scenario.denyListAddress}`);
      const sendTransactionPromise = scenario.sendDelegatedInitializeTx();

      await expectBlockedTransaction(sendTransactionPromise);
      logger.debug("EIP-7702 transaction correctly rejected for denied contract delegation.");
    });

    logger.debug("Contract address removed from deny list.");
  }, 120_000);

  it("should block EIP-7702 tx after non-denied authority is added to denylist and plugin config is reloaded", async () => {
    const scenario = await createDelegationScenario({
      scenarioType: DelegationScenarioType.DenylistedAuthority,
      contractAddress: targetContractAddress,
    });

    try {
      await expectSuccessfulTransaction(l2PublicClient, scenario.sendDelegatedInitializeTx());

      addToDenyList([scenario.denyListAddress]);
      await reloadDenyList(sequencerClient);

      await expectBlockedTransaction(scenario.sendDelegatedInitializeTx());
    } finally {
      removeFromDenyList([scenario.denyListAddress]);
      await reloadDenyList(sequencerClient);
    }
  }, 120_000);
});
