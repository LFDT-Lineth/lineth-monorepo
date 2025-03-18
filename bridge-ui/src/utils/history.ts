import { Address, decodeAbiParameters } from "viem";
import { compareAsc, fromUnixTime, subDays } from "date-fns";
import { getPublicClient } from "@wagmi/core";
import { LineaSDK, OnChainMessageStatus } from "@consensys/linea-sdk";
import { config as wagmiConfig } from "@/lib/wagmi";
import { config } from "@/config";
import { Proof } from "@consensys/linea-sdk/dist/lib/sdk/merkleTree/types";
import { defaultTokensConfig } from "@/stores";
import { LineaSDKContracts } from "@/hooks";
import { Chain, ChainLayer, Token, TransactionStatus, BridgingInitiatedV2Event, MessageSentEvent } from "@/types";
import {
  CCTP_TOKEN_MESSENGER,
  eventETH,
  eventERC20V2,
  eventUSDC,
  getCCTPClaimTx,
  isCCTPNonceUsed,
  getCCTPTransactionStatus,
} from "@/utils";
import { DepositForBurnEvent } from "@/types/events";
import { fetchCctpAttestation } from "@/services/cctp";

type TransactionHistoryParams = {
  lineaSDK: LineaSDK;
  lineaSDKContracts: LineaSDKContracts;
  fromChain: Chain;
  toChain: Chain;
  address: Address;
  tokens: Token[];
};

export type NativeBridgeMessage = {
  from: Address;
  to: Address;
  fee: bigint;
  value: bigint;
  nonce: bigint;
  calldata: string;
  messageHash: string;
  proof?: Proof;
  amountSent: bigint;
};

// Params expected for `receiveMessage` as per https://developers.circle.com/stablecoins/transfer-usdc-on-testnet-from-ethereum-to-avalanche
export type CCTPV2BridgeMessage = {
  message: string;
  attestation: string;
  amountSent: bigint;
};
export interface BridgeTransaction {
  type: "ETH" | "ERC20";
  status: TransactionStatus;
  timestamp: bigint;
  fromChain: Chain;
  toChain: Chain;
  token: Token;
  message: NativeBridgeMessage | CCTPV2BridgeMessage;
  bridgingTx: string;
  claimingTx?: string;
}

export async function fetchTransactionsHistory({
  lineaSDK,
  lineaSDKContracts,
  fromChain,
  toChain,
  address,
  tokens,
}: TransactionHistoryParams): Promise<BridgeTransaction[]> {
  const events = await Promise.all([
    fetchBridgeEvents(lineaSDK, lineaSDKContracts, fromChain, toChain, address, tokens),
    fetchBridgeEvents(lineaSDK, lineaSDKContracts, toChain, fromChain, address, tokens),
  ]);
  return events.flat().sort((a, b) => Number(b.timestamp.toString()) - Number(a.timestamp.toString()));
}

async function fetchBridgeEvents(
  lineaSDK: LineaSDK,
  lineaSDKContracts: LineaSDKContracts,
  fromChain: Chain,
  toChain: Chain,
  address: Address,
  tokens: Token[],
): Promise<BridgeTransaction[]> {
  const [ethEvents, erc20Events, cctpEvents] = await Promise.all([
    fetchETHBridgeEvents(lineaSDK, lineaSDKContracts, address, fromChain, toChain, tokens),
    fetchERC20BridgeEvents(lineaSDK, lineaSDKContracts, address, fromChain, toChain, tokens),
    // Feature toggle for CCTP, will filter out USDC transactions if isCCTPEnabled == false
    config.isCCTPEnabled ? fetchCCTPBridgeEvents(address, fromChain, toChain, tokens) : [],
  ]);

  return [...ethEvents, ...erc20Events, ...cctpEvents];
}

