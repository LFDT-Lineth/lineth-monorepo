import { etherToWei } from "@consensys/linea-shared-utils";
import { encodeDeployData, formatEther, parseGwei } from "viem";

import { DEPLOYER_ACCOUNT_INDEX, LIVENESS_ACCOUNT_INDEX } from "../../common/constants";
import { deployContract, linkBytecode } from "../../common/deployments";
import { estimateLineaGas } from "../../common/utils";
import {
  DummyContractAbi,
  DummyContractAbiBytecode,
  LineaSequencerUptimeFeedAbi,
  LineaSequencerUptimeFeedAbiBytecode,
  Poseidon2Abi,
  Poseidon2AbiBytecode,
  SparseMerkleProofAbi,
  SparseMerkleProofAbiBytecode,
  SparseMerkleProofAbiLinkReferences,
  TestContractAbi,
  TestContractAbiBytecode,
} from "../../generated";
import { L2RpcEndpoint } from "../clients/l2-client";

import type { TestContext } from "../setup";

export async function ensureOnceOffPrerequisites(context: TestContext, logger: { info: (msg: string) => void }) {
  const l1PublicClient = context.l1PublicClient();
  const dummyContractCode = await l1PublicClient.getCode({
    address: context.l1Contracts.dummyContract(l1PublicClient).address,
  });

  if (!dummyContractCode) {
    logger.info("Configuring once-off prerequisite contracts");
    await configureOnceOffPrerequisities(context, logger);
  }
}

