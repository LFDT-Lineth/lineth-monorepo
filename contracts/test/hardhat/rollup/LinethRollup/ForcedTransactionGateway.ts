import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { type Address, getAddress } from "viem";

import {
  getAccountsFixture,
  deployForcedTransactionGatewayFixture,
  setNextExpectedL2BlockNumberForForcedTx,
  getForcedTransactionRollingHash,
  setForcedTransactionFee,
  decodeForcedTransactionAdded,
} from "./../helpers/index.js";
import {
  DEFAULT_ADMIN_ROLE,
  FORCED_TRANSACTION_FEE_SETTER_ROLE,
  PRECOMPILES_ADDRESSES,
} from "../../../../common/constants/index.js";
import l2SendMessageTransaction from "../../_testData/eip1559RlpEncoderTransactions/l2SendMessage.json" with { type: "json" };
import transactionWithCalldata from "../../_testData/eip1559RlpEncoderTransactions/withCalldata.json" with { type: "json" };
import transactionWithCalldataAndAccessList from "../../_testData/eip1559RlpEncoderTransactions/withCalldataAndAccessList.json" with { type: "json" };
import transactionWithLargeCalldata from "../../_testData/eip1559RlpEncoderTransactions/withLargeCalldata.json" with { type: "json" };
import transactionWithoutCalldata from "../../_testData/eip1559RlpEncoderTransactions/withoutCalldata.json" with { type: "json" };
import {
  ADDRESS_ZERO,
  DEFAULT_LAST_FINALIZED_TIMESTAMP,
  FORCED_TRANSACTION_SENDER_ROLE,
  HASH_ZERO,
  L2_BLOCK_DURATION_SECONDS,
  LINEA_MAINNET_CHAIN_ID,
  MAX_GAS_LIMIT,
  MAX_INPUT_LENGTH_LIMIT,
  THREE_DAYS_IN_SECONDS,
  DEFAULT_FUTURE_NEXT_NETWORK_TIMESTAMP,
  FORCED_TRANSACTION_FEE,
  BLOCK_NUMBER_DEADLINE_BUFFER,
} from "../../common/constants/index.js";
import { deployFromFactory } from "../../common/deployment.js";
import {
  type TestAccount,
  address,
  buildAccessErrorMessage,
  buildEip1559Transaction,
  calculateLastFinalizedState,
  generateRandomBytes,
  hex,
  expectRevertWithCustomError,
  expectRevertWithReason,
  expectEvent,
} from "../../common/helpers/index.js";