async function fetchETHBridgeEvents(
  lineaSDK: LineaSDK,
  lineaSDKContracts: LineaSDKContracts,
  address: Address,
  fromChain: Chain,
  toChain: Chain,
  tokens: Token[],
): Promise<BridgeTransaction[]> {
  const transactionsMap = new Map<string, BridgeTransaction>();

  const client = getPublicClient(wagmiConfig, {
    chainId: fromChain.id,
  });

  const contract = fromChain.layer === ChainLayer.L1 ? lineaSDK.getL2Contract() : lineaSDK.getL1Contract();

  const messageServiceAddress = fromChain.messageServiceAddress;
  const [ethLogsForSender, ethLogsForRecipient] = await Promise.all([<Promise<MessageSentEvent[]>>client.getLogs({
      event: eventETH,
      fromBlock: "earliest",
      toBlock: "latest",
      address: messageServiceAddress,
      args: {
        _from: address,
      },
    }), <Promise<MessageSentEvent[]>>client.getLogs({
      event: eventETH,
      fromBlock: "earliest",
      toBlock: "latest",
      address: messageServiceAddress,
      args: {
        _to: address,
      },
    })]);

  const messageSentLogs = [...ethLogsForSender, ...ethLogsForRecipient];

  const uniqueLogsMap = new Map<string, (typeof messageSentLogs)[0]>();
  for (const log of messageSentLogs) {
    const uniqueKey = `${log.args._from}-${log.args._to}-${log.transactionHash}`;
    if (!uniqueLogsMap.has(uniqueKey)) {
      uniqueLogsMap.set(uniqueKey, log);
    }
  }

  const currentTimestamp = new Date();

  await Promise.all(
    Array.from(uniqueLogsMap.values()).map(async (log) => {
      const messageHash = log.args._messageHash;

      const block = await client.getBlock({ blockNumber: log.blockNumber, includeTransactions: false });

      if (compareAsc(fromUnixTime(Number(block.timestamp.toString())), subDays(currentTimestamp, 90)) === -1) {
        return;
      }

      const [messageStatus, [messageClaimedEvent]] = await Promise.all([
        contract.getMessageStatus(messageHash),
        contract.getEvents(lineaSDKContracts[toChain.layer].contract.filters.MessageClaimed(messageHash)),
      ]);

      const messageProof =
        toChain.layer === ChainLayer.L1 && messageStatus === OnChainMessageStatus.CLAIMABLE
          ? await lineaSDK.getL1ClaimingService().getMessageProof(messageHash)
          : undefined;

      const token = tokens.find((token) => token.type.includes("eth"));
      const uniqueKey = `${log.args._from}-${log.args._to}-${log.transactionHash}`;
      transactionsMap.set(uniqueKey, {
        type: "ETH",
        status: formatStatus(messageStatus),
        token: token || defaultTokensConfig.MAINNET[0],
        fromChain,
        toChain,
        timestamp: block.timestamp,
        bridgingTx: log.transactionHash,
        claimingTx: messageClaimedEvent?.transactionHash,
        message: {
          from: log.args._from,
          to: log.args._to,
          fee: log.args._fee,
          value: log.args._value,
          nonce: log.args._nonce,
          calldata: log.args._calldata,
          messageHash: log.args._messageHash,
          proof: messageProof,
          amountSent: log.args._value,
        },
      });
    }),
  );

  return Array.from(transactionsMap.values());
}

async function fetchERC20BridgeEvents(
  lineaSDK: LineaSDK,
  lineaSDKContracts: LineaSDKContracts,
  address: Address,
  fromChain: Chain,
  toChain: Chain,
  tokens: Token[],
): Promise<BridgeTransaction[]> {
  const transactionsMap = new Map<string, BridgeTransaction>();

  const client = getPublicClient(wagmiConfig, {
    chainId: fromChain.id,
  });

  const [originContract, destinationContract] =
    fromChain.layer === ChainLayer.L1
      ? [lineaSDK.getL1Contract(), lineaSDK.getL2Contract()]
      : [lineaSDK.getL2Contract(), lineaSDK.getL1Contract()];

  const tokenBridgeAddress = fromChain.tokenBridgeAddress;
  const [erc20LogsForSender, erc20LogsForRecipient] = await Promise.all([
    <Promise<BridgingInitiatedV2Event[]>>client.getLogs({
      event: eventERC20V2,
      fromBlock: "earliest",
      toBlock: "latest",
      address: tokenBridgeAddress,
      args: {
        sender: address,
      },
    }),
    <Promise<BridgingInitiatedV2Event[]>>client.getLogs({
      event: eventERC20V2,
      fromBlock: "earliest",
      toBlock: "latest",
      address: tokenBridgeAddress,
      args: {
        recipient: address,
      },
    }),
  ]);

  const erc20Logs = [...erc20LogsForSender, ...erc20LogsForRecipient];

  const uniqueLogsMap = new Map<string, (typeof erc20Logs)[0]>();
  for (const log of erc20Logs) {
    const transactionHash = log.transactionHash;
    if (!uniqueLogsMap.has(transactionHash)) {
      uniqueLogsMap.set(transactionHash, log);
    }
  }

  const currentTimestamp = new Date();

  await Promise.all(
    Array.from(uniqueLogsMap.values()).map(async (log) => {
      const transactionHash = log.transactionHash;

      const block = await client.getBlock({ blockNumber: log.blockNumber, includeTransactions: false });

      if (compareAsc(fromUnixTime(Number(block.timestamp.toString())), subDays(currentTimestamp, 90)) === -1) {
        return;
      }

      const message = await originContract.getMessagesByTransactionHash(transactionHash);
      if (!message || message.length === 0) {
        return;
      }

      const [messageStatus, [messageClaimedEvent]] = await Promise.all([
        destinationContract.getMessageStatus(message[0].messageHash),
        destinationContract.getEvents(
          lineaSDKContracts[toChain.layer].contract.filters.MessageClaimed(message[0].messageHash),
        ),
      ]);

      const messageProof =
        toChain.layer === ChainLayer.L1 && messageStatus === OnChainMessageStatus.CLAIMABLE
          ? await lineaSDK.getL1ClaimingService().getMessageProof(message[0].messageHash)
          : undefined;

      const token = tokens.find(
        (token) =>
          token.L1?.toLowerCase() === log.args.token.toLowerCase() ||
          token.L2?.toLowerCase() === log.args.token.toLowerCase(),
      );

      if (!token) {
        return;
      }

      const [amount] = decodeAbiParameters([{ type: "uint256", name: "amount" }], log.data);

      transactionsMap.set(transactionHash, {
        type: "ERC20",
        status: formatStatus(messageStatus),
        token,
        fromChain,
        toChain,
        timestamp: block.timestamp,
        bridgingTx: log.transactionHash,
        claimingTx: messageClaimedEvent?.transactionHash,
        message: {
          from: message[0].messageSender as Address,
          to: message[0].destination as Address,
          fee: message[0].fee,
          value: message[0].value,
          nonce: message[0].messageNonce,
          calldata: message[0].calldata,
          messageHash: message[0].messageHash,
          proof: messageProof,
          amountSent: amount,
        },
      });
    }),
  );

  return Array.from(transactionsMap.values());
}

