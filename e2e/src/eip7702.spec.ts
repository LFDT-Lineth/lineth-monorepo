import { describe, it } from "@jest/globals";
import { PrivateKeyAccount } from "viem";

import { expectBlockedTransaction, withDenyListAddresses } from "./common/test-helpers";
import { encodeEip7702InitializeData, deployTestEip7702Delegation } from "./common/test-helpers/eip7702-delegation";
import { estimateLineaGas, expectSuccessfulTransaction } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";

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

describe("EIP-7702 test suite", () => {
  const l2PublicClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });
  const sequencerClient = context.l2PublicClient({ type: L2RpcEndpoint.Sequencer });
  const initializeData = encodeEip7702InitializeData();

  async function deployDelegationContract(deployer: PrivateKeyAccount): Promise<`0x${string}`> {
    const deployerWalletClient = context.l2WalletClient({ account: deployer });
    const deployedContractAddress = await deployTestEip7702Delegation(l2PublicClient, deployerWalletClient, deployer);
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

  it.concurrent("should execute EIP-7702 (Set Code) transactions", async () => {
    const [deployer] = await l2AccountManager.generateAccounts(1);
    const contractAddress = await deployDelegationContract(deployer);
    const scenario = await createDelegationScenario({
      scenarioType: DelegationScenarioType.DenylistedContract,
      contractAddress,
    });

    await expectSuccessfulTransaction(l2PublicClient, scenario.sendDelegatedInitializeTx());
  });

  it.concurrent("should execute EIP-7702 self-call with no calldata when delegating to a codeless EOA", async () => {
    const [accountA, accountB] = await l2AccountManager.generateAccounts(2);
    const accountAWalletClient = context.l2WalletClient({ account: accountA });

    const authorization = await accountAWalletClient.signAuthorization({
      contractAddress: accountB.address,
      executor: "self",
    });

    const { maxFeePerGas, maxPriorityFeePerGas } = await estimateLineaGas(l2PublicClient, {
      account: accountA,
      to: accountA.address,
      data: "0x",
    });

    const nonce = await l2PublicClient.getTransactionCount({ address: accountA.address });

    await expectSuccessfulTransaction(
      l2PublicClient,
      accountAWalletClient.sendTransaction({
        authorizationList: [authorization],
        to: accountA.address,
        data: "0x",
        nonce,
        gas: 100_000n,
        maxFeePerGas,
        maxPriorityFeePerGas,
      }),
    );
  });

  it.concurrent(
    "should block EIP-7702 tx when authorization_list authority is denylisted",
    async () => {
      const [deployer] = await l2AccountManager.generateAccounts(1);
      const contractAddress = await deployDelegationContract(deployer);
      const scenario = await createDelegationScenario({
        scenarioType: DelegationScenarioType.DenylistedAuthority,
        contractAddress,
      });

      await withDenyListAddresses(sequencerClient, [scenario.denyListAddress], async () => {
        logger.debug(`Authority address added to deny list. address=${scenario.denyListAddress}`);
        const sendTransactionPromise = scenario.sendDelegatedInitializeTx();

        await expectBlockedTransaction(sendTransactionPromise);
        logger.debug("EIP-7702 transaction correctly rejected for denied authority.");
      });

      logger.debug("Authority address removed from deny list.");
    },
    120_000,
  );

  it.concurrent(
    "should block EIP-7702 tx when authorization_list delegates to denylisted contract",
    async () => {
      const [deployer] = await l2AccountManager.generateAccounts(1);
      const contractAddress = await deployDelegationContract(deployer);
      const scenario = await createDelegationScenario({
        scenarioType: DelegationScenarioType.DenylistedContract,
        contractAddress,
      });
      await withDenyListAddresses(sequencerClient, [scenario.denyListAddress], async () => {
        logger.debug(`Contract address added to deny list. address=${scenario.denyListAddress}`);
        const sendTransactionPromise = scenario.sendDelegatedInitializeTx();

        await expectBlockedTransaction(sendTransactionPromise);
        logger.debug("EIP-7702 transaction correctly rejected for denied contract delegation.");
      });

      logger.debug("Contract address removed from deny list.");
    },
    120_000,
  );

  it.concurrent(
    "should block EIP-7702 tx after non-denied authority is added to denylist and plugin config is reloaded",
    async () => {
      const [deployer] = await l2AccountManager.generateAccounts(1);
      const contractAddress = await deployDelegationContract(deployer);
      const scenario = await createDelegationScenario({
        scenarioType: DelegationScenarioType.DenylistedAuthority,
        contractAddress,
      });

      await expectSuccessfulTransaction(l2PublicClient, scenario.sendDelegatedInitializeTx());

      await withDenyListAddresses(sequencerClient, [scenario.denyListAddress], async () => {
        await expectBlockedTransaction(scenario.sendDelegatedInitializeTx());
      });
    },
    120_000,
  );
});