import type { LastFinalizedState } from "../../common/types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("Lineth Rollup contract: Forced Transactions", () => {
  let linethRollup: ContractReturnType<"TestLinethRollup">;
  let addressFilter: ContractReturnType<"AddressFilter">;
  let forcedTransactionGateway: ContractReturnType<"ForcedTransactionGateway">;
  let mimcLibrary: ContractReturnType<"Mimc">;
  let mimcLibraryAddress: Address;

  let securityCouncil: TestAccount;
  let nonAuthorizedAccount: TestAccount;
  let defaultFinalizedState: LastFinalizedState = {
    messageNumber: 0n,
    messageRollingHash: HASH_ZERO,
    forcedTransactionNumber: 0n,
    forcedTransactionRollingHash: HASH_ZERO,
    timestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
  };

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ nonAuthorizedAccount, securityCouncil } = await networkHelpers.loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({
      linethRollup,
      forcedTransactionGateway,
      addressFilter,
      mimc: mimcLibrary,
    } = await networkHelpers.loadFixture(deployForcedTransactionGatewayFixture));
    mimcLibraryAddress = mimcLibrary.address;

    await linethRollup.write.grantRole([FORCED_TRANSACTION_SENDER_ROLE, forcedTransactionGateway.address], {
      account: securityCouncil.account,
    });

    await linethRollup.write.grantRole([FORCED_TRANSACTION_SENDER_ROLE, securityCouncil.address], {
      account: securityCouncil.account,
    });

    defaultFinalizedState = {
      messageNumber: 0n,
      messageRollingHash: HASH_ZERO,
      forcedTransactionNumber: 0n,
      forcedTransactionRollingHash: HASH_ZERO,
      timestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
    };
  });

  describe("Contract Construction", () => {
    // Configuration object for constructor arguments
    interface ConstructorConfig {
      linethRollupAddr: Address;
      chainId: bigint;
      blockBuffer: bigint;
      maxGasLimit: bigint;
      maxInputLengthLimit: bigint;
      securityCouncilAddr: Address;
      addressFilterAddr: Address;
      l2BlockDurationSeconds: bigint;
      blockNumberDeadlineBuffer: bigint;
    }

    // Convert config object to constructor args tuple
    const toArgs = (
      config: ConstructorConfig,
    ): [Address, bigint, bigint, bigint, bigint, Address, Address, bigint, bigint] => [
      config.linethRollupAddr,
      config.chainId,
      config.blockBuffer,
      config.maxGasLimit,
      config.maxInputLengthLimit,
      config.securityCouncilAddr,
      config.addressFilterAddr,
      config.l2BlockDurationSeconds,
      config.blockNumberDeadlineBuffer,
    ];

    // Parameterized constructor validation tests - each case only specifies the override
    const constructorValidationCases: Array<{
      description: string;
      override: Partial<ConstructorConfig>;
      expectedError: "ZeroAddressNotAllowed" | "ZeroValueNotAllowed";
    }> = [
      {
        description: "Lineth rollup is set as address(0)",
        override: { linethRollupAddr: ADDRESS_ZERO },
        expectedError: "ZeroAddressNotAllowed",
      },
      {
        description: "chainId is set to zero",
        override: { chainId: 0n },
        expectedError: "ZeroValueNotAllowed",
      },
      {
        description: "block buffer is set to zero",
        override: { blockBuffer: 0n },
        expectedError: "ZeroValueNotAllowed",
      },
      {
        description: "max gas limit is set to zero",
        override: { maxGasLimit: 0n },
        expectedError: "ZeroValueNotAllowed",
      },
      {
        description: "max input limit is set to zero",
        override: { maxInputLengthLimit: 0n },
        expectedError: "ZeroValueNotAllowed",
      },
      {
        description: "default admin is the zero address",
        override: { securityCouncilAddr: ADDRESS_ZERO },
        expectedError: "ZeroAddressNotAllowed",
      },
      {
        description: "address filter address is zero address",
        override: { addressFilterAddr: ADDRESS_ZERO },
        expectedError: "ZeroAddressNotAllowed",
      },
      {
        description: "l2 block time is set to zero",
        override: { l2BlockDurationSeconds: 0n },
        expectedError: "ZeroValueNotAllowed",
      },
      {
        description: "block number deadline buffer is set to zero",
        override: { blockNumberDeadlineBuffer: 0n },
        expectedError: "ZeroValueNotAllowed",
      },
    ];

    constructorValidationCases.forEach(({ description, override, expectedError }) => {
      it(`Should fail if the ${description}`, async () => {
        const defaultConfig: ConstructorConfig = {
          linethRollupAddr: linethRollup.address,
          chainId: LINEA_MAINNET_CHAIN_ID,
          blockBuffer: BigInt(THREE_DAYS_IN_SECONDS),
          maxGasLimit: BigInt(MAX_GAS_LIMIT),
          maxInputLengthLimit: MAX_INPUT_LENGTH_LIMIT,
          securityCouncilAddr: securityCouncil.address,
          addressFilterAddr: addressFilter.address,
          l2BlockDurationSeconds: L2_BLOCK_DURATION_SECONDS,
          blockNumberDeadlineBuffer: BLOCK_NUMBER_DEADLINE_BUFFER,
        };

        const args = toArgs({ ...defaultConfig, ...override });

        await expectRevertWithCustomError(
          forcedTransactionGateway,
          deployFromFactory("ForcedTransactionGateway", args, { libraries: { Mimc: mimcLibraryAddress } }),
          expectedError,
        );
      });
    });

    it("Should fail if the l2 block time is set to zero", async () => {
      await expectRevertWithCustomError(
        forcedTransactionGateway,
        deployFromFactory(
          "ForcedTransactionGateway",
          [
            linethRollup.address,
            LINEA_MAINNET_CHAIN_ID,
            BigInt(THREE_DAYS_IN_SECONDS),
            BigInt(MAX_GAS_LIMIT),
            MAX_INPUT_LENGTH_LIMIT,
            securityCouncil.address,
            addressFilter.address,
            0n,
            BLOCK_NUMBER_DEADLINE_BUFFER,
          ],
          { libraries: { Mimc: mimcLibraryAddress } },
        ),
        "ZeroValueNotAllowed",
      );
    });

    it("Should fail if the block number deadline buffer is set to zero", async () => {
      await expectRevertWithCustomError(
        forcedTransactionGateway,
        deployFromFactory(
          "ForcedTransactionGateway",
          [
            linethRollup.address,
            LINEA_MAINNET_CHAIN_ID,
            BigInt(THREE_DAYS_IN_SECONDS),
            BigInt(MAX_GAS_LIMIT),
            MAX_INPUT_LENGTH_LIMIT,
            securityCouncil.address,
            addressFilter.address,
            L2_BLOCK_DURATION_SECONDS,
            0n,
          ],
          { libraries: { Mimc: mimcLibraryAddress } },
        ),
        "ZeroValueNotAllowed",
      );
    });
  });

  describe("Toggling the address filter feature", () => {
    it("Should fail to toggle if unauthorized", async () => {
      await expectRevertWithReason(
        forcedTransactionGateway.write.toggleUseAddressFilter([false], { account: nonAuthorizedAccount.account }),
        buildAccessErrorMessage(nonAuthorizedAccount, DEFAULT_ADMIN_ROLE),
      );
    });

    it("Should fail to toggle if the status is the same", async () => {
      const asyncCall = forcedTransactionGateway.write.toggleUseAddressFilter([true], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(forcedTransactionGateway, asyncCall, "AddressFilterAlreadySet", [true]);
    });

    it("Should toggle if the status is the different", async () => {
      let useAddressFilter = await forcedTransactionGateway.read.useAddressFilter();
      expect(useAddressFilter).to.be.true;

      await forcedTransactionGateway.write.toggleUseAddressFilter([false], { account: securityCouncil.account });

      useAddressFilter = await forcedTransactionGateway.read.useAddressFilter();
      expect(useAddressFilter).to.be.false;
    });

    it("Should emit AddressFilterSet when changed", async () => {
      let asyncCall = forcedTransactionGateway.write.toggleUseAddressFilter([false], {
        account: securityCouncil.account,
      });
      await expectEvent(forcedTransactionGateway, asyncCall, "AddressFilterSet", [false]);

      asyncCall = forcedTransactionGateway.write.toggleUseAddressFilter([true], { account: securityCouncil.account });
      await expectEvent(forcedTransactionGateway, asyncCall, "AddressFilterSet", [true]);

      const useAddressFilter = await forcedTransactionGateway.read.useAddressFilter();
      expect(useAddressFilter).to.be.true;
    });
  });

  describe("Setting the forced transaction fee", () => {
    it("Should fail to set if unauthorized", async () => {
      await expectRevertWithReason(
        setForcedTransactionFee(linethRollup, FORCED_TRANSACTION_FEE, nonAuthorizedAccount),
        buildAccessErrorMessage(nonAuthorizedAccount, FORCED_TRANSACTION_FEE_SETTER_ROLE),
      );
    });

    it("Should set the forced transaction fee", async () => {
      let forcedTransactionFee = await linethRollup.read.forcedTransactionFeeInWei();
      expect(forcedTransactionFee).to.be.equal(0n);
      await linethRollup.write.setForcedTransactionFee([FORCED_TRANSACTION_FEE], { account: securityCouncil.account });
      forcedTransactionFee = await linethRollup.read.forcedTransactionFeeInWei();
      expect(forcedTransactionFee).to.be.equal(FORCED_TRANSACTION_FEE);
    });

    it("Should fail to set the forced transaction fee is zero", async () => {
      await expectRevertWithCustomError(
        linethRollup,
        setForcedTransactionFee(linethRollup, 0n, securityCouncil),
        "ZeroValueNotAllowed",
      );
    });

    it("Should set the forced transaction fee and emit ForcedTransactionFeeSet event", async () => {
      // Todo - see why wrapped call fails
      const asyncCall = linethRollup.write.setForcedTransactionFee([FORCED_TRANSACTION_FEE], {
        account: securityCouncil.account,
      });
      await expectEvent(linethRollup, asyncCall, "ForcedTransactionFeeSet", [FORCED_TRANSACTION_FEE]);
    });
  });

  describe("Adding forced transactions", () => {
    it("Should fail if the forced transaction fee is zero", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);

      await setForcedTransactionFee(linethRollup, FORCED_TRANSACTION_FEE, securityCouncil);

      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.write.submitForcedTransaction([forcedTransaction, defaultFinalizedState]),
        "ForcedTransactionFeeNotMet",
        [FORCED_TRANSACTION_FEE, 0n],
      );
    });

    it("Should fail if the forced transaction fee is less than the required fee", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);

      await setForcedTransactionFee(linethRollup, FORCED_TRANSACTION_FEE, securityCouncil);

      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.write.submitForcedTransaction([forcedTransaction, defaultFinalizedState], {
          value: FORCED_TRANSACTION_FEE - 1n,
        }),
        "ForcedTransactionFeeNotMet",
        [FORCED_TRANSACTION_FEE, FORCED_TRANSACTION_FEE - 1n],
      );
    });

    it("Should fail if the forced transaction fee is more than the required fee", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);

      await setForcedTransactionFee(linethRollup, FORCED_TRANSACTION_FEE, securityCouncil);

      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.write.submitForcedTransaction([forcedTransaction, defaultFinalizedState], {
          value: FORCED_TRANSACTION_FEE + 1n,
        }),
        "ForcedTransactionFeeNotMet",
        [FORCED_TRANSACTION_FEE, FORCED_TRANSACTION_FEE + 1n],
      );
    });

    it("Should fail if the gas limit is too high", async () => {
      const sendCall = forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(transactionWithLargeCalldata.result),
        defaultFinalizedState,
      ]);
      await expectRevertWithCustomError(forcedTransactionGateway, sendCall, "MaxGasLimitExceeded");
    });

    it("Should fail if the gas limit is too low", async () => {
      const forcedTransaction = buildEip1559Transaction(transactionWithLargeCalldata.result);
      forcedTransaction.gasLimit = 20999n;

      const sendCall = forcedTransactionGateway.write.submitForcedTransaction([
        forcedTransaction,
        defaultFinalizedState,
      ]);
      await expectRevertWithCustomError(forcedTransactionGateway, sendCall, "GasLimitTooLow");
    });

    it("Should fail if the calldata input is too long", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      forcedTransaction.input = generateRandomBytes(Number(MAX_INPUT_LENGTH_LIMIT) + 1);
      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.write.submitForcedTransaction([forcedTransaction, defaultFinalizedState]),
        "CalldataInputLengthLimitExceeded",
      );
    });

    it("Should fail if the maxPriorityFeePerGas is zero", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      forcedTransaction.maxPriorityFeePerGas = 0n;
      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.write.submitForcedTransaction([forcedTransaction, defaultFinalizedState]),
        "GasFeeParametersContainZero",
        [forcedTransaction.maxFeePerGas, 0n],
      );
    });

    it("Should fail if the maxFeePerGas is zero", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      forcedTransaction.maxFeePerGas = 0n;
      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.write.submitForcedTransaction([forcedTransaction, defaultFinalizedState]),
        "GasFeeParametersContainZero",
        [0n, forcedTransaction.maxPriorityFeePerGas],
      );
    });

    it("Should fail if maxPriorityFeePerGas > maxFeePerGas", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      forcedTransaction.maxPriorityFeePerGas = 2n;
      forcedTransaction.maxFeePerGas = 1n;
      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.write.submitForcedTransaction([forcedTransaction, defaultFinalizedState]),
        "MaxPriorityFeePerGasHigherThanMaxFee",
        [forcedTransaction.maxFeePerGas, forcedTransaction.maxPriorityFeePerGas],
      );
    });

    it("Should fail if YParity > 1", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      forcedTransaction.yParity = 2;
      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.write.submitForcedTransaction([forcedTransaction, defaultFinalizedState]),
        "YParityGreaterThanOne",
        [BigInt(forcedTransaction.yParity)],
      );
    });

    it("Should fail if the To address is a precompile address", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      for (let i = 0; i < PRECOMPILES_ADDRESSES.length; i++) {
        forcedTransaction.to = getAddress(PRECOMPILES_ADDRESSES[i]);
        await expectRevertWithCustomError(
          forcedTransactionGateway,
          forcedTransactionGateway.write.submitForcedTransaction([forcedTransaction, defaultFinalizedState]),
          "AddressIsFiltered",
        );
      }
    });

    it("Should fail if the last finalized state hash does not match", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);

      const defaultFinalizedStateHash = calculateLastFinalizedState(
        defaultFinalizedState.messageNumber,
        defaultFinalizedState.messageRollingHash,
        defaultFinalizedState.forcedTransactionNumber,
        defaultFinalizedState.forcedTransactionRollingHash,
        defaultFinalizedState.timestamp,
      );

      const corruptedFinalizedStateStruct = {
        ...defaultFinalizedState,
        forcedTransactionNumber: 1n,
      };

      const corruptedFinalizationStateHash = calculateLastFinalizedState(
        corruptedFinalizedStateStruct.messageNumber,
        corruptedFinalizedStateStruct.messageRollingHash,
        corruptedFinalizedStateStruct.forcedTransactionNumber,
        corruptedFinalizedStateStruct.forcedTransactionRollingHash,
        corruptedFinalizedStateStruct.timestamp,
      );

      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.write.submitForcedTransaction([forcedTransaction, corruptedFinalizedStateStruct]),
        "FinalizationStateIncorrect",
        [defaultFinalizedStateHash, corruptedFinalizationStateHash],
      );
    });

    it("Should fail LinethRollup.storeForcedTransaction if not FORCED_TRANSACTION_SENDER_ROLE", async () => {
      await expectRevertWithReason(
        linethRollup.write.storeForcedTransaction(
          [generateRandomBytes(32), securityCouncil.address, 121n, generateRandomBytes(32)],
          { account: nonAuthorizedAccount.account },
        ),
        buildAccessErrorMessage(nonAuthorizedAccount, FORCED_TRANSACTION_SENDER_ROLE),
      );
    });

    it("Should fail if the rlp encoded signed transaction is zero length", async () => {
      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.write.storeForcedTransaction(
          [generateRandomBytes(32), securityCouncil.address, 121n, generateRandomBytes(0)],
          { account: securityCouncil.account },
        ),
        "ZeroLengthNotAllowed",
      );
    });

    it("Should fail if the block number deadline is zero", async () => {
      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.write.storeForcedTransaction(
          [generateRandomBytes(32), securityCouncil.address, 0n, generateRandomBytes(32)],
          { account: securityCouncil.account },
        ),
        "ZeroValueNotAllowed",
      );
    });

    it("Should fail if the from address is zero address", async () => {
      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.write.storeForcedTransaction(
          [generateRandomBytes(32), ADDRESS_ZERO, 121n, generateRandomBytes(32)],
          { account: securityCouncil.account },
        ),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should fail if the forced transaction rolling hash is zero hash", async () => {
      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.write.storeForcedTransaction([HASH_ZERO, securityCouncil.address, 121n, generateRandomBytes(32)], {
          account: securityCouncil.account,
        }),
        "ZeroHashNotAllowed",
      );
    });

    it("Should set lastSubmissionBlock after a forced transaction submission", async () => {
      const { viem } = await network.getOrCreate();
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);

      const tx = await forcedTransactionGateway.write.submitForcedTransaction([
        forcedTransaction,
        defaultFinalizedState,
      ]);

      const publicClient = await viem.getPublicClient();
      const receipt = await publicClient.waitForTransactionReceipt({ hash: tx });
      expect(await forcedTransactionGateway.read.lastSubmissionBlock()).to.equal(receipt.blockNumber);
    });

    it("Should prevent two forced transactions in the same L1 block", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);

      const { viem, networkHelpers } = await network.getOrCreate();
      const publicClient = await viem.getPublicClient();
      const testClient = await viem.getTestClient();

      // Disable automine to batch transactions in the same block
      await testClient.setAutomine(false);

      try {
        // Queue both forced transactions in the mempool. An explicit gas limit skips the node's gas estimation, which
        // runs against the pending block and would reject the second transaction before it reaches the mempool.
        const tx1 = await forcedTransactionGateway.write.submitForcedTransaction(
          [forcedTransaction, defaultFinalizedState],
          {
            gas: BigInt(MAX_GAS_LIMIT),
          },
        );
        const tx2 = await forcedTransactionGateway.write.submitForcedTransaction(
          [forcedTransaction, defaultFinalizedState],
          {
            gas: BigInt(MAX_GAS_LIMIT),
          },
        );

        // Mine a single block containing both queued transactions
        await networkHelpers.mine();

        // First transaction should succeed
        const receipt1 = await publicClient.getTransactionReceipt({ hash: tx1 });
        expect(receipt1).to.not.be.null;
        expect(receipt1.status).to.equal("success");

        // Second transaction should have reverted during block execution
        const receipt2 = await publicClient.getTransactionReceipt({ hash: tx2 });
        expect(receipt2).to.not.be.null;
        expect(receipt2.status).to.equal("reverted");

        // Only one forced transaction should be stored
        expect(await linethRollup.read.nextForcedTransactionNumber()).to.equal(2n);
      } finally {
        await testClient.setAutomine(true);
      }
    });

    it("Should fail if the second transaction is expected on the same block", async () => {
      const tx = await forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      ]);

      const events = await decodeForcedTransactionAdded(tx, linethRollup);
      const event = events[0];

      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.write.storeForcedTransaction(
          [
            event.args.forcedTransactionRollingHash,
            event.args.from,
            event.args.blockNumberDeadline,
            event.args.rlpEncodedSignedTransaction,
          ],
          { account: securityCouncil.account },
        ),
        "ForcedTransactionExistsForBlockOrIsTooLow",
        [event.args.blockNumberDeadline],
      );
    });

    it("Should fail if the second transaction has a lower block number", async () => {
      const blockNumberDeadline = await setNextExpectedL2BlockNumberForForcedTx(
        linethRollup,
        DEFAULT_FUTURE_NEXT_NETWORK_TIMESTAMP,
        defaultFinalizedState.timestamp + 1n,
      );

      await forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      ]);

      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.write.storeForcedTransaction(
          [generateRandomBytes(32), securityCouncil.address, blockNumberDeadline, generateRandomBytes(32)],
          { account: securityCouncil.account },
        ),
        "ForcedTransactionExistsForBlockOrIsTooLow",
        [blockNumberDeadline],
      );
    });

    it("Should fail if the signer address is zero", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      // Force the signer address to be zero when performing the ecrecover calculation.
      forcedTransaction.r = 0n;
      forcedTransaction.s = 0n;
      forcedTransaction.yParity = 0;
      await expectRevertWithReason(
        forcedTransactionGateway.write.submitForcedTransaction([forcedTransaction, defaultFinalizedState]),
        "ECDSA: invalid signature",
      );
    });

    it("Should submit the forced transaction with no calldata", async () => {
      await forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(transactionWithoutCalldata.result),
        defaultFinalizedState,
      ]);
    });

    it("Should submit the forced transaction with calldata and access list", async () => {
      await forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(transactionWithCalldataAndAccessList.result),
        defaultFinalizedState,
      ]);
    });

    it("Should submit the forced transaction with calldata", async () => {
      await forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(transactionWithCalldata.result),
        defaultFinalizedState,
      ]);
    });

    it("Should submit the forced L2 SendMessage transaction with calldata", async () => {
      await forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      ]);
    });

    it("Should emit the ForcedTransactionAdded event on adding a transaction", async () => {
      // use a way future dated timestamp and mimc the calculation for the block number
      const blockNumberDeadline = await setNextExpectedL2BlockNumberForForcedTx(
        linethRollup,
        1954213624n,
        defaultFinalizedState.timestamp,
      );

      const expectedForcedTransactionNumber = 1n;

      const expectedMimcHashWithPreviousZeroValueRollingHash = await getForcedTransactionRollingHash(
        mimcLibrary,
        linethRollup,
        buildEip1559Transaction(l2SendMessageTransaction.result),
        blockNumberDeadline,
        l2SendMessageTransaction?.result?.from,
        BigInt(l2SendMessageTransaction.result.chainId),
      );

      await expectEvent(
        linethRollup,
        forcedTransactionGateway.write.submitForcedTransaction([
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        ]),
        "ForcedTransactionAdded",
        [
          expectedForcedTransactionNumber,
          getAddress(l2SendMessageTransaction.result.from),
          blockNumberDeadline,
          expectedMimcHashWithPreviousZeroValueRollingHash,
          hex(l2SendMessageTransaction.rlpEncodedSigned),
        ],
      );
    });

    // Test cases for different block times with even and odd elapsed times
    // Uses a large base elapsed time to ensure network timestamp is always in the future
    const baseElapsedTime = DEFAULT_FUTURE_NEXT_NETWORK_TIMESTAMP - DEFAULT_LAST_FINALIZED_TIMESTAMP;
    const blockTimeTestCases = [
      {
        description: "even elapsed time with 2s block time",
        l2BlockTimeSeconds: 2n,
        // Ensure elapsed time is even by subtracting 1 if base is odd
        networkTimestamp: DEFAULT_FUTURE_NEXT_NETWORK_TIMESTAMP - (baseElapsedTime % 2n),
      },
      {
        description: "odd elapsed time with 2s block time (truncated)",
        l2BlockTimeSeconds: 2n,
        // Ensure elapsed time is odd by adding 1 if base is even
        networkTimestamp: DEFAULT_FUTURE_NEXT_NETWORK_TIMESTAMP - (baseElapsedTime % 2n) + 1n,
      },
    ];

    blockTimeTestCases.forEach(({ description, l2BlockTimeSeconds, networkTimestamp }) => {
      it(`Should calculate correct blockNumberDeadline with ${description}`, async () => {
        // Deploy a new ForcedTransactionGateway with custom block time
        const forcedTransactionGatewayWithCustomBlockTime = await deployFromFactory(
          "ForcedTransactionGateway",
          [
            linethRollup.address,
            LINEA_MAINNET_CHAIN_ID,
            BigInt(THREE_DAYS_IN_SECONDS),
            BigInt(MAX_GAS_LIMIT),
            MAX_INPUT_LENGTH_LIMIT,
            securityCouncil.address,
            addressFilter.address,
            l2BlockTimeSeconds,
            BLOCK_NUMBER_DEADLINE_BUFFER,
          ],
          { libraries: { Mimc: mimcLibraryAddress } },
        );

        await linethRollup.write.grantRole(
          [FORCED_TRANSACTION_SENDER_ROLE, forcedTransactionGatewayWithCustomBlockTime.address],
          { account: securityCouncil.account },
        );

        // Calculate expected block deadline with custom block time
        const blockNumberDeadline = await setNextExpectedL2BlockNumberForForcedTx(
          linethRollup,
          networkTimestamp,
          defaultFinalizedState.timestamp,
          l2BlockTimeSeconds,
        );

        const expectedForcedTransactionNumber = 1n;

        const expectedMimcHashWithPreviousZeroValueRollingHash = await getForcedTransactionRollingHash(
          mimcLibrary,
          linethRollup,
          buildEip1559Transaction(l2SendMessageTransaction.result),
          blockNumberDeadline,
          l2SendMessageTransaction?.result?.from,
          BigInt(l2SendMessageTransaction.result.chainId),
        );

        await expectEvent(
          linethRollup,
          forcedTransactionGatewayWithCustomBlockTime.write.submitForcedTransaction([
            buildEip1559Transaction(l2SendMessageTransaction.result),
            defaultFinalizedState,
          ]),
          "ForcedTransactionAdded",
          [
            expectedForcedTransactionNumber,
            getAddress(l2SendMessageTransaction.result.from),
            blockNumberDeadline,
            expectedMimcHashWithPreviousZeroValueRollingHash,
            hex(l2SendMessageTransaction.rlpEncodedSigned),
          ],
        );
      });
    });

    describe("blockNumberDeadline adjustment when computed deadline does not exceed previous", () => {
      let customGateway: ContractReturnType<"ForcedTransactionGateway">;
      const l2BlockDurationSeconds = 12n;
      // Timestamp chosen so (T - T0) mod 12 = 5, giving room for multiple consecutive
      // seconds to produce the same integer division result:
      // 120_000_005 / 12 = 10_000_000, 120_000_006 / 12 = 10_000_000, 120_000_007 / 12 = 10_000_000
      let firstTimestamp: bigint;

      beforeEach(async () => {
        customGateway = await deployFromFactory(
          "ForcedTransactionGateway",
          [
            linethRollup.address,
            LINEA_MAINNET_CHAIN_ID,
            BigInt(THREE_DAYS_IN_SECONDS),
            BigInt(MAX_GAS_LIMIT),
            MAX_INPUT_LENGTH_LIMIT,
            securityCouncil.address,
            addressFilter.address,
            l2BlockDurationSeconds,
            BLOCK_NUMBER_DEADLINE_BUFFER,
          ],
          { libraries: { Mimc: mimcLibraryAddress } },
        );

        await linethRollup.write.grantRole([FORCED_TRANSACTION_SENDER_ROLE, customGateway.address], {
          account: securityCouncil.account,
        });

        firstTimestamp = DEFAULT_LAST_FINALIZED_TIMESTAMP + 120_000_005n;
      });

      it("Should adjust when computed deadline equals previous forced transaction deadline", async () => {
        const { networkHelpers } = await network.getOrCreate();
        // Submit first forced transaction
        await networkHelpers.time.setNextBlockTimestamp(firstTimestamp);
        const tx1 = await customGateway.write.submitForcedTransaction([
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        ]);
        const events1 = await decodeForcedTransactionAdded(tx1, linethRollup);
        const firstDeadline = events1[0].args.blockNumberDeadline;

        // Second tx computed deadline equals firstDeadline (same integer division result)
        // so the gateway adjusts it to firstDeadline + BLOCK_NUMBER_DEADLINE_BUFFER
        await networkHelpers.time.setNextBlockTimestamp(firstTimestamp + 1n);
        const tx2 = await customGateway.write.submitForcedTransaction([
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        ]);
        const events2 = await decodeForcedTransactionAdded(tx2, linethRollup);
        const secondDeadline = events2[0].args.blockNumberDeadline;

        expect(secondDeadline).to.equal(firstDeadline + BLOCK_NUMBER_DEADLINE_BUFFER);
      });

      it("Should adjust when computed deadline is less than previous forced transaction deadline", async () => {
        const { networkHelpers } = await network.getOrCreate();
        // Submit first forced transaction
        await networkHelpers.time.setNextBlockTimestamp(firstTimestamp);
        const tx1 = await customGateway.write.submitForcedTransaction([
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        ]);
        const events1 = await decodeForcedTransactionAdded(tx1, linethRollup);
        const firstDeadline = events1[0].args.blockNumberDeadline;

        // Second tx: computed deadline == firstDeadline → adjusted to firstDeadline + BUFFER
        await networkHelpers.time.setNextBlockTimestamp(firstTimestamp + 1n);
        const tx2 = await customGateway.write.submitForcedTransaction([
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        ]);
        const events2 = await decodeForcedTransactionAdded(tx2, linethRollup);
        const secondDeadline = events2[0].args.blockNumberDeadline;

        // Verify the second deadline was adjusted above the first
        expect(secondDeadline).to.equal(firstDeadline + BLOCK_NUMBER_DEADLINE_BUFFER);

        // Third tx: computed deadline still equals firstDeadline (same integer division)
        // which is strictly less than secondDeadline (firstDeadline + BUFFER)
        // so the gateway adjusts it to secondDeadline + BLOCK_NUMBER_DEADLINE_BUFFER
        await networkHelpers.time.setNextBlockTimestamp(firstTimestamp + 2n);
        const tx3 = await customGateway.write.submitForcedTransaction([
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        ]);
        const events3 = await decodeForcedTransactionAdded(tx3, linethRollup);
        const thirdDeadline = events3[0].args.blockNumberDeadline;

        expect(thirdDeadline).to.equal(secondDeadline + BLOCK_NUMBER_DEADLINE_BUFFER);
      });
    });

    it("Should change rolling hash with different expected block number", async () => {
      // use a way future dated timestamp and mimic the calculation for the block number
      const blockNumberDeadline = await setNextExpectedL2BlockNumberForForcedTx(
        linethRollup,
        1854213624n,
        defaultFinalizedState.timestamp,
      );

      const expectedForcedTransactionNumber = 1n;

      const expectedMimcHashWithPreviousZeroValueRollingHash = await getForcedTransactionRollingHash(
        mimcLibrary,
        linethRollup,
        buildEip1559Transaction(l2SendMessageTransaction.result),
        blockNumberDeadline,
        l2SendMessageTransaction?.result?.from,
        BigInt(l2SendMessageTransaction.result.chainId),
      );

      await expectEvent(
        linethRollup,
        forcedTransactionGateway.write.submitForcedTransaction([
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        ]),
        "ForcedTransactionAdded",
        [
          expectedForcedTransactionNumber,
          getAddress(l2SendMessageTransaction.result.from),
          blockNumberDeadline,
          expectedMimcHashWithPreviousZeroValueRollingHash,
          hex(l2SendMessageTransaction.rlpEncodedSigned),
        ],
      );
    });

    it("Updates the next message number on the Lineth Rollup", async () => {
      expect(await linethRollup.read.nextForcedTransactionNumber()).equal(1n);

      await forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      ]);

      expect(await linethRollup.read.nextForcedTransactionNumber()).equal(2n);
    });

    it("Updates the forcedTransactionL2BlockNumbers on the Lineth Rollup", async () => {
      expect(await linethRollup.read.forcedTransactionL2BlockNumbers([1n])).equal(0n);

      await forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      ]);

      expect(await linethRollup.read.forcedTransactionL2BlockNumbers([1n])).greaterThan(0n);
    });

    it("Fails to add the transaction with `to` on the address filter list.", async () => {
      expect(await linethRollup.read.forcedTransactionL2BlockNumbers([1n])).equal(0n);

      await addressFilter.write.setFilteredStatus([[address(l2SendMessageTransaction.result.to)], true], {
        account: securityCouncil.account,
      });

      const asyncCall = forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      ]);

      await expectRevertWithCustomError(forcedTransactionGateway, asyncCall, "AddressIsFiltered");
    });

    it("Fails to add the transaction with `from` on the address filter list.", async () => {
      expect(await linethRollup.read.forcedTransactionL2BlockNumbers([1n])).equal(0n);

      await addressFilter.write.setFilteredStatus([[address(l2SendMessageTransaction.result.from)], true], {
        account: securityCouncil.account,
      });

      const asyncCall = forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      ]);

      await expectRevertWithCustomError(forcedTransactionGateway, asyncCall, "AddressIsFiltered");
    });

    it("Adds the transaction with `to` on the address filter list, but the feature is disabled.", async () => {
      expect(await linethRollup.read.forcedTransactionL2BlockNumbers([1n])).equal(0n);

      await addressFilter.write.setFilteredStatus([[address(l2SendMessageTransaction.result.to)], true], {
        account: securityCouncil.account,
      });

      await forcedTransactionGateway.write.toggleUseAddressFilter([false], { account: securityCouncil.account });

      await forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      ]);

      expect(await linethRollup.read.forcedTransactionL2BlockNumbers([1n])).greaterThan(0n);
    });

    it("Adds the transaction with `from` on the address filter list, but the feature is disabled.", async () => {
      expect(await linethRollup.read.forcedTransactionL2BlockNumbers([1n])).equal(0n);

      await addressFilter.write.setFilteredStatus([[address(l2SendMessageTransaction.result.from)], true], {
        account: securityCouncil.account,
      });

      await forcedTransactionGateway.write.toggleUseAddressFilter([false], { account: securityCouncil.account });

      await forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      ]);

      expect(await linethRollup.read.forcedTransactionL2BlockNumbers([1n])).greaterThan(0n);
    });

    it("Updates the forcedTransactionRollingHashes on the Lineth Rollup", async () => {
      expect(await linethRollup.read.forcedTransactionRollingHashes([1n])).equal(HASH_ZERO);
      const blockNumberDeadline = await setNextExpectedL2BlockNumberForForcedTx(
        linethRollup,
        DEFAULT_FUTURE_NEXT_NETWORK_TIMESTAMP,
        defaultFinalizedState.timestamp,
      );
      const expectedForcedTxRollingHash = await getForcedTransactionRollingHash(
        mimcLibrary,
        linethRollup,
        buildEip1559Transaction(l2SendMessageTransaction.result),
        blockNumberDeadline,
        l2SendMessageTransaction?.result?.from,
        BigInt(l2SendMessageTransaction.result.chainId),
      );

      await forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      ]);

      expect(await linethRollup.read.forcedTransactionRollingHashes([1n])).equal(expectedForcedTxRollingHash);
    });
  });
});
