/* eslint-disable no-var */
import { ethers } from "ethers";
import { config } from "../tests-config";
import { deployContract } from "../../common/deployments";
import { DummyContract__factory, TestContract__factory } from "../../typechain";
import { etherToWei, sendTransactionsToGenerateTrafficWithInterval } from "../../common/utils";
import { EMPTY_CONTRACT_CODE } from "../../common/constants";
import { createTestLogger } from "../logger";

const logger = createTestLogger();

export default async (): Promise<void> => {
  const dummyContractCode = await config.getL1Provider().getCode(config.getL1DummyContractAddress());

  // If this is empty, we have not deployed and prerequisites or configured token bridges.
  if (dummyContractCode === EMPTY_CONTRACT_CODE) {
    logger.info("Configuring once-off prerequisite contracts");
    await configureOnceOffPrerequisities();
  }

  logger.info("Generating L2 traffic...");
  const pollingAccount = await config.getL2AccountManager().generateAccount(etherToWei("200"));
  const stopPolling = await sendTransactionsToGenerateTrafficWithInterval(pollingAccount, 2_000);

  global.stopL2TrafficGeneration = stopPolling;
};

async function configureOnceOffPrerequisities() {
  const account = config.getL1AccountManager().whaleAccount(0);
  const l2Account = config.getL2AccountManager().whaleAccount(0);
  const lineaRollup = config.getLineaRollupContract(account);

  const [l1AccountNonce, l2AccountNonce] = await Promise.all([account.getNonce(), l2Account.getNonce()]);

  const fee = etherToWei("3");
  const to = "0x8D97689C9818892B700e27F316cc3E41e17fBeb9";
  const calldata = "0x";

  const [dummyContract, l2DummyContract, l2TestContract] = await Promise.all([
    deployContract(new DummyContract__factory(), account, [{ nonce: l1AccountNonce }]),
    deployContract(new DummyContract__factory(), l2Account, [{ nonce: l2AccountNonce }]),
    deployContract(new TestContract__factory(), l2Account, [{ nonce: l2AccountNonce + 1 }]),

    // Send ETH to the LineaRollup contract
    (
      await lineaRollup.sendMessage(to, fee, calldata, {
        value: etherToWei("500"),
        gasPrice: ethers.parseUnits("300", "gwei"),
        nonce: l1AccountNonce + 1,
      })
    ).wait(),
  ]);

  logger.info(`L1 Dummy contract deployed. address=${await dummyContract.getAddress()}`);
  logger.info(`L2 Dummy contract deployed. address=${await l2DummyContract.getAddress()}`);
  logger.info(`L2 Test contract deployed. address=${await l2TestContract.getAddress()}`);
}