async function configureOnceOffPrerequisities(context: TestContext, logger: { info: (msg: string) => void }) {
  const account = context.getL1AccountManager().whaleAccount(DEPLOYER_ACCOUNT_INDEX);
  const l2Account = context.getL2AccountManager().whaleAccount(DEPLOYER_ACCOUNT_INDEX);

  const l1PublicClient = context.l1PublicClient();
  const l1WalletClient = context.l1WalletClient({ account });
  const l2SequencerPublicClient = context.l2PublicClient({ type: L2RpcEndpoint.Sequencer });
  const l2SequencerWalletClient = context.l2WalletClient({ type: L2RpcEndpoint.Sequencer, account: l2Account });
  /**
   * Account index {@link LIVENESS_ACCOUNT_INDEX} is reserved for liveness testing to avoid nonce conflicts with other concurrent e2e tests.
   */
  const livenessSignerAccount = context.getL2AccountManager().whaleAccount(LIVENESS_ACCOUNT_INDEX);

  const lineaRollup = context.l1Contracts.lineaRollup(l1WalletClient);

  const [l1AccountNonce, l2AccountNonce] = await Promise.all([
    l1PublicClient.getTransactionCount({ address: account.address }),
    l2SequencerPublicClient?.getTransactionCount({ address: l2Account.address }),
  ]);

  const fee = etherToWei("3");
  const to = "0x8D97689C9818892B700e27F316cc3E41e17fBeb9";
  const calldata = "0x";

  const l2BesuNodePublicClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode })!;
  const [
    { maxPriorityFeePerGas, maxFeePerGas },
    { maxPriorityFeePerGas: maxPriorityFeePerGasTestContract, maxFeePerGas: maxFeePerGasTestContract },
    { maxPriorityFeePerGas: maxPriorityFeePerGasPoseidon2, maxFeePerGas: maxFeePerGasPoseidon2 },
    {
      maxPriorityFeePerGas: maxPriorityFeePerGasLineaSequencerUptimeFeed,
      maxFeePerGas: maxFeePerGasLineaSequencerUptimeFeed,
    },
  ] = await Promise.all([
    estimateLineaGas(l2BesuNodePublicClient, {
      account: l2Account.address,
      data: encodeDeployData({
        abi: DummyContractAbi,
        bytecode: DummyContractAbiBytecode,
      }),
      value: 0n,
    }),
    estimateLineaGas(l2BesuNodePublicClient, {
      account: l2Account.address,
      data: encodeDeployData({
        abi: TestContractAbi,
        bytecode: TestContractAbiBytecode,
      }),
      value: 0n,
    }),
    estimateLineaGas(l2BesuNodePublicClient, {
      account: l2Account.address,
      data: encodeDeployData({
        abi: Poseidon2Abi,
        bytecode: Poseidon2AbiBytecode,
      }),
      value: 0n,
    }),
    estimateLineaGas(l2BesuNodePublicClient, {
      account: l2Account.address,
      data: encodeDeployData({
        abi: LineaSequencerUptimeFeedAbi,
        bytecode: LineaSequencerUptimeFeedAbiBytecode,
        args: [false, livenessSignerAccount.address, livenessSignerAccount.address],
      }),
      value: 0n,
    }),
  ]);

  const [
    dummyContractAddress,
    l2DummyContractAddress,
    l2TestContractAddress,
    l2Poseidon2ContractAddress,
    l2LineaSequencerUptimeFeedContractAddress,
  ] = await Promise.all([
    deployContract(l1WalletClient, {
      account,
      chain: l1WalletClient.chain,
      abi: DummyContractAbi,
      bytecode: DummyContractAbiBytecode,
      nonce: l1AccountNonce,
    }),
    deployContract(l2SequencerWalletClient!, {
      account: l2Account,
      chain: l2SequencerWalletClient!.chain,
      abi: DummyContractAbi,
      bytecode: DummyContractAbiBytecode,
      nonce: l2AccountNonce,
      maxPriorityFeePerGas,
      maxFeePerGas,
    }),
    deployContract(l2SequencerWalletClient!, {
      account: l2Account,
      chain: l2SequencerWalletClient!.chain,
      abi: TestContractAbi,
      bytecode: TestContractAbiBytecode,
      nonce: l2AccountNonce! + 1,
      maxPriorityFeePerGas: maxPriorityFeePerGasTestContract,
      maxFeePerGas: maxFeePerGasTestContract,
    }),
    deployContract(l2SequencerWalletClient!, {
      account: l2Account,
      chain: l2SequencerWalletClient!.chain,
      abi: Poseidon2Abi,
      bytecode: Poseidon2AbiBytecode,
      nonce: l2AccountNonce! + 2,
      maxPriorityFeePerGas: maxPriorityFeePerGasPoseidon2,
      maxFeePerGas: maxFeePerGasPoseidon2,
    }),
    deployContract(l2SequencerWalletClient!, {
      account: l2Account,
      chain: l2SequencerWalletClient!.chain,
      abi: LineaSequencerUptimeFeedAbi,
      bytecode: LineaSequencerUptimeFeedAbiBytecode,
      args: [false, livenessSignerAccount.address, livenessSignerAccount.address],
      nonce: l2AccountNonce! + 3,
      maxPriorityFeePerGas: maxPriorityFeePerGasLineaSequencerUptimeFeed,
      maxFeePerGas: maxFeePerGasLineaSequencerUptimeFeed,
    }),
    l1PublicClient.waitForTransactionReceipt({
      hash: await lineaRollup.write.sendMessage([to, fee, calldata], {
        value: etherToWei("500"),
        gasPrice: parseGwei("300"),
        nonce: l1AccountNonce + 1,
      }),
    }),
  ]);

  const lineaRollupBalance = await l1PublicClient.getBalance({ address: lineaRollup.address });
  if (lineaRollupBalance < etherToWei("500")) {
    throw new Error("LineaRollup funding failed");
  }

  const { maxPriorityFeePerGas: maxPriorityFeePerGasSparseMerkleProof, maxFeePerGas: maxFeePerGasSparseMerkleProof } =
    await estimateLineaGas(l2BesuNodePublicClient, {
      account: l2Account.address,
      data: encodeDeployData({
        abi: SparseMerkleProofAbi,
        bytecode: linkBytecode(SparseMerkleProofAbiBytecode, SparseMerkleProofAbiLinkReferences, {
          Poseidon2: l2Poseidon2ContractAddress,
        }),
      }),
      value: 0n,
    });

  const l2SparseMerkleProofContractAddress = await deployContract(l2SequencerWalletClient!, {
    account: l2Account,
    chain: l2SequencerWalletClient!.chain,
    abi: SparseMerkleProofAbi,
    bytecode: linkBytecode(SparseMerkleProofAbiBytecode, SparseMerkleProofAbiLinkReferences, {
      Poseidon2: l2Poseidon2ContractAddress,
    }),
    nonce: l2AccountNonce! + 4,
    maxPriorityFeePerGas: maxPriorityFeePerGasSparseMerkleProof,
    maxFeePerGas: maxFeePerGasSparseMerkleProof,
  });

  logger.info(`L1 Dummy contract deployed. address=${dummyContractAddress}`);
  logger.info(`L2 Dummy contract deployed. address=${l2DummyContractAddress}`);
  logger.info(`L2 Test contract deployed. address=${l2TestContractAddress}`);
  logger.info(`L2 Poseidon2 contract deployed. address=${l2Poseidon2ContractAddress}`);
  logger.info(`L2 LineaSequencerUptimeFeed contract deployed. address=${l2LineaSequencerUptimeFeedContractAddress}`);
  logger.info(`L2 SparseMerkleProof contract deployed. address=${l2SparseMerkleProofContractAddress}`);
  logger.info(`LineaRollup funded with ${formatEther(lineaRollupBalance)} ETH on L1`);
}
