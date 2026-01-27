import {
  JsonRpcProvider,
  Signature,
  Signer,
  TransactionReceipt,
  TransactionRequest,
  TransactionResponse,
  ethers,
  randomBytes,
} from "ethers";
import {
  TEST_ADDRESS_1,
  TEST_BLOCK_HASH,
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_MESSAGE_HASH,
  TEST_TRANSACTION_HASH,
} from "./constants/common";
import {
  DEFAULT_ENFORCE_MAX_GAS_FEE,
  DEFAULT_GAS_ESTIMATION_PERCENTILE,
  DEFAULT_L2_MESSAGE_TREE_DEPTH,
  DEFAULT_MAX_FEE_PER_GAS_CAP,
  L2_MERKLE_TREE_ADDED_EVENT_SIGNATURE,
  L2_MESSAGING_BLOCK_ANCHORED_EVENT_SIGNATURE,
  MESSAGE_SENT_EVENT_SIGNATURE,
} from "../../core/constants";
import { Message, SDKMode } from "../../core/types";
import {
  LineaRollupClient,
  EthersLineaRollupLogClient,
  LineaRollupMessageRetriever,
  MerkleTreeService,
} from "../../clients/ethereum";
import {
  L2MessageServiceClient,
  EthersL2MessageServiceLogClient,
  L2MessageServiceMessageRetriever,
} from "../../clients/linea";
import { DefaultGasProvider, GasProvider } from "../../clients/gas";
import { LineaProvider, Provider } from "../../clients/providers";
import { Direction } from "../../core/enums";

export const getTestProvider = () => {
  return new JsonRpcProvider("http://localhost:8545");
};

const mocks = new Map();

export const mockProperty = <T extends object, K extends keyof T>(object: T, property: K, value: T[K]) => {
  const descriptor = Object.getOwnPropertyDescriptor(object, property);
  const mocksForThisObject = mocks.get(object) || {};
  mocksForThisObject[property] = descriptor;
  mocks.set(object, mocksForThisObject);
  Object.defineProperty(object, property, { get: () => value });
};

export const undoMockProperty = <T extends object, K extends keyof T>(object: T, property: K) => {
  Object.defineProperty(object, property, mocks.get(object)[property]);
};

export const generateHexString = (length: number): string => ethers.hexlify(ethers.randomBytes(length));

export const generateTransactionReceipt = (overrides?: Partial<TransactionReceipt>): TransactionReceipt => {
  return new TransactionReceipt(
    {
      hash: TEST_TRANSACTION_HASH,
      blockHash: TEST_BLOCK_HASH,
      to: TEST_CONTRACT_ADDRESS_1,
      from: TEST_ADDRESS_1,
      contractAddress: TEST_CONTRACT_ADDRESS_1,
      index: 0,
      gasUsed: 70_000n,
      logsBloom: "",
      logs: [
        {
          transactionIndex: 0,
          blockNumber: 100_000,
          removed: false,
          transactionHash: TEST_TRANSACTION_HASH,
          address: TEST_CONTRACT_ADDRESS_1,
          topics: [
            MESSAGE_SENT_EVENT_SIGNATURE,
            `0x000000000000000000000000${TEST_ADDRESS_1.slice(2)}`,
            `0x000000000000000000000000${TEST_ADDRESS_1.slice(2)}`,
            TEST_MESSAGE_HASH,
          ],
          data: "0x00000000000000000000000000000000000000000000000000038d7ea4c68000000000000000000000000000000000000000000000000000015fb7f9b8c3800000000000000000000000000000000000000000000000000000000000000003d700000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000",
          index: 0,
          blockHash: TEST_BLOCK_HASH,
        },
      ],
      blockNumber: 100_000,
      confirmations: 1,
      cumulativeGasUsed: 75_000n,
      effectiveGasPrice: 30_000n,
      root: "",
      type: 2,
      status: 1,
      ...overrides,
    },
    getTestProvider(),
  );
};

