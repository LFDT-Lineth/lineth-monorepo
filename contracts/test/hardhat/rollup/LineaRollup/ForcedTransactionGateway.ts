import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture, time as networkTime } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import {
  DEFAULT_ADMIN_ROLE,
  FORCED_TRANSACTION_FEE_SETTER_ROLE,
  PRECOMPILES_ADDRESSES,
} from "contracts/common/constants";
import { ForcedTransactionGateway, AddressFilter, Mimc, TestLineaRollup } from "contracts/typechain-types";
import { ethers } from "hardhat";

import {
  getAccountsFixture,
  deployForcedTransactionGatewayFixture,
  setNextExpectedL2BlockNumberForForcedTx,
  getForcedTransactionRollingHash,
  setForcedTransactionFee,
  decodeForcedTransactionAdded,
} from "./../helpers";
import l2SendMessageTransaction from "../../_testData/eip1559RlpEncoderTransactions/l2SendMessage.json";
import transactionWithCalldata from "../../_testData/eip1559RlpEncoderTransactions/withCalldata.json";
import transactionWithCalldataAndAccessList from "../../_testData/eip1559RlpEncoderTransactions/withCalldataAndAccessList.json";
import transactionWithLargeCalldata from "../../_testData/eip1559RlpEncoderTransactions/withLargeCalldata.json";
import transactionWithoutCalldata from "../../_testData/eip1559RlpEncoderTransactions/withoutCalldata.json";
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
} from "../../common/constants";
import {
  buildAccessErrorMessage,
  buildEip1559Transaction,
  calculateLastFinalizedState,
  expectEvent,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateRandomBytes,
} from "../../common/helpers";
import { LastFinalizedState } from "../../common/types";

