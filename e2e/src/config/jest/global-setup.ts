/* eslint-disable no-var */
import { ethers, toBeHex } from "ethers";
import { config } from "../tests-config";
import { deployContract } from "../../common/deployments";
import {
  DummyContract__factory,
  LineaSequencerUptimeFeed__factory,
  Mimc__factory,
  SparseMerkleProof__factory,
  TestContract__factory,
} from "../../typechain";
import { etherToWei, LineaEstimateGasClient, sendTransactionsToGenerateTrafficWithInterval } from "../../common/utils";
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
  // accIndex set as 1 to use a different whale account than the one deployed the contracts to
  // avoid transaction discard in sequencer
  const pollingAccount = await config.getL2AccountManager().generateAccount(etherToWei("200"), 1);
  const stopPolling = await sendTransactionsToGenerateTrafficWithInterval(pollingAccount, 2_000);

  global.stopL2TrafficGeneration = stopPolling;
};

async function configureOnceOffPrerequisities() {
  const account = config.getL1AccountManager().whaleAccount(0);
  const l2Account = config.getL2AccountManager().whaleAccount(0).connect(config.getL2SequencerProvider()!);
  const lineaRollup = config.getLineaRollupContract(account);

  const [l1AccountNonce, l2AccountNonce] = await Promise.all([account.getNonce(), l2Account.getNonce()]);

  const fee = etherToWei("3");
  const to = "0x8D97689C9818892B700e27F316cc3E41e17fBeb9";
  const calldata = "0x";

  const lineaEstimateGasClient = new LineaEstimateGasClient(config.getL2BesuNodeEndpoint()!);
  const [
    { maxPriorityFeePerGas, maxFeePerGas },
    { maxPriorityFeePerGas: maxPriorityFeePerGasTestContract, maxFeePerGas: maxFeePerGasTestContract },
    { maxPriorityFeePerGas: maxPriorityFeePerGasMimc, maxFeePerGas: maxFeePerGasMimc },
  ] = await Promise.all([
    lineaEstimateGasClient.lineaEstimateGas(
      await l2Account.getAddress(),
      undefined,
      new DummyContract__factory().interface.encodeDeploy(),
      toBeHex(0),
    ),
    lineaEstimateGasClient.lineaEstimateGas(
      await l2Account.getAddress(),
      undefined,
      new TestContract__factory().interface.encodeDeploy(),
      toBeHex(0),
    ),
    lineaEstimateGasClient.lineaEstimateGas(
      await l2Account.getAddress(),
      undefined,
      new Mimc__factory().interface.encodeDeploy(),
      toBeHex(0),
    ),
  ]);

  const [dummyContract, l2DummyContract, l2TestContract, l2MimcContract, l2LineaSequencerUptimeFeedContract] =
    await Promise.all([
      deployContract(new DummyContract__factory(), account, [{ nonce: l1AccountNonce }]),
      deployContract(new DummyContract__factory(), l2Account, [
        { nonce: l2AccountNonce, maxPriorityFeePerGas, maxFeePerGas },
      ]),
      deployContract(new TestContract__factory(), l2Account, [
        {
          nonce: l2AccountNonce + 1,
          maxPriorityFeePerGas: maxPriorityFeePerGasTestContract,
          maxFeePerGas: maxFeePerGasTestContract,
        },
      ]),
      deployContract(new Mimc__factory(), l2Account, [
        { nonce: l2AccountNonce + 2, maxPriorityFeePerGas: maxPriorityFeePerGasMimc, maxFeePerGas: maxFeePerGasMimc },
      ]),
      deployContract(new LineaSequencerUptimeFeed__factory(), l2Account, [
        { nonce: l2AccountNonce + 3, maxPriorityFeePerGas: maxPriorityFeePerGasMimc, maxFeePerGas: maxFeePerGasMimc },
      ]),

      // Send ETH to the LineaRollup contract
      (
        await lineaRollup.sendMessage(to, fee, calldata, {
          value: etherToWei("500"),
          gasPrice: ethers.parseUnits("300", "gwei"),
          nonce: l1AccountNonce + 1,
        })
      ).wait(),
    ]);

  const l2MimcContractAddress = await l2MimcContract.getAddress();

  const { maxPriorityFeePerGas: maxPriorityFeePerGasSparseMerkleProof, maxFeePerGas: maxFeePerGasSparseMerkleProof } =
    await lineaEstimateGasClient.lineaEstimateGas(
      await l2Account.getAddress(),
      undefined,
      new SparseMerkleProof__factory({ "contracts/Mimc.sol:Mimc": l2MimcContractAddress }).interface.encodeDeploy(),
      toBeHex(0),
    );

  const l2SparseMerkleProofContract = await deployContract(
    new SparseMerkleProof__factory({ "contracts/Mimc.sol:Mimc": l2MimcContractAddress }),
    l2Account,
    [
      {
        nonce: l2AccountNonce + 4,
        maxPriorityFeePerGas: maxPriorityFeePerGasSparseMerkleProof,
        maxFeePerGas: maxFeePerGasSparseMerkleProof,
      },
    ],
  );

  logger.info(`L1 Dummy contract deployed. address=${await dummyContract.getAddress()}`);
  logger.info(`L2 Dummy contract deployed. address=${await l2DummyContract.getAddress()}`);
  logger.info(`L2 Test contract deployed. address=${await l2TestContract.getAddress()}`);
  logger.info(`L2 Mimc contract deployed. address=${l2MimcContractAddress}`);
  logger.info(
    `L2 LineaSequencerUptimeFeed contract deployed. address=${await l2LineaSequencerUptimeFeedContract.getAddress()}`,
  );
  logger.info(`L2 SparseMerkleProof contract deployed. address=${await l2SparseMerkleProofContract.getAddress()}`);
}