export const generateTransactionReceiptWithLogs = (
  overrides?: Partial<TransactionReceipt>,
  logs: ethers.Log[] = [],
): TransactionReceipt => {
  return new TransactionReceipt(
    {
      hash: TEST_TRANSACTION_HASH,
      blockHash: TEST_BLOCK_HASH,
      to: TEST_CONTRACT_ADDRESS_1,
      from: TEST_ADDRESS_1,
      contractAddress: TEST_CONTRACT_ADDRESS_1,
      index: 0,
      gasUsed: 70_000n,
      logsBloom: "",
      logs,
      blockNumber: 100_000,
      confirmations: 1,
      cumulativeGasUsed: 75_000n,
      effectiveGasPrice: 30_000n,
      root: "",
      type: 2,
      status: 1,
      ...overrides,
    },
    getTestProvider(),
  );
};

export const generateTransactionResponse = (overrides?: Partial<TransactionResponse>): TransactionResponse => {
  return new TransactionResponse(
    {
      hash: TEST_TRANSACTION_HASH,
      type: 2,
      accessList: null,
      index: 0,
      blockHash: TEST_BLOCK_HASH,
      blockNumber: 1077297,
      confirmations: async () => 212238,
      from: TEST_ADDRESS_1,
      gasPrice: BigInt("3492211493612"),
      gasLimit: BigInt("74959"),
      to: TEST_CONTRACT_ADDRESS_2,
      value: BigInt("4313771350571206145"),
      maxPriorityFeePerGas: 50_000_000n,
      maxFeePerGas: 100_000_000n,
      nonce: 40,
      signature: Signature.from(),
      data: `0x9f3ce55a000000000000000000000000${TEST_ADDRESS_1.slice(
        2,
      )}000000000000000000000000000000000000000000000000002386f26fc1000000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000`,
      chainId: 59140n,
      ...overrides,
    },
    getTestProvider(),
  );
};

export const generateMessage = (overrides?: Partial<Message>): Message => {
  return {
    messageSender: TEST_ADDRESS_1,
    destination: TEST_CONTRACT_ADDRESS_1,
    fee: 10n,
    value: 2n,
    messageNonce: 1n,
    calldata: "0x",
    messageHash: TEST_MESSAGE_HASH,
    ...overrides,
  };
};

export function generateLineaRollupClient(
  l1Provider: Provider,
  l2Provider: LineaProvider,
  l1ContractAddress: string,
  l2ContractAddres: string,
  mode: SDKMode,
  signer?: Signer,
  gasFeesOptions?: {
    maxFeePerGasCap?: bigint;
    gasEstimationPercentile?: number;
    enforceMaxGasFee?: boolean;
  },
): {
  lineaRollupClient: LineaRollupClient;
  lineaRollupLogClient: EthersLineaRollupLogClient;
  l2MessageServiceLogClient: EthersL2MessageServiceLogClient;
  gasProvider: DefaultGasProvider;
  messageRetriever: LineaRollupMessageRetriever;
  merkleTreeService: MerkleTreeService;
} {
  const lineaRollupLogClient = new EthersLineaRollupLogClient(l1Provider, l1ContractAddress);
  const l2MessageServiceLogClient = new EthersL2MessageServiceLogClient(l2Provider, l2ContractAddres);
  const gasProvider = new DefaultGasProvider(l1Provider, {
    maxFeePerGasCap: gasFeesOptions?.maxFeePerGasCap ?? DEFAULT_MAX_FEE_PER_GAS_CAP,
    gasEstimationPercentile: gasFeesOptions?.gasEstimationPercentile ?? DEFAULT_GAS_ESTIMATION_PERCENTILE,
    enforceMaxGasFee: gasFeesOptions?.enforceMaxGasFee ?? DEFAULT_ENFORCE_MAX_GAS_FEE,
  });
  const messageRetriever = new LineaRollupMessageRetriever(l1Provider, lineaRollupLogClient, l1ContractAddress);
  const merkleTreeService = new MerkleTreeService(
    l1Provider,
    l1ContractAddress,
    lineaRollupLogClient,
    l2MessageServiceLogClient,
    DEFAULT_L2_MESSAGE_TREE_DEPTH,
  );
  const lineaRollupClient = new LineaRollupClient(
    l1Provider,
    l1ContractAddress,
    lineaRollupLogClient,
    l2MessageServiceLogClient,
    gasProvider,
    messageRetriever,
    merkleTreeService,
    mode,
    signer,
  );

  return {
    lineaRollupClient,
    lineaRollupLogClient,
    l2MessageServiceLogClient,
    gasProvider,
    messageRetriever,
    merkleTreeService,
  };
}

