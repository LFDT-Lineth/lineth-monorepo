import { etherToWei } from "@consensys/linea-shared-utils";
import { describe, expect, it } from "@jest/globals";
import { type Address, type Hash, encodeDeployData, encodeFunctionData } from "viem";

import { deployContract } from "./common/deployments";
import {
  getL1SecurityCouncilAccount,
  withDenyListAddresses,
  withOnChainFilteredAddresses,
} from "./common/test-helpers";
import {
  buildSignedForcedTransaction,
  getDefaultLastFinalizedTimestamp,
  resolveLastFinalizedState,
} from "./common/test-helpers/forced-transactions";
import { estimateLineaGas, getEvents, sendTransactionWithRetry, waitForEvents } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";
import {
  ExcludedPrecompilesAbi,
  ExcludedPrecompilesAbiBytecode,
  MultiMessageSenderAbi,
  MultiMessageSenderAbiBytecode,
} from "./generated";

const context = createTestContext();
const l1AccountManager = context.getL1AccountManager();
const l2AccountManager = context.getL2AccountManager();

async function expectReceiptNotFound(
  client: { getTransactionReceipt: (args: { hash: Hash }) => Promise<unknown> },
  hash: Hash,
): Promise<void> {
  await expect(client.getTransactionReceipt({ hash })).rejects.toMatchObject({
    name: "TransactionReceiptNotFoundError",
  });
}

