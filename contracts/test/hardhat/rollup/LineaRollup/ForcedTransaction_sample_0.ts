import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { ForcedTransactionGateway, TestLineaRollup } from "contracts/typechain-types";
import { ethers, network } from "hardhat";

import forcedTx0 from "../../_testData/eip1559RlpEncoderTransactions/forced-transaction-0.json";
import {
  DEFAULT_LAST_FINALIZED_TIMESTAMP,
  FORCED_TRANSACTION_SENDER_ROLE,
  HASH_ZERO,
  MAX_INPUT_LENGTH_LIMIT,
} from "../../common/constants";
import { buildEip1559Transaction } from "../../common/helpers";
import { getAccountsFixture, deployLineaRollupFixture, deployMimcFixture, deployAddressFilter } from "../helpers";

describe.skip("Linea Rollup contract: Forced Transactions", () => {
  const MAX_GAS_LIMIT = 10_000_000n;
  const CHAIN_ID = 789979n;

  let lineaRollup: TestLineaRollup;
  let forcedTransactionGateway: ForcedTransactionGateway;

  let securityCouncil: SignerWithAddress;
  let defaultFinalizedState = {
    messageNumber: 0n,
    messageRollingHash: HASH_ZERO,
    forcedTransactionNumber: 0n,
    forcedTransactionRollingHash: HASH_ZERO,
    timestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
  };

  before(async () => {
    await network.provider.send("hardhat_reset");
    ({ securityCouncil } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({ lineaRollup, forcedTransactionGateway } = await loadFixture(deployForcedTransactionGatewayFixtureLocally));

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

  describe("Adding forced transactions", () => {
    it("Should submit the forced transaction with calldata", async () => {
      await forcedTransactionGateway.submitForcedTransaction(
        buildEip1559Transaction(forcedTx0.Transaction),
        defaultFinalizedState,
      );
    });
  });

  async function deployForcedTransactionGatewayFixtureLocally() {
    const { securityCouncil, nonAuthorizedAccount } = await loadFixture(getAccountsFixture);
    const { lineaRollup } = await loadFixture(deployLineaRollupFixture);
    const { mimc } = await loadFixture(deployMimcFixture);

    const forcedTransactionGatewayFactory = await ethers.getContractFactory("ForcedTransactionGateway", {
      libraries: { Mimc: await mimc.getAddress() },
    });

    const { addressFilter } = await deployAddressFilter(securityCouncil.address, [nonAuthorizedAccount.address]);

    const forcedTransactionGateway = (await forcedTransactionGatewayFactory.deploy(
      await lineaRollup.getAddress(),
      CHAIN_ID,
      290n,
      MAX_GAS_LIMIT,
      MAX_INPUT_LENGTH_LIMIT,
      securityCouncil.address,
      await addressFilter.getAddress(),
    )) as unknown as ForcedTransactionGateway;

    await forcedTransactionGateway.waitForDeployment();

    return { lineaRollup, forcedTransactionGateway, addressFilter, mimc };
  }
});