export function generateL2MessageServiceClient(
  l2Provider: LineaProvider,
  l2ContractAddress: string,
  mode: SDKMode,
  signer?: Signer,
  gasFeesOptions?: {
    maxFeePerGasCap?: bigint;
    gasEstimationPercentile?: number;
    enforceMaxGasFee?: boolean;
    enableLineaEstimateGas?: boolean;
  },
): {
  l2MessageServiceClient: L2MessageServiceClient;
  l2MessageServiceLogClient: EthersL2MessageServiceLogClient;
  gasProvider: GasProvider;
  messageRetriever: L2MessageServiceMessageRetriever;
} {
  const l2MessageServiceLogClient = new EthersL2MessageServiceLogClient(l2Provider, l2ContractAddress);

  const messageRetriever = new L2MessageServiceMessageRetriever(
    l2Provider,
    l2MessageServiceLogClient,
    l2ContractAddress,
  );

  const gasProvider = new GasProvider(l2Provider, {
    maxFeePerGasCap: gasFeesOptions?.maxFeePerGasCap ?? DEFAULT_MAX_FEE_PER_GAS_CAP,
    enforceMaxGasFee: gasFeesOptions?.enforceMaxGasFee ?? DEFAULT_ENFORCE_MAX_GAS_FEE,
    gasEstimationPercentile: gasFeesOptions?.gasEstimationPercentile ?? DEFAULT_GAS_ESTIMATION_PERCENTILE,
    direction: Direction.L1_TO_L2,
    enableLineaEstimateGas: gasFeesOptions?.enableLineaEstimateGas ?? false,
  });

  const l2MessageServiceClient = new L2MessageServiceClient(
    l2Provider,
    l2ContractAddress,
    messageRetriever,
    gasProvider,
    mode,
    signer,
  );

  return { l2MessageServiceClient, l2MessageServiceLogClient, gasProvider, messageRetriever };
}

export const generateL2MessagingBlockAnchoredLog = (l2Block: bigint, overrides?: Partial<ethers.Log>): ethers.Log => {
  return new ethers.Log(
    {
      transactionIndex: 0,
      blockNumber: 100_000,
      removed: false,
      transactionHash: TEST_TRANSACTION_HASH,
      address: TEST_CONTRACT_ADDRESS_1,
      topics: [L2_MESSAGING_BLOCK_ANCHORED_EVENT_SIGNATURE, ethers.zeroPadValue(ethers.toBeHex(l2Block), 32)],
      data: "0x",
      index: 0,
      blockHash: TEST_BLOCK_HASH,
      ...overrides,
    },
    getTestProvider(),
  );
};

export const generateL2MerkleTreeAddedLog = (
  l2MerkleRoot: string,
  treeDepth: number,
  overrides?: Partial<ethers.Log>,
): ethers.Log => {
  return new ethers.Log(
    {
      transactionIndex: 0,
      blockNumber: 100_000,
      removed: false,
      transactionHash: TEST_TRANSACTION_HASH,
      address: TEST_CONTRACT_ADDRESS_1,
      topics: [
        L2_MERKLE_TREE_ADDED_EVENT_SIGNATURE,
        ethers.hexlify(l2MerkleRoot),
        ethers.zeroPadValue(ethers.toBeHex(treeDepth), 32),
      ],
      data: "0x",
      index: 0,
      blockHash: TEST_BLOCK_HASH,
      ...overrides,
    },
    getTestProvider(),
  );
};

export const generateTransactionRequest = (overrides?: Partial<TransactionRequest>): TransactionRequest => ({
  from: TEST_ADDRESS_1,
  to: TEST_CONTRACT_ADDRESS_1,
  value: ethers.parseEther("1"),
  data: ethers.hexlify(randomBytes(32)),
  ...overrides,
});