describe("Forced transaction test suite", () => {
  it.skip(
    "Should successfully submit a forced transaction containing a valid l2 transaction",
    async () => {
      const [l1Account, l2Account] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const l1PublicClient = context.l1PublicClient();
      const l1WalletClient = context.l1WalletClient({ account: l1Account });

      const lineaRollup = context.l1Contracts.lineaRollup(l1PublicClient);
      const gateway = context.l1Contracts.forcedTransactionGateway(l1WalletClient);
      const gatewayRead = context.l1Contracts.forcedTransactionGateway(l1PublicClient);

      const destinationChainId = await gatewayRead.read.DESTINATION_CHAIN_ID();
      logger.debug(`Gateway config — destinationChainId=${destinationChainId}`);

      let lastFinalizedState = await resolveLastFinalizedState(
        lineaRollup,
        l1PublicClient,
        getDefaultLastFinalizedTimestamp(),
      );

      logger.debug(
        `Resolved finalized state — timestamp=${lastFinalizedState.timestamp} messageNumber=${lastFinalizedState.messageNumber} messageRollingHash=${lastFinalizedState.messageRollingHash} forcedTransactionNumber=${lastFinalizedState.forcedTransactionNumber} forcedTransactionRollingHash=${lastFinalizedState.forcedTransactionRollingHash}`,
      );

      const { forcedTransaction, l2TxHash } = await buildSignedForcedTransaction(context, {
        l2Account,
        to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9" as Address,
        nonce: 0n,
        value: etherToWei("0.1"),
        gasLimit: 21_000n,
        maxFeePerGas: 1_000_000_000n,
        maxPriorityFeePerGas: 100_000_000n,
      });

      logger.debug(
        `Built forced transaction — signer=${l2Account.address} to=${forcedTransaction.to} gasLimit=${forcedTransaction.gasLimit} l2TxHash=${l2TxHash}`,
      );

      const [, , , , feeAmount] = await lineaRollup.read.getRequiredForcedTransactionFields();
      const { maxPriorityFeePerGas, maxFeePerGas } = await l1PublicClient.estimateFeesPerGas();

      const { hash: txHash, receipt } = await sendTransactionWithRetry(
        l1PublicClient,
        (fees) =>
          gateway.write.submitForcedTransaction([forcedTransaction, lastFinalizedState], {
            value: feeAmount,
            maxPriorityFeePerGas,
            maxFeePerGas,
            ...fees,
          }),
        {
          receiptTimeoutMs: 30_000,
          abi: [...gateway.abi, ...lineaRollup.abi],
          retryOnRevert: true,
          beforeRetry: async () => {
            lastFinalizedState = await resolveLastFinalizedState(
              lineaRollup,
              l1PublicClient,
              getDefaultLastFinalizedTimestamp(),
            );
          },
        },
      );

      logger.debug(
        `submitForcedTransaction confirmed. txHash=${txHash} status=${receipt.status} blockNumber=${receipt.blockNumber}`,
      );
      expect(receipt.status).toEqual("success");

      const [forcedTxEvent] = await getEvents(l1PublicClient, {
        abi: lineaRollup.abi,
        address: lineaRollup.address,
        eventName: "ForcedTransactionAdded",
        fromBlock: receipt.blockNumber,
        toBlock: receipt.blockNumber,
        strict: true,
      });

      expect(forcedTxEvent).toBeDefined();
      logger.debug(
        `ForcedTransactionAdded — forcedTransactionNumber=${forcedTxEvent.args.forcedTransactionNumber} from=${forcedTxEvent.args.from} blockNumberDeadline=${forcedTxEvent.args.blockNumberDeadline} forcedTransactionRollingHash=${forcedTxEvent.args.forcedTransactionRollingHash}`,
      );
      const submittedForcedTransactionNumber = forcedTxEvent.args.forcedTransactionNumber;

      const l2PublicClient = context.l2PublicClient();
      logger.debug(`Waiting for forced transaction receipt on L2. l2TxHash=${l2TxHash}`);

      const l2Receipt = await l2PublicClient.waitForTransactionReceipt({ hash: l2TxHash, timeout: 120_000 });
      expect(l2Receipt.status).toEqual("success");

      logger.debug(
        `Forced transaction executed on L2. l2TxHash=${l2TxHash} blockNumber=${l2Receipt.blockNumber} status=${l2Receipt.status}`,
      );

      logger.debug(
        `Waiting for FinalizedStateUpdated with forcedTransactionNumber >= ${submittedForcedTransactionNumber}`,
      );

      const [finalizedEvent] = await waitForEvents(l1PublicClient, {
        abi: lineaRollup.abi,
        address: lineaRollup.address,
        eventName: "FinalizedStateUpdated",
        fromBlock: receipt.blockNumber,
        toBlock: "latest",
        pollingIntervalMs: 2_000,
        strict: true,
        timeoutMs: 200_000,
        criteria: async (events) =>
          events.filter((e) => e.args.forcedTransactionNumber >= submittedForcedTransactionNumber),
      });

      logger.debug(
        `Finalization includes forced transaction. blockNumber=${finalizedEvent.args.blockNumber} forcedTransactionNumber=${finalizedEvent.args.forcedTransactionNumber}`,
      );
    },
    200_000,
  );

  it.skip(
    "Should successfully submit a forced transaction containing an invalid L2 tx.",
    async () => {
      const [l1Account, l2Account] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const l1PublicClient = context.l1PublicClient();
      const l1WalletClient = context.l1WalletClient({ account: l1Account });

      const lineaRollup = context.l1Contracts.lineaRollup(l1PublicClient);
      const gateway = context.l1Contracts.forcedTransactionGateway(l1WalletClient);
      const gatewayRead = context.l1Contracts.forcedTransactionGateway(l1PublicClient);

      const destinationChainId = await gatewayRead.read.DESTINATION_CHAIN_ID();
      logger.debug(`Gateway config — destinationChainId=${destinationChainId}`);

      let lastFinalizedState = await resolveLastFinalizedState(
        lineaRollup,
        l1PublicClient,
        getDefaultLastFinalizedTimestamp(),
      );

      logger.debug(
        `Resolved finalized state — timestamp=${lastFinalizedState.timestamp} messageNumber=${lastFinalizedState.messageNumber} messageRollingHash=${lastFinalizedState.messageRollingHash} forcedTransactionNumber=${lastFinalizedState.forcedTransactionNumber} forcedTransactionRollingHash=${lastFinalizedState.forcedTransactionRollingHash}`,
      );

      const { forcedTransaction, l2TxHash } = await buildSignedForcedTransaction(context, {
        l2Account,
        to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9" as Address,
        nonce: 1_000_000n, // HIGH NONCE TO ENSURE THE L2 TX REVERTS ON L2.
        value: etherToWei("0.1"),
        gasLimit: 21_000n,
        maxFeePerGas: 1_000_000_000n,
        maxPriorityFeePerGas: 100_000_000n,
      });

      logger.debug(
        `Built forced transaction — signer=${l2Account.address} to=${forcedTransaction.to} gasLimit=${forcedTransaction.gasLimit} l2TxHash=${l2TxHash}`,
      );

      const [, , , , feeAmount] = await lineaRollup.read.getRequiredForcedTransactionFields();
      const { maxPriorityFeePerGas, maxFeePerGas } = await l1PublicClient.estimateFeesPerGas();

      const { hash: txHash, receipt } = await sendTransactionWithRetry(
        l1PublicClient,
        (fees) =>
          gateway.write.submitForcedTransaction([forcedTransaction, lastFinalizedState], {
            value: feeAmount,
            maxPriorityFeePerGas,
            maxFeePerGas,
            ...fees,
          }),
        {
          receiptTimeoutMs: 30_000,
          abi: [...gateway.abi, ...lineaRollup.abi],
          retryOnRevert: true,
          beforeRetry: async () => {
            lastFinalizedState = await resolveLastFinalizedState(
              lineaRollup,
              l1PublicClient,
              getDefaultLastFinalizedTimestamp(),
            );
          },
        },
      );

      logger.debug(
        `submitForcedTransaction confirmed. txHash=${txHash} status=${receipt.status} blockNumber=${receipt.blockNumber}`,
      );
      expect(receipt.status).toEqual("success");

      const [forcedTxEvent] = await getEvents(l1PublicClient, {
        abi: lineaRollup.abi,
        address: lineaRollup.address,
        eventName: "ForcedTransactionAdded",
        fromBlock: receipt.blockNumber,
        toBlock: receipt.blockNumber,
        strict: true,
      });

      expect(forcedTxEvent).toBeDefined();
      logger.debug(
        `ForcedTransactionAdded — forcedTransactionNumber=${forcedTxEvent.args.forcedTransactionNumber} from=${forcedTxEvent.args.from} blockNumberDeadline=${forcedTxEvent.args.blockNumberDeadline} forcedTransactionRollingHash=${forcedTxEvent.args.forcedTransactionRollingHash}`,
      );

      const submittedForcedTransactionNumber = forcedTxEvent.args.forcedTransactionNumber;
      logger.debug(
        `Waiting for FinalizedStateUpdated with forcedTransactionNumber >= ${submittedForcedTransactionNumber}`,
      );

      const [finalizedEvent] = await waitForEvents(l1PublicClient, {
        abi: lineaRollup.abi,
        address: lineaRollup.address,
        eventName: "FinalizedStateUpdated",
        fromBlock: receipt.blockNumber,
        toBlock: "latest",
        pollingIntervalMs: 2_000,
        timeoutMs: 200_000,
        strict: true,
        criteria: async (events) =>
          events.filter((e) => e.args.forcedTransactionNumber >= submittedForcedTransactionNumber),
      });

      logger.debug(
        `Finalization includes forced transaction. blockNumber=${finalizedEvent.args.blockNumber} forcedTransactionNumber=${finalizedEvent.args.forcedTransactionNumber}`,
      );

      const l2PublicClient = context.l2PublicClient();
      logger.debug(`Verifying that the forced transaction receipt is not available on L2 for l2TxHash=${l2TxHash}`);
      await expectReceiptNotFound(l2PublicClient, l2TxHash);
      logger.debug("Checked for forced transaction receipt on L2; confirmed that receipt was not found as expected.");
    },
    200_000,
  );

  it.skip(
    "Should reject a forced transaction that calls an excluded precompile (BadPrecompile)",
    async () => {
      const [l1Account, l2Deployer, l2ForcedAccount] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const l1PublicClient = context.l1PublicClient();
      const l1WalletClient = context.l1WalletClient({ account: l1Account });
      const l2PublicClient = context.l2PublicClient();
      const l2DeployerWalletClient = context.l2WalletClient({ account: l2Deployer });

      // Deploy ExcludedPrecompiles contract on L2
      const excludedPrecompilesDeployFees = await estimateLineaGas(l2PublicClient, {
        account: l2Deployer.address,
        data: encodeDeployData({
          abi: ExcludedPrecompilesAbi,
          bytecode: ExcludedPrecompilesAbiBytecode as `0x${string}`,
        }),
        value: 0n,
      });
      const contractAddress = await deployContract(l2DeployerWalletClient, {
        account: l2Deployer,
        chain: l2DeployerWalletClient.chain,
        abi: ExcludedPrecompilesAbi,
        bytecode: ExcludedPrecompilesAbiBytecode as `0x${string}`,
        maxFeePerGas: excludedPrecompilesDeployFees.maxFeePerGas,
        maxPriorityFeePerGas: excludedPrecompilesDeployFees.maxPriorityFeePerGas,
      });

      logger.debug(`ExcludedPrecompiles deployed on L2. address=${contractAddress}`);

      // Encode calldata for callRIPEMD160
      const callData = encodeFunctionData({
        abi: ExcludedPrecompilesAbi,
        functionName: "callRIPEMD160",
        args: ["0x68656c6c6f"], // "hello" in hex
      });

      const lineaRollup = context.l1Contracts.lineaRollup(l1PublicClient);
      const gateway = context.l1Contracts.forcedTransactionGateway(l1WalletClient);

      // Resolve finalized state
      let lastFinalizedState = await resolveLastFinalizedState(
        lineaRollup,
        l1PublicClient,
        getDefaultLastFinalizedTimestamp(),
      );

      logger.debug(
        `Resolved finalized state — timestamp=${lastFinalizedState.timestamp} forcedTransactionNumber=${lastFinalizedState.forcedTransactionNumber}`,
      );

      // Build forced transaction calling the excluded precompile
      const { forcedTransaction, l2TxHash } = await buildSignedForcedTransaction(context, {
        l2Account: l2ForcedAccount,
        to: contractAddress as Address,
        nonce: 0n,
        value: 0n,
        data: callData,
        gasLimit: 300_000n,
        maxFeePerGas: 1_000_000_000n,
        maxPriorityFeePerGas: 100_000_000n,
      });

      logger.debug(
        `Built BadPrecompile forced transaction — signer=${l2ForcedAccount.address} to=${contractAddress} l2TxHash=${l2TxHash}`,
      );

      const [, , , , feeAmount] = await lineaRollup.read.getRequiredForcedTransactionFields();
      const { maxPriorityFeePerGas, maxFeePerGas } = await l1PublicClient.estimateFeesPerGas();

      // Submit the forced transaction
      const { hash: txHash, receipt } = await sendTransactionWithRetry(
        l1PublicClient,
        (fees) =>
          gateway.write.submitForcedTransaction([forcedTransaction, lastFinalizedState], {
            value: feeAmount,
            maxPriorityFeePerGas,
            maxFeePerGas,
            ...fees,
          }),
        {
          receiptTimeoutMs: 30_000,
          abi: [...gateway.abi, ...lineaRollup.abi],
          retryOnRevert: true,
          beforeRetry: async () => {
            lastFinalizedState = await resolveLastFinalizedState(
              lineaRollup,
              l1PublicClient,
              getDefaultLastFinalizedTimestamp(),
            );
          },
        },
      );

      logger.debug(
        `submitForcedTransaction confirmed. txHash=${txHash} status=${receipt.status} blockNumber=${receipt.blockNumber}`,
      );

      expect(receipt.status).toEqual("success");

      const [forcedTxEvent] = await getEvents(l1PublicClient, {
        abi: lineaRollup.abi,
        address: lineaRollup.address,
        eventName: "ForcedTransactionAdded",
        fromBlock: receipt.blockNumber,
        toBlock: receipt.blockNumber,
        strict: true,
      });

      expect(forcedTxEvent).toBeDefined();

      const submittedForcedTransactionNumber = forcedTxEvent.args.forcedTransactionNumber;

      logger.debug(
        `ForcedTransactionAdded — forcedTransactionNumber=${submittedForcedTransactionNumber} from=${forcedTxEvent.args.from}`,
      );

      // Wait for L1 finalization that includes the forced transaction
      logger.debug(
        `Waiting for FinalizedStateUpdated with forcedTransactionNumber >= ${submittedForcedTransactionNumber}`,
      );

      const [finalizedEvent] = await waitForEvents(l1PublicClient, {
        abi: lineaRollup.abi,
        address: lineaRollup.address,
        eventName: "FinalizedStateUpdated",
        fromBlock: receipt.blockNumber,
        toBlock: "latest",
        pollingIntervalMs: 2_000,
        timeoutMs: 200_000,
        strict: true,
        criteria: async (events) =>
          events.filter((e) => e.args.forcedTransactionNumber >= submittedForcedTransactionNumber),
      });

      logger.debug(
        `Finalization includes forced transaction. blockNumber=${finalizedEvent.args.blockNumber} forcedTransactionNumber=${finalizedEvent.args.forcedTransactionNumber}`,
      );

      // Verify L2 receipt is NOT available (BadPrecompile triggers virtual trace generation, tx is not executed on L2)
      logger.debug(`Verifying that the forced transaction receipt is not available on L2 for l2TxHash=${l2TxHash}`);
      await expectReceiptNotFound(l2PublicClient, l2TxHash);
      logger.debug("BadPrecompile forced transaction confirmed: receipt not found on L2 as expected.");
    },
    300_000,
  );

  it.skip(
    "Should reject a forced transaction that exceeds the L2-L1 log limit (TooManyLogs)",
    async () => {
      const [l1Account, l2Deployer, l2ForcedAccount] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const l1PublicClient = context.l1PublicClient();
      const l1WalletClient = context.l1WalletClient({ account: l1Account });
      const l2PublicClient = context.l2PublicClient();
      const l2DeployerWalletClient = context.l2WalletClient({ account: l2Deployer });

      // Deploy MultiMessageSender contract on L2
      const multiMessageSenderDeployFees = await estimateLineaGas(l2PublicClient, {
        account: l2Deployer.address,
        data: encodeDeployData({
          abi: MultiMessageSenderAbi,
          bytecode: MultiMessageSenderAbiBytecode as `0x${string}`,
        }),
        value: 0n,
      });
      const contractAddress = await deployContract(l2DeployerWalletClient, {
        account: l2Deployer,
        chain: l2DeployerWalletClient.chain,
        abi: MultiMessageSenderAbi,
        bytecode: MultiMessageSenderAbiBytecode as `0x${string}`,
        maxFeePerGas: multiMessageSenderDeployFees.maxFeePerGas,
        maxPriorityFeePerGas: multiMessageSenderDeployFees.maxPriorityFeePerGas,
      });

      logger.debug(`MultiMessageSender deployed on L2. address=${contractAddress}`);

      // Read L2MessageService address and minimumFeeInWei
      const l2MessageService = context.l2Contracts.l2MessageService(l2PublicClient);
      const l2MessageServiceAddress = l2MessageService.address;
      const minimumFeeInWei: bigint = await l2MessageService.read.minimumFeeInWei();

      logger.debug(`L2MessageService — address=${l2MessageServiceAddress} minimumFeeInWei=${minimumFeeInWei}`);

      // Encode calldata: send 11 messages to exceed the BLOCK_L2_L1_LOGS limit of 10
      const messageCount = 11n;
      const callData = encodeFunctionData({
        abi: MultiMessageSenderAbi,
        functionName: "sendMultipleMessages",
        args: [l2MessageServiceAddress, l2ForcedAccount.address, minimumFeeInWei, messageCount],
      });

      const totalValue = minimumFeeInWei * messageCount;

      const lineaRollup = context.l1Contracts.lineaRollup(l1PublicClient);
      const gateway = context.l1Contracts.forcedTransactionGateway(l1WalletClient);

      // Resolve finalized state
      let lastFinalizedState = await resolveLastFinalizedState(
        lineaRollup,
        l1PublicClient,
        getDefaultLastFinalizedTimestamp(),
      );

      logger.debug(
        `Resolved finalized state — timestamp=${lastFinalizedState.timestamp} forcedTransactionNumber=${lastFinalizedState.forcedTransactionNumber}`,
      );

      // Build forced transaction calling sendMultipleMessages
      const { forcedTransaction, l2TxHash } = await buildSignedForcedTransaction(context, {
        l2Account: l2ForcedAccount,
        to: contractAddress as Address,
        nonce: 0n,
        value: totalValue,
        data: callData,
        gasLimit: 300_000n,
        maxFeePerGas: 1_000_000_000n,
        maxPriorityFeePerGas: 100_000_000n,
      });

      logger.debug(
        `Built TooManyLogs forced transaction — signer=${l2ForcedAccount.address} to=${contractAddress} value=${totalValue} l2TxHash=${l2TxHash}`,
      );

      const [, , , , feeAmount] = await lineaRollup.read.getRequiredForcedTransactionFields();
      const { maxPriorityFeePerGas, maxFeePerGas } = await l1PublicClient.estimateFeesPerGas();

      // Submit the forced transaction
      const { hash: txHash, receipt } = await sendTransactionWithRetry(
        l1PublicClient,
        (fees) =>
          gateway.write.submitForcedTransaction([forcedTransaction, lastFinalizedState], {
            value: feeAmount,
            maxPriorityFeePerGas,
            maxFeePerGas,
            ...fees,
          }),
        {
          receiptTimeoutMs: 30_000,
          abi: [...gateway.abi, ...lineaRollup.abi],
          retryOnRevert: true,
          beforeRetry: async () => {
            lastFinalizedState = await resolveLastFinalizedState(
              lineaRollup,
              l1PublicClient,
              getDefaultLastFinalizedTimestamp(),
            );
          },
        },
      );

      logger.debug(
        `submitForcedTransaction confirmed. txHash=${txHash} status=${receipt.status} blockNumber=${receipt.blockNumber}`,
      );

      expect(receipt.status).toEqual("success");

      const [forcedTxEvent] = await getEvents(l1PublicClient, {
        abi: lineaRollup.abi,
        address: lineaRollup.address,
        eventName: "ForcedTransactionAdded",
        fromBlock: receipt.blockNumber,
        toBlock: receipt.blockNumber,
        strict: true,
      });

      expect(forcedTxEvent).toBeDefined();

      const submittedForcedTransactionNumber = forcedTxEvent.args.forcedTransactionNumber;

      logger.debug(
        `ForcedTransactionAdded — forcedTransactionNumber=${submittedForcedTransactionNumber} from=${forcedTxEvent.args.from}`,
      );

      // Wait for L1 finalization that includes the forced transaction
      logger.debug(
        `Waiting for FinalizedStateUpdated with forcedTransactionNumber >= ${submittedForcedTransactionNumber}`,
      );

      const [finalizedEvent] = await waitForEvents(l1PublicClient, {
        abi: lineaRollup.abi,
        address: lineaRollup.address,
        eventName: "FinalizedStateUpdated",
        fromBlock: receipt.blockNumber,
        toBlock: "latest",
        pollingIntervalMs: 2_000,
        timeoutMs: 200_000,
        strict: true,
        criteria: async (events) =>
          events.filter((e) => e.args.forcedTransactionNumber >= submittedForcedTransactionNumber),
      });

      logger.debug(
        `Finalization includes forced transaction. blockNumber=${finalizedEvent.args.blockNumber} forcedTransactionNumber=${finalizedEvent.args.forcedTransactionNumber}`,
      );

      // Verify L2 receipt is NOT available (TooManyLogs triggers virtual trace generation, tx is not executed on L2)
      logger.debug(`Verifying that the forced transaction receipt is not available on L2 for l2TxHash=${l2TxHash}`);
      await expectReceiptNotFound(l2PublicClient, l2TxHash);
      logger.debug("TooManyLogs forced transaction confirmed: receipt not found on L2 as expected.");
    },
    300_000,
  );

  it.skip(
    "Should reject a forced transaction from a denylisted sender (FilteredAddressFrom)",
    async () => {
      const [l1Account, l2DeniedSender, l2Recipient] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const l1PublicClient = context.l1PublicClient();
      const l1WalletClient = context.l1WalletClient({ account: l1Account });
      const l1SecurityCouncilClient = context.l1WalletClient({ account: getL1SecurityCouncilAccount() });
      const l2PublicClient = context.l2PublicClient();
      const sequencerClient = context.l2PublicClient({ type: L2RpcEndpoint.Sequencer });

      const lineaRollup = context.l1Contracts.lineaRollup(l1PublicClient);
      const gateway = context.l1Contracts.forcedTransactionGateway(l1WalletClient);

      let lastFinalizedState = await resolveLastFinalizedState(
        lineaRollup,
        l1PublicClient,
        getDefaultLastFinalizedTimestamp(),
      );

      logger.debug(
        `Resolved finalized state — timestamp=${lastFinalizedState.timestamp} forcedTransactionNumber=${lastFinalizedState.forcedTransactionNumber}`,
      );

      const { forcedTransaction, l2TxHash } = await buildSignedForcedTransaction(context, {
        l2Account: l2DeniedSender,
        to: l2Recipient.address,
        nonce: 0n,
        value: etherToWei("0.1"),
        gasLimit: 21_000n,
        maxFeePerGas: 1_000_000_000n,
        maxPriorityFeePerGas: 100_000_000n,
      });

      logger.debug(
        `Built FilteredAddressFrom forced transaction — signer=${l2DeniedSender.address} to=${l2Recipient.address} l2TxHash=${l2TxHash}`,
      );

      const [, , , , feeAmount] = await lineaRollup.read.getRequiredForcedTransactionFields();
      const { maxPriorityFeePerGas, maxFeePerGas } = await l1PublicClient.estimateFeesPerGas();

      await withDenyListAddresses(sequencerClient, [l2DeniedSender.address], async () => {
        logger.debug(`Sender added to deny list. address=${l2DeniedSender.address}`);

        // Submit BEFORE adding to the on-chain AddressFilter — the gateway rejects with
        // AddressIsFiltered() if the sender or recipient is filtered at submission time.
        const { hash: txHash, receipt } = await sendTransactionWithRetry(
          l1PublicClient,
          (fees) =>
            gateway.write.submitForcedTransaction([forcedTransaction, lastFinalizedState], {
              value: feeAmount,
              maxPriorityFeePerGas,
              maxFeePerGas,
              ...fees,
            }),
          {
            receiptTimeoutMs: 30_000,
            abi: [...gateway.abi, ...lineaRollup.abi],
            retryOnRevert: true,
            beforeRetry: async () => {
              lastFinalizedState = await resolveLastFinalizedState(
                lineaRollup,
                l1PublicClient,
                getDefaultLastFinalizedTimestamp(),
              );
            },
          },
        );

        logger.debug(
          `submitForcedTransaction confirmed. txHash=${txHash} status=${receipt.status} blockNumber=${receipt.blockNumber}`,
        );

        expect(receipt.status).toEqual("success");

        const [forcedTxEvent] = await getEvents(l1PublicClient, {
          abi: lineaRollup.abi,
          address: lineaRollup.address,
          eventName: "ForcedTransactionAdded",
          fromBlock: receipt.blockNumber,
          toBlock: receipt.blockNumber,
          strict: true,
        });

        expect(forcedTxEvent).toBeDefined();
        const submittedForcedTransactionNumber = forcedTxEvent.args.forcedTransactionNumber;

        logger.debug(
          `ForcedTransactionAdded — forcedTransactionNumber=${submittedForcedTransactionNumber} from=${forcedTxEvent.args.from}`,
        );

        // Add to on-chain AddressFilter AFTER submission so the coordinator's finalize call
        // passes _validateFilteredAddresses(). Removed in finally so subsequent submissions
        // for the same address (other tests) aren't blocked by gateway pre-filtering.
        await withOnChainFilteredAddresses(
          l1PublicClient,
          l1SecurityCouncilClient,
          lineaRollup.address,
          [l2DeniedSender.address],
          async () => {
            logger.debug(`Sender added to on-chain AddressFilter. address=${l2DeniedSender.address}`);

            logger.debug(
              `Waiting for FinalizedStateUpdated with forcedTransactionNumber >= ${submittedForcedTransactionNumber}`,
            );

            const [finalizedEvent] = await waitForEvents(l1PublicClient, {
              abi: lineaRollup.abi,
              address: lineaRollup.address,
              eventName: "FinalizedStateUpdated",
              fromBlock: receipt.blockNumber,
              toBlock: "latest",
              pollingIntervalMs: 2_000,
              timeoutMs: 200_000,
              strict: true,
              criteria: async (events) =>
                events.filter((e) => e.args.forcedTransactionNumber >= submittedForcedTransactionNumber),
            });

            logger.debug(
              `Finalization includes forced transaction. blockNumber=${finalizedEvent.args.blockNumber} forcedTransactionNumber=${finalizedEvent.args.forcedTransactionNumber}`,
            );

            // Verify L2 receipt is NOT available (denylisted sender triggers virtual trace generation, tx is not executed on L2)
            logger.debug(
              `Verifying that the forced transaction receipt is not available on L2 for l2TxHash=${l2TxHash}`,
            );
            await expectReceiptNotFound(l2PublicClient, l2TxHash);
            logger.debug("FilteredAddressFrom forced transaction confirmed: receipt not found on L2 as expected.");
          },
        );
      });
    },
    300_000,
  );

  it.skip(
    "Should reject a forced transaction to a denylisted recipient (FilteredAddressTo)",
    async () => {
      const [l1Account, l2ForcedSender, l2DeniedRecipient] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const l1PublicClient = context.l1PublicClient();
      const l1WalletClient = context.l1WalletClient({ account: l1Account });
      const l1SecurityCouncilClient = context.l1WalletClient({ account: getL1SecurityCouncilAccount() });
      const l2PublicClient = context.l2PublicClient();
      const sequencerClient = context.l2PublicClient({ type: L2RpcEndpoint.Sequencer });

      const lineaRollup = context.l1Contracts.lineaRollup(l1PublicClient);
      const gateway = context.l1Contracts.forcedTransactionGateway(l1WalletClient);

      let lastFinalizedState = await resolveLastFinalizedState(
        lineaRollup,
        l1PublicClient,
        getDefaultLastFinalizedTimestamp(),
      );

      logger.debug(
        `Resolved finalized state — timestamp=${lastFinalizedState.timestamp} forcedTransactionNumber=${lastFinalizedState.forcedTransactionNumber}`,
      );

      const { forcedTransaction, l2TxHash } = await buildSignedForcedTransaction(context, {
        l2Account: l2ForcedSender,
        to: l2DeniedRecipient.address,
        nonce: 0n,
        value: etherToWei("0.1"),
        gasLimit: 21_000n,
        maxFeePerGas: 1_000_000_000n,
        maxPriorityFeePerGas: 100_000_000n,
      });

      logger.debug(
        `Built FilteredAddressTo forced transaction — signer=${l2ForcedSender.address} to=${l2DeniedRecipient.address} l2TxHash=${l2TxHash}`,
      );

      const [, , , , feeAmount] = await lineaRollup.read.getRequiredForcedTransactionFields();
      const { maxPriorityFeePerGas, maxFeePerGas } = await l1PublicClient.estimateFeesPerGas();

      await withDenyListAddresses(sequencerClient, [l2DeniedRecipient.address], async () => {
        logger.debug(`Recipient added to deny list. address=${l2DeniedRecipient.address}`);

        // Submit BEFORE adding to the on-chain AddressFilter — the gateway rejects with
        // AddressIsFiltered() if the sender or recipient is filtered at submission time.
        const { hash: txHash, receipt } = await sendTransactionWithRetry(
          l1PublicClient,
          (fees) =>
            gateway.write.submitForcedTransaction([forcedTransaction, lastFinalizedState], {
              value: feeAmount,
              maxPriorityFeePerGas,
              maxFeePerGas,
              ...fees,
            }),
          {
            receiptTimeoutMs: 30_000,
            abi: [...gateway.abi, ...lineaRollup.abi],
            retryOnRevert: true,
            beforeRetry: async () => {
              lastFinalizedState = await resolveLastFinalizedState(
                lineaRollup,
                l1PublicClient,
                getDefaultLastFinalizedTimestamp(),
              );
            },
          },
        );

        logger.debug(
          `submitForcedTransaction confirmed. txHash=${txHash} status=${receipt.status} blockNumber=${receipt.blockNumber}`,
        );

        expect(receipt.status).toEqual("success");

        const [forcedTxEvent] = await getEvents(l1PublicClient, {
          abi: lineaRollup.abi,
          address: lineaRollup.address,
          eventName: "ForcedTransactionAdded",
          fromBlock: receipt.blockNumber,
          toBlock: receipt.blockNumber,
          strict: true,
        });

        expect(forcedTxEvent).toBeDefined();
        const submittedForcedTransactionNumber = forcedTxEvent.args.forcedTransactionNumber;

        logger.debug(
          `ForcedTransactionAdded — forcedTransactionNumber=${submittedForcedTransactionNumber} from=${forcedTxEvent.args.from}`,
        );

        // Add to on-chain AddressFilter AFTER submission so the coordinator's finalize call
        // passes _validateFilteredAddresses(). Removed in finally so subsequent submissions
        // for the same address (other tests) aren't blocked by gateway pre-filtering.
        await withOnChainFilteredAddresses(
          l1PublicClient,
          l1SecurityCouncilClient,
          lineaRollup.address,
          [l2DeniedRecipient.address],
          async () => {
            logger.debug(`Recipient added to on-chain AddressFilter. address=${l2DeniedRecipient.address}`);

            logger.debug(
              `Waiting for FinalizedStateUpdated with forcedTransactionNumber >= ${submittedForcedTransactionNumber}`,
            );

            const [finalizedEvent] = await waitForEvents(l1PublicClient, {
              abi: lineaRollup.abi,
              address: lineaRollup.address,
              eventName: "FinalizedStateUpdated",
              fromBlock: receipt.blockNumber,
              toBlock: "latest",
              pollingIntervalMs: 2_000,
              timeoutMs: 200_000,
              strict: true,
              criteria: async (events) =>
                events.filter((e) => e.args.forcedTransactionNumber >= submittedForcedTransactionNumber),
            });

            logger.debug(
              `Finalization includes forced transaction. blockNumber=${finalizedEvent.args.blockNumber} forcedTransactionNumber=${finalizedEvent.args.forcedTransactionNumber}`,
            );

            // Verify L2 receipt is NOT available (denylisted recipient triggers virtual trace generation, tx is not executed on L2)
            logger.debug(
              `Verifying that the forced transaction receipt is not available on L2 for l2TxHash=${l2TxHash}`,
            );
            await expectReceiptNotFound(l2PublicClient, l2TxHash);
            logger.debug("FilteredAddressTo forced transaction confirmed: receipt not found on L2 as expected.");
          },
        );
      });
    },
    300_000,
  );
});
