import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";

import forcedTx0 from "../../_testData/eip1559RlpEncoderTransactions/forced-transaction-0.json" with { type: "json" };
import {
  BLOCK_NUMBER_DEADLINE_BUFFER,
  DEFAULT_LAST_FINALIZED_TIMESTAMP,
  FORCED_TRANSACTION_SENDER_ROLE,
  HASH_ZERO,
  L2_BLOCK_DURATION_SECONDS,
  MAX_INPUT_LENGTH_LIMIT,
} from "../../common/constants/index.js";
import { deployFromFactory } from "../../common/deployment.js";
import { type TestAccount, buildEip1559Transaction } from "../../common/helpers/index.js";
import {
  getAccountsFixture,
  deployLinethRollupFixture,
  deployMimcFixture,
  deployAddressFilter,
} from "../helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe.skip("Lineth Rollup contract: Forced Transactions", () => {
  const MAX_GAS_LIMIT = 10_000_000n;
  const CHAIN_ID = 789979n;

  let linethRollup: ContractReturnType<"TestLinethRollup">;
  let forcedTransactionGateway: ContractReturnType<"ForcedTransactionGateway">;

  let securityCouncil: TestAccount;
  let defaultFinalizedState = {
    messageNumber: 0n,
    messageRollingHash: HASH_ZERO,
    forcedTransactionNumber: 0n,
    forcedTransactionRollingHash: HASH_ZERO,
    timestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
  };

  before(async () => {
    // No `hardhat_reset`: Hardhat 3's simulated network does not support it; loadFixture snapshots isolate the state.
    const { networkHelpers } = await network.getOrCreate();
    ({ securityCouncil } = await networkHelpers.loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ linethRollup, forcedTransactionGateway } = await networkHelpers.loadFixture(
      deployForcedTransactionGatewayFixtureLocally,
    ));

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

  describe("Adding forced transactions", () => {
    it("Should submit the forced transaction with calldata", async () => {
      await forcedTransactionGateway.write.submitForcedTransaction([
        buildEip1559Transaction(forcedTx0.Transaction),
        defaultFinalizedState,
      ]);
    });
  });

  async function deployForcedTransactionGatewayFixtureLocally() {
    const { networkHelpers } = await network.getOrCreate();
    const { securityCouncil, nonAuthorizedAccount } = await networkHelpers.loadFixture(getAccountsFixture);
    const { linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture);
    const { mimc } = await networkHelpers.loadFixture(deployMimcFixture);

    const { addressFilter } = await deployAddressFilter(securityCouncil.address, [nonAuthorizedAccount.address]);

    // The constructor now also takes the L2 block duration and the block number deadline buffer.
    const forcedTransactionGateway = await deployFromFactory(
      "ForcedTransactionGateway",
      [
        linethRollup.address,
        CHAIN_ID,
        290n,
        MAX_GAS_LIMIT,
        MAX_INPUT_LENGTH_LIMIT,
        securityCouncil.address,
        addressFilter.address,
        L2_BLOCK_DURATION_SECONDS,
        BLOCK_NUMBER_DEADLINE_BUFFER,
      ],
      { libraries: { Mimc: mimc.address } },
    );

    return { linethRollup, forcedTransactionGateway, addressFilter, mimc };
  }
});