async function fetchCCTPBridgeEvents(
  address: Address,
  fromChain: Chain,
  toChain: Chain,
  tokens: Token[],
): Promise<BridgeTransaction[]> {
  const transactionsMap = new Map<string, BridgeTransaction>();
  const fromChainClient = getPublicClient(wagmiConfig, {
    chainId: fromChain.id,
  });
  const toChainClient = getPublicClient(wagmiConfig, {
    chainId: toChain.id,
  });

  const usdcLogs = <DepositForBurnEvent[]>await fromChainClient.getLogs({
    event: eventUSDC,
    fromBlock: "earliest",
    toBlock: "latest",
    address: CCTP_TOKEN_MESSENGER,
    args: {
      depositor: address,
    },
  });

  // TODO - Consider deduplication

  const currentTimestamp = new Date();

  await Promise.all(
    usdcLogs.map(async (log) => {
      const transactionHash = log.transactionHash;

      const block = await fromChainClient.getBlock({ blockNumber: log.blockNumber, includeTransactions: false });

      if (compareAsc(fromUnixTime(Number(block.timestamp.toString())), subDays(currentTimestamp, 90)) === -1) {
        return;
      }

      const token = tokens.find((token) => token.symbol === "USDC" && token.type.includes("bridge-reserved"));
      if (!token) return;

      const attestationApiResp = await fetchCctpAttestation(transactionHash, fromChain.cctpDomain);
      if (!attestationApiResp) return;
      // console.log("attestationApiResp:", attestationApiResp, "transactionHash:", transactionHash);

      const message = attestationApiResp.messages[0];
      if (!message) return;

      const nonce = message.eventNonce;
      // console.log("messageNonce:", nonce);

      const isNonceUsed = await isCCTPNonceUsed(toChainClient, nonce);
      // console.log("isNonceUsed:", isNonceUsed);

      const status = getCCTPTransactionStatus(message.status, isNonceUsed);
      // console.log("status:", status);

      const claimTx = await getCCTPClaimTx(toChainClient, message.status, isNonceUsed, nonce);
      // console.log("claimTx:", claimTx);

      transactionsMap.set(transactionHash, {
        type: "ERC20",
        status,
        token,
        fromChain,
        toChain,
        timestamp: block.timestamp,
        bridgingTx: log.transactionHash,
        claimingTx: claimTx,
        message: {
          attestation: message.attestation,
          message: message.message,
          amountSent: BigInt(log.args.amount),
        },
      });
    }),
  );

  return Array.from(transactionsMap.values());
}

function formatStatus(status: OnChainMessageStatus): TransactionStatus {
  switch (status) {
    case OnChainMessageStatus.UNKNOWN:
      return TransactionStatus.PENDING;
    case OnChainMessageStatus.CLAIMABLE:
      return TransactionStatus.READY_TO_CLAIM;
    case OnChainMessageStatus.CLAIMED:
      return TransactionStatus.COMPLETED;
    default:
      return TransactionStatus.PENDING;
  }
}