describe("Linea Rollup contract: Forced Transactions", () => {
  let lineaRollup: TestLineaRollup;
  let addressFilter: AddressFilter;
  let forcedTransactionGateway: ForcedTransactionGateway;
  let mimcLibrary: Mimc;
  let mimcLibraryAddress: string;

  let securityCouncil: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let defaultFinalizedState: LastFinalizedState = {
    messageNumber: 0n,
    messageRollingHash: HASH_ZERO,
    forcedTransactionNumber: 0n,
    forcedTransactionRollingHash: HASH_ZERO,
    timestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
  };

  before(async () => {
    ({ nonAuthorizedAccount, securityCouncil } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({
      lineaRollup,
      forcedTransactionGateway,
      addressFilter,
      mimc: mimcLibrary,
    } = await loadFixture(deployForcedTransactionGatewayFixture));
    mimcLibraryAddress = await mimcLibrary.getAddress();

    await lineaRollup
      .connect(securityCouncil)
      .grantRole(FORCED_TRANSACTION_SENDER_ROLE, await forcedTransactionGateway.getAddress());

    await lineaRollup
      .connect(securityCouncil)
      .grantRole(FORCED_TRANSACTION_SENDER_ROLE, await securityCouncil.getAddress());

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
      lineaRollupAddr: string;
      chainId: bigint | number;
      blockBuffer: bigint | number;
      maxGasLimit: bigint | number;
      maxInputLengthLimit: bigint | number;
      securityCouncilAddr: string;
      addressFilterAddr: string;
      l2BlockDurationSeconds: bigint | number;
      blockNumberDeadlineBuffer: bigint | number;
    }

    // Convert config object to constructor args tuple
    const toArgs = (
      config: ConstructorConfig,
    ): [
      string,
      bigint | number,
      bigint | number,
      bigint | number,
      bigint | number,
      string,
      string,
      bigint | number,
      bigint | number,
    ] => [
      config.lineaRollupAddr,
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
      expectedError: string;
    }> = [
      {
        description: "Linea rollup is set as address(0)",
        override: { lineaRollupAddr: ADDRESS_ZERO },
        expectedError: "ZeroAddressNotAllowed",
      },
      {
        description: "chainId is set to zero",
        override: { chainId: 0 },
        expectedError: "ZeroValueNotAllowed",
      },
      {
        description: "block buffer is set to zero",
        override: { blockBuffer: 0 },
        expectedError: "ZeroValueNotAllowed",
      },
      {
        description: "max gas limit is set to zero",
        override: { maxGasLimit: 0 },
        expectedError: "ZeroValueNotAllowed",
      },
      {
        description: "max input limit is set to zero",
        override: { maxInputLengthLimit: 0 },
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
        override: { l2BlockDurationSeconds: 0 },
        expectedError: "ZeroValueNotAllowed",
      },
      {
        description: "block number deadline buffer is set to zero",
        override: { blockNumberDeadlineBuffer: 0 },
        expectedError: "ZeroValueNotAllowed",
      },
    ];

    constructorValidationCases.forEach(({ description, override, expectedError }) => {
      it(`Should fail if the ${description}`, async () => {
        const forcedTransactionGatewayFactory = await ethers.getContractFactory("ForcedTransactionGateway", {
          libraries: { Mimc: mimcLibraryAddress },
        });

        const defaultConfig: ConstructorConfig = {
          lineaRollupAddr: await lineaRollup.getAddress(),
          chainId: LINEA_MAINNET_CHAIN_ID,
          blockBuffer: THREE_DAYS_IN_SECONDS,
          maxGasLimit: MAX_GAS_LIMIT,
          maxInputLengthLimit: MAX_INPUT_LENGTH_LIMIT,
          securityCouncilAddr: securityCouncil.address,
          addressFilterAddr: await addressFilter.getAddress(),
          l2BlockDurationSeconds: L2_BLOCK_DURATION_SECONDS,
          blockNumberDeadlineBuffer: BLOCK_NUMBER_DEADLINE_BUFFER,
        };

        const args = toArgs({ ...defaultConfig, ...override });

        await expectRevertWithCustomError(
          forcedTransactionGateway,
          forcedTransactionGatewayFactory.deploy(...args),
          expectedError,
        );
      });
    });

    it("Should fail if the l2 block time is set to zero", async () => {
      const forcedTransactionGatewayFactory = await ethers.getContractFactory("ForcedTransactionGateway", {
        libraries: { Mimc: mimcLibraryAddress },
      });

      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGatewayFactory.deploy(
          await lineaRollup.getAddress(),
          LINEA_MAINNET_CHAIN_ID,
          THREE_DAYS_IN_SECONDS,
          MAX_GAS_LIMIT,
          MAX_INPUT_LENGTH_LIMIT,
          securityCouncil.address,
          await addressFilter.getAddress(),
          0n,
          BLOCK_NUMBER_DEADLINE_BUFFER,
        ),
        "ZeroValueNotAllowed",
      );
    });

    it("Should fail if the block number deadline buffer is set to zero", async () => {
      const forcedTransactionGatewayFactory = await ethers.getContractFactory("ForcedTransactionGateway", {
        libraries: { Mimc: mimcLibraryAddress },
      });

      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGatewayFactory.deploy(
          await lineaRollup.getAddress(),
          LINEA_MAINNET_CHAIN_ID,
          THREE_DAYS_IN_SECONDS,
          MAX_GAS_LIMIT,
          MAX_INPUT_LENGTH_LIMIT,
          securityCouncil.address,
          await addressFilter.getAddress(),
          L2_BLOCK_DURATION_SECONDS,
          0n,
        ),
        "ZeroValueNotAllowed",
      );
    });
  });

  describe("Toggling the address filter feature", () => {
    it("Should fail to toggle if unauthorized", async () => {
      await expectRevertWithReason(
        forcedTransactionGateway.connect(nonAuthorizedAccount).toggleUseAddressFilter(false),
        buildAccessErrorMessage(nonAuthorizedAccount, DEFAULT_ADMIN_ROLE),
      );
    });

    it("Should fail to toggle if the status is the same", async () => {
      const asyncCall = forcedTransactionGateway.connect(securityCouncil).toggleUseAddressFilter(true);
      await expectRevertWithCustomError(forcedTransactionGateway, asyncCall, "AddressFilterAlreadySet", [true]);
    });

    it("Should toggle if the status is the different", async () => {
      let useAddressFilter = await forcedTransactionGateway.useAddressFilter();
      expect(useAddressFilter).to.be.true;

      await forcedTransactionGateway.connect(securityCouncil).toggleUseAddressFilter(false);

      useAddressFilter = await forcedTransactionGateway.useAddressFilter();
      expect(useAddressFilter).to.be.false;
    });

    it("Should emit AddressFilterSet when changed", async () => {
      let asyncCall = forcedTransactionGateway.connect(securityCouncil).toggleUseAddressFilter(false);
      await expectEvent(forcedTransactionGateway, asyncCall, "AddressFilterSet", [false]);

      asyncCall = forcedTransactionGateway.connect(securityCouncil).toggleUseAddressFilter(true);
      await expectEvent(forcedTransactionGateway, asyncCall, "AddressFilterSet", [true]);

      const useAddressFilter = await forcedTransactionGateway.useAddressFilter();
      expect(useAddressFilter).to.be.true;
    });
  });

  describe("Setting the forced transaction fee", () => {
    it("Should fail to set if unauthorized", async () => {
      await expectRevertWithReason(
        setForcedTransactionFee(lineaRollup, FORCED_TRANSACTION_FEE, nonAuthorizedAccount),
        buildAccessErrorMessage(nonAuthorizedAccount, FORCED_TRANSACTION_FEE_SETTER_ROLE),
      );
    });

    it("Should set the forced transaction fee", async () => {
      let forcedTransactionFee = await lineaRollup.forcedTransactionFeeInWei();
      expect(forcedTransactionFee).to.be.equal(0n);
      await lineaRollup.connect(securityCouncil).setForcedTransactionFee(FORCED_TRANSACTION_FEE);
      forcedTransactionFee = await lineaRollup.forcedTransactionFeeInWei();
      expect(forcedTransactionFee).to.be.equal(FORCED_TRANSACTION_FEE);
    });

    it("Should fail to set the forced transaction fee is zero", async () => {
      await expectRevertWithCustomError(
        lineaRollup,
        setForcedTransactionFee(lineaRollup, 0n, securityCouncil),
        "ZeroValueNotAllowed",
      );
    });

    it("Should set the forced transaction fee and emit ForcedTransactionFeeSet event", async () => {
      // Todo - see why wrapped call fails
      const asyncCall = lineaRollup.connect(securityCouncil).setForcedTransactionFee(FORCED_TRANSACTION_FEE);
      await expectEvent(lineaRollup, asyncCall, "ForcedTransactionFeeSet", [FORCED_TRANSACTION_FEE]);
    });
  });

  describe("Adding forced transactions", () => {
    it("Should fail if the forced transaction fee is zero", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);

      await setForcedTransactionFee(lineaRollup, FORCED_TRANSACTION_FEE, securityCouncil);

      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState),
        "ForcedTransactionFeeNotMet",
        [FORCED_TRANSACTION_FEE, 0],
      );
    });

    it("Should fail if the forced transaction fee is less than the required fee", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);

      await setForcedTransactionFee(lineaRollup, FORCED_TRANSACTION_FEE, securityCouncil);

      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState, {
          value: FORCED_TRANSACTION_FEE - 1n,
        }),
        "ForcedTransactionFeeNotMet",
        [FORCED_TRANSACTION_FEE, FORCED_TRANSACTION_FEE - 1n],
      );
    });

    it("Should fail if the forced transaction fee is more than the required fee", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);

      await setForcedTransactionFee(lineaRollup, FORCED_TRANSACTION_FEE, securityCouncil);

      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState, {
          value: FORCED_TRANSACTION_FEE + 1n,
        }),
        "ForcedTransactionFeeNotMet",
        [FORCED_TRANSACTION_FEE, FORCED_TRANSACTION_FEE + 1n],
      );
    });

    it("Should fail if the gas limit is too high", async () => {
      const sendCall = forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(transactionWithLargeCalldata.result),
        defaultFinalizedState,
      );
      await expectRevertWithCustomError(forcedTransactionGateway, sendCall, "MaxGasLimitExceeded");
    });

    it("Should fail if the gas limit is too low", async () => {
      const forcedTransaction = buildEip1559Transaction(transactionWithLargeCalldata.result);
      forcedTransaction.gasLimit = 20999n;

      const sendCall = forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState);
      await expectRevertWithCustomError(forcedTransactionGateway, sendCall, "GasLimitTooLow");
    });

    it("Should fail if the calldata input is too long", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      forcedTransaction.input = generateRandomBytes(Number(MAX_INPUT_LENGTH_LIMIT) + 1);
      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState),
        "CalldataInputLengthLimitExceeded",
      );
    });

    it("Should fail if the maxPriorityFeePerGas is zero", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      forcedTransaction.maxPriorityFeePerGas = 0n;
      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState),
        "GasFeeParametersContainZero",
        [forcedTransaction.maxFeePerGas, 0],
      );
    });

    it("Should fail if the maxFeePerGas is zero", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      forcedTransaction.maxFeePerGas = 0n;
      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState),
        "GasFeeParametersContainZero",
        [0, forcedTransaction.maxPriorityFeePerGas],
      );
    });

    it("Should fail if maxPriorityFeePerGas > maxFeePerGas", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      forcedTransaction.maxPriorityFeePerGas = 2n;
      forcedTransaction.maxFeePerGas = 1n;
      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState),
        "MaxPriorityFeePerGasHigherThanMaxFee",
        [forcedTransaction.maxFeePerGas, forcedTransaction.maxPriorityFeePerGas],
      );
    });

    it("Should fail if YParity > 1", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      forcedTransaction.yParity = 2n;
      await expectRevertWithCustomError(
        forcedTransactionGateway,
        forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState),
        "YParityGreaterThanOne",
        [forcedTransaction.yParity],
      );
    });

    it("Should fail if the To address is a precompile address", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);
      for (let i = 0; i < PRECOMPILES_ADDRESSES.length; i++) {
        forcedTransaction.to = ethers.getAddress(PRECOMPILES_ADDRESSES[i]);
        await expectRevertWithCustomError(
          forcedTransactionGateway,
          forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState),
          "AddressIsFiltered",
          [],
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
        forcedTransactionGateway.submitForcedTransaction(forcedTransaction, corruptedFinalizedStateStruct),
        "FinalizationStateIncorrect",
        [defaultFinalizedStateHash, corruptedFinalizationStateHash],
      );
    });

    it("Should fail LineaRollup.storeForcedTransaction if not FORCED_TRANSACTION_SENDER_ROLE", async () => {
      await expectRevertWithReason(
        lineaRollup
          .connect(nonAuthorizedAccount)
          .storeForcedTransaction(generateRandomBytes(32), securityCouncil.address, 121n, generateRandomBytes(32)),
        buildAccessErrorMessage(nonAuthorizedAccount, FORCED_TRANSACTION_SENDER_ROLE),
      );
    });

    it("Should fail if the rlp encoded signed transaction is zero length", async () => {
      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup
          .connect(securityCouncil)
          .storeForcedTransaction(generateRandomBytes(32), securityCouncil.address, 121n, generateRandomBytes(0)),
        "ZeroLengthNotAllowed",
      );
    });

    it("Should fail if the block number deadline is zero", async () => {
      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup
          .connect(securityCouncil)
          .storeForcedTransaction(generateRandomBytes(32), securityCouncil.address, 0n, generateRandomBytes(32)),
        "ZeroValueNotAllowed",
      );
    });

    it("Should fail if the from address is zero address", async () => {
      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup
          .connect(securityCouncil)
          .storeForcedTransaction(generateRandomBytes(32), ADDRESS_ZERO, 121n, generateRandomBytes(32)),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should fail if the forced transaction rolling hash is zero hash", async () => {
      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup
          .connect(securityCouncil)
          .storeForcedTransaction(HASH_ZERO, securityCouncil.address, 121n, generateRandomBytes(32)),
        "ZeroHashNotAllowed",
      );
    });

    it("Should set lastSubmissionBlock after a forced transaction submission", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);

      const tx = await forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState);

      const receipt = await tx.wait();
      expect(await forcedTransactionGateway.lastSubmissionBlock()).to.equal(receipt!.blockNumber);
    });

    it("Should prevent two forced transactions in the same L1 block", async () => {
      const forcedTransaction = buildEip1559Transaction(l2SendMessageTransaction.result);

      // Disable automine to batch transactions in the same block
      await ethers.provider.send("evm_setAutomine", [false]);

      try {
        // Queue both forced transactions in the mempool
        const tx1 = await forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState);
        const tx2 = await forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState);

        // Mine a single block containing both queued transactions
        await ethers.provider.send("evm_mine");

        // First transaction should succeed
        const receipt1 = await ethers.provider.getTransactionReceipt(tx1.hash);
        expect(receipt1).to.not.be.null;
        expect(receipt1!.status).to.equal(1);

        // Second transaction should have reverted during block execution
        const receipt2 = await ethers.provider.getTransactionReceipt(tx2.hash);
        expect(receipt2).to.not.be.null;
        expect(receipt2!.status).to.equal(0);

        // Only one forced transaction should be stored
        expect(await lineaRollup.nextForcedTransactionNumber()).to.equal(2n);
      } finally {
        await ethers.provider.send("evm_setAutomine", [true]);
      }
    });

    it("Should fail if the second transaction is expected on the same block", async () => {
      const tx = await forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      );

      const events = await decodeForcedTransactionAdded(tx, lineaRollup);
      const event = events[0];

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup
          .connect(securityCouncil)
          .storeForcedTransaction(
            event.args.forcedTransactionRollingHash,
            event.args.from,
            event.args.blockNumberDeadline,
            event.args.rlpEncodedSignedTransaction,
          ),
        "ForcedTransactionExistsForBlockOrIsTooLow",
        [event.args.blockNumberDeadline],
      );
    });

    it("Should fail if the second transaction has a lower block number", async () => {
      const blockNumberDeadline = await setNextExpectedL2BlockNumberForForcedTx(
        lineaRollup,
        DEFAULT_FUTURE_NEXT_NETWORK_TIMESTAMP,
        defaultFinalizedState.timestamp + 1n,
      );

      await forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      );

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup
          .connect(securityCouncil)
          .storeForcedTransaction(
            generateRandomBytes(32),
            securityCouncil.address,
            blockNumberDeadline,
            generateRandomBytes(32),
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
      forcedTransaction.yParity = 0n;
      await expectRevertWithReason(
        forcedTransactionGateway.submitForcedTransaction(forcedTransaction, defaultFinalizedState),
        "ECDSA: invalid signature",
      );
    });

    it("Should submit the forced transaction with no calldata", async () => {
      await forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(transactionWithoutCalldata.result),
        defaultFinalizedState,
      );
    });

    it("Should submit the forced transaction with calldata and access list", async () => {
      await forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(transactionWithCalldataAndAccessList.result),
        defaultFinalizedState,
      );
    });

    it("Should submit the forced transaction with calldata", async () => {
      await forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(transactionWithCalldata.result),
        defaultFinalizedState,
      );
    });

    it("Should submit the forced L2 SendMessage transaction with calldata", async () => {
      await forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      );
    });

    it("Should emit the ForcedTransactionAdded event on adding a transaction", async () => {
      // use a way future dated timestamp and mimc the calculation for the block number
      const blockNumberDeadline = await setNextExpectedL2BlockNumberForForcedTx(
        lineaRollup,
        1954213624n,
        defaultFinalizedState.timestamp,
      );

      const expectedForcedTransactionNumber = 1n;

      const expectedMimcHashWithPreviousZeroValueRollingHash = await getForcedTransactionRollingHash(
        mimcLibrary,
        lineaRollup,
        buildEip1559Transaction(l2SendMessageTransaction.result),
        blockNumberDeadline,
        l2SendMessageTransaction?.result?.from,
        BigInt(l2SendMessageTransaction.result.chainId),
      );

      const expectedEventArgs = [
        expectedForcedTransactionNumber,
        ethers.getAddress(l2SendMessageTransaction.result.from),
        blockNumberDeadline,
        expectedMimcHashWithPreviousZeroValueRollingHash,
        l2SendMessageTransaction.rlpEncodedSigned,
      ];

      await expectEvent(
        lineaRollup,
        forcedTransactionGateway.submitForcedTransaction(
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        ),
        "ForcedTransactionAdded",
        expectedEventArgs,
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
        const forcedTransactionGatewayFactory = await ethers.getContractFactory("ForcedTransactionGateway", {
          libraries: { Mimc: mimcLibraryAddress },
        });
        const forcedTransactionGatewayWithCustomBlockTime = await forcedTransactionGatewayFactory.deploy(
          await lineaRollup.getAddress(),
          LINEA_MAINNET_CHAIN_ID,
          THREE_DAYS_IN_SECONDS,
          MAX_GAS_LIMIT,
          MAX_INPUT_LENGTH_LIMIT,
          securityCouncil.address,
          await addressFilter.getAddress(),
          l2BlockTimeSeconds,
          BLOCK_NUMBER_DEADLINE_BUFFER,
        );

        await lineaRollup
          .connect(securityCouncil)
          .grantRole(FORCED_TRANSACTION_SENDER_ROLE, await forcedTransactionGatewayWithCustomBlockTime.getAddress());

        // Calculate expected block deadline with custom block time
        const blockNumberDeadline = await setNextExpectedL2BlockNumberForForcedTx(
          lineaRollup,
          networkTimestamp,
          defaultFinalizedState.timestamp,
          l2BlockTimeSeconds,
        );

        const expectedForcedTransactionNumber = 1n;

        const expectedMimcHashWithPreviousZeroValueRollingHash = await getForcedTransactionRollingHash(
          mimcLibrary,
          lineaRollup,
          buildEip1559Transaction(l2SendMessageTransaction.result),
          blockNumberDeadline,
          l2SendMessageTransaction?.result?.from,
          BigInt(l2SendMessageTransaction.result.chainId),
        );

        const expectedEventArgs = [
          expectedForcedTransactionNumber,
          ethers.getAddress(l2SendMessageTransaction.result.from),
          blockNumberDeadline,
          expectedMimcHashWithPreviousZeroValueRollingHash,
          l2SendMessageTransaction.rlpEncodedSigned,
        ];

        await expectEvent(
          lineaRollup,
          forcedTransactionGatewayWithCustomBlockTime.submitForcedTransaction(
            buildEip1559Transaction(l2SendMessageTransaction.result),
            defaultFinalizedState,
          ),
          "ForcedTransactionAdded",
          expectedEventArgs,
        );
      });
    });

    describe("blockNumberDeadline adjustment when computed deadline does not exceed previous", () => {
      let customGateway: ForcedTransactionGateway;
      const l2BlockDurationSeconds = 12n;
      // Timestamp chosen so (T - T0) mod 12 = 5, giving room for multiple consecutive
      // seconds to produce the same integer division result:
      // 120_000_005 / 12 = 10_000_000, 120_000_006 / 12 = 10_000_000, 120_000_007 / 12 = 10_000_000
      let firstTimestamp: bigint;

      beforeEach(async () => {
        const forcedTransactionGatewayFactory = await ethers.getContractFactory("ForcedTransactionGateway", {
          libraries: { Mimc: mimcLibraryAddress },
        });
        customGateway = (await forcedTransactionGatewayFactory.deploy(
          await lineaRollup.getAddress(),
          LINEA_MAINNET_CHAIN_ID,
          THREE_DAYS_IN_SECONDS,
          MAX_GAS_LIMIT,
          MAX_INPUT_LENGTH_LIMIT,
          securityCouncil.address,
          await addressFilter.getAddress(),
          l2BlockDurationSeconds,
          BLOCK_NUMBER_DEADLINE_BUFFER,
        )) as unknown as ForcedTransactionGateway;

        await lineaRollup
          .connect(securityCouncil)
          .grantRole(FORCED_TRANSACTION_SENDER_ROLE, await customGateway.getAddress());

        firstTimestamp = DEFAULT_LAST_FINALIZED_TIMESTAMP + 120_000_005n;
      });

      it("Should adjust when computed deadline equals previous forced transaction deadline", async () => {
        // Submit first forced transaction
        await networkTime.setNextBlockTimestamp(firstTimestamp);
        const tx1 = await customGateway.submitForcedTransaction(
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        );
        const events1 = await decodeForcedTransactionAdded(tx1, lineaRollup);
        const firstDeadline = events1[0].args.blockNumberDeadline;

        // Second tx computed deadline equals firstDeadline (same integer division result)
        // so the gateway adjusts it to firstDeadline + BLOCK_NUMBER_DEADLINE_BUFFER
        await networkTime.setNextBlockTimestamp(firstTimestamp + 1n);
        const tx2 = await customGateway.submitForcedTransaction(
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        );
        const events2 = await decodeForcedTransactionAdded(tx2, lineaRollup);
        const secondDeadline = events2[0].args.blockNumberDeadline;

        expect(secondDeadline).to.equal(firstDeadline + BLOCK_NUMBER_DEADLINE_BUFFER);
      });

      it("Should adjust when computed deadline is less than previous forced transaction deadline", async () => {
        // Submit first forced transaction
        await networkTime.setNextBlockTimestamp(firstTimestamp);
        const tx1 = await customGateway.submitForcedTransaction(
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        );
        const events1 = await decodeForcedTransactionAdded(tx1, lineaRollup);
        const firstDeadline = events1[0].args.blockNumberDeadline;

        // Second tx: computed deadline == firstDeadline → adjusted to firstDeadline + BUFFER
        await networkTime.setNextBlockTimestamp(firstTimestamp + 1n);
        const tx2 = await customGateway.submitForcedTransaction(
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        );
        const events2 = await decodeForcedTransactionAdded(tx2, lineaRollup);
        const secondDeadline = events2[0].args.blockNumberDeadline;

        // Verify the second deadline was adjusted above the first
        expect(secondDeadline).to.equal(firstDeadline + BLOCK_NUMBER_DEADLINE_BUFFER);

        // Third tx: computed deadline still equals firstDeadline (same integer division)
        // which is strictly less than secondDeadline (firstDeadline + BUFFER)
        // so the gateway adjusts it to secondDeadline + BLOCK_NUMBER_DEADLINE_BUFFER
        await networkTime.setNextBlockTimestamp(firstTimestamp + 2n);
        const tx3 = await customGateway.submitForcedTransaction(
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        );
        const events3 = await decodeForcedTransactionAdded(tx3, lineaRollup);
        const thirdDeadline = events3[0].args.blockNumberDeadline;

        expect(thirdDeadline).to.equal(secondDeadline + BLOCK_NUMBER_DEADLINE_BUFFER);
      });
    });

    it("Should change rolling hash with different expected block number", async () => {
      // use a way future dated timestamp and mimic the calculation for the block number
      const blockNumberDeadline = await setNextExpectedL2BlockNumberForForcedTx(
        lineaRollup,
        1854213624n,
        defaultFinalizedState.timestamp,
      );

      const expectedForcedTransactionNumber = 1n;

      const expectedMimcHashWithPreviousZeroValueRollingHash = await getForcedTransactionRollingHash(
        mimcLibrary,
        lineaRollup,
        buildEip1559Transaction(l2SendMessageTransaction.result),
        blockNumberDeadline,
        l2SendMessageTransaction?.result?.from,
        BigInt(l2SendMessageTransaction.result.chainId),
      );

      const expectedEventArgs = [
        expectedForcedTransactionNumber,
        ethers.getAddress(l2SendMessageTransaction.result.from),
        blockNumberDeadline,
        expectedMimcHashWithPreviousZeroValueRollingHash,
        l2SendMessageTransaction.rlpEncodedSigned,
      ];

      await expectEvent(
        lineaRollup,
        forcedTransactionGateway.submitForcedTransaction(
          buildEip1559Transaction(l2SendMessageTransaction.result),
          defaultFinalizedState,
        ),
        "ForcedTransactionAdded",
        expectedEventArgs,
      );
    });

    it("Updates the next message number on the Linea Rollup", async () => {
      expect(await lineaRollup.nextForcedTransactionNumber()).equal(1);

      await forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      );

      expect(await lineaRollup.nextForcedTransactionNumber()).equal(2);
    });

    it("Updates the forcedTransactionL2BlockNumbers on the Linea Rollup", async () => {
      expect(await lineaRollup.forcedTransactionL2BlockNumbers(1)).equal(0);

      await forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      );

      expect(await lineaRollup.forcedTransactionL2BlockNumbers(1)).greaterThan(0);
    });

    it("Fails to add the transaction with `to` on the address filter list.", async () => {
      expect(await lineaRollup.forcedTransactionL2BlockNumbers(1)).equal(0);

      await addressFilter.connect(securityCouncil).setFilteredStatus([l2SendMessageTransaction.result.to], true);

      const asyncCall = forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      );

      await expectRevertWithCustomError(forcedTransactionGateway, asyncCall, "AddressIsFiltered");
    });

    it("Fails to add the transaction with `from` on the address filter list.", async () => {
      expect(await lineaRollup.forcedTransactionL2BlockNumbers(1)).equal(0);

      await addressFilter.connect(securityCouncil).setFilteredStatus([l2SendMessageTransaction.result.from], true);

      const asyncCall = forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      );

      await expectRevertWithCustomError(forcedTransactionGateway, asyncCall, "AddressIsFiltered");
    });

    it("Adds the transaction with `to` on the address filter list, but the feature is disabled.", async () => {
      expect(await lineaRollup.forcedTransactionL2BlockNumbers(1)).equal(0);

      await addressFilter.connect(securityCouncil).setFilteredStatus([l2SendMessageTransaction.result.to], true);

      await forcedTransactionGateway.connect(securityCouncil).toggleUseAddressFilter(false);

      await forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      );

      expect(await lineaRollup.forcedTransactionL2BlockNumbers(1)).greaterThan(0);
    });

    it("Adds the transaction with `from` on the address filter list, but the feature is disabled.", async () => {
      expect(await lineaRollup.forcedTransactionL2BlockNumbers(1)).equal(0);

      await addressFilter.connect(securityCouncil).setFilteredStatus([l2SendMessageTransaction.result.from], true);

      await forcedTransactionGateway.connect(securityCouncil).toggleUseAddressFilter(false);

      await forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      );

      expect(await lineaRollup.forcedTransactionL2BlockNumbers(1)).greaterThan(0);
    });

    it("Updates the forcedTransactionRollingHashes on the Linea Rollup", async () => {
      expect(await lineaRollup.forcedTransactionRollingHashes(1)).equal(HASH_ZERO);
      const blockNumberDeadline = await setNextExpectedL2BlockNumberForForcedTx(
        lineaRollup,
        DEFAULT_FUTURE_NEXT_NETWORK_TIMESTAMP,
        defaultFinalizedState.timestamp,
      );
      const expectedForcedTxRollingHash = await getForcedTransactionRollingHash(
        mimcLibrary,
        lineaRollup,
        buildEip1559Transaction(l2SendMessageTransaction.result),
        blockNumberDeadline,
        l2SendMessageTransaction?.result?.from,
        BigInt(l2SendMessageTransaction.result.chainId),
      );

      await forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(l2SendMessageTransaction.result),
        defaultFinalizedState,
      );

      expect(await lineaRollup.forcedTransactionRollingHashes(1)).equal(expectedForcedTxRollingHash);
    });
  });
});
