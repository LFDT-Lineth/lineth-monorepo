import { createWalletClient, http, isAddress, isHex } from "viem";
import { sepolia } from "viem/chains";

import { Config } from "./config-schema";
import Account from "../accounts/account";
import { EnvironmentBasedAccountManager } from "../accounts/environment-based-account-manager";
import { lineaDevnet } from "../chains/constants";

const L1_RPC_URL = new URL(`https://sepolia.infura.io/v3/${process.env.INFURA_PROJECT_ID}`);
const L2_RPC_URL = new URL("https://rpc.devnet.linea.build");
const L1_CHAIN_ID = 11155111;
const L2_CHAIN_ID = 59139;

const L1_WHALE_ACCOUNTS_PRIVATE_KEYS: string[] = process.env.L1_WHALE_ACCOUNTS_PRIVATE_KEYS?.split(",") ?? [];
const L2_WHALE_ACCOUNTS_PRIVATE_KEYS: string[] = process.env.L2_WHALE_ACCOUNTS_PRIVATE_KEYS?.split(",") ?? [];
const L1_WHALE_ACCOUNTS_ADDRESSES: string[] = process.env.L1_WHALE_ACCOUNTS_ADDRESSES?.split(",") ?? [];
const L2_WHALE_ACCOUNTS_ADDRESSES: string[] = process.env.L2_WHALE_ACCOUNTS_ADDRESSES?.split(",") ?? [];

const L1_WHALE_ACCOUNTS: Account[] = L1_WHALE_ACCOUNTS_PRIVATE_KEYS.map((privateKey, index) => {
  if (!isHex(privateKey) || !isAddress(L1_WHALE_ACCOUNTS_ADDRESSES[index])) {
    throw new Error(`Invalid hex string for L1 whale account private key at index ${index}`);
  }
  return new Account(privateKey, L1_WHALE_ACCOUNTS_ADDRESSES[index] as `0x${string}`);
});

const L2_WHALE_ACCOUNTS: Account[] = L2_WHALE_ACCOUNTS_PRIVATE_KEYS.map((privateKey, index) => {
  if (!isHex(privateKey) || !isAddress(L2_WHALE_ACCOUNTS_ADDRESSES[index])) {
    throw new Error(`Invalid hex string for L2 whale account private key at index ${index}`);
  }
  return new Account(privateKey, L2_WHALE_ACCOUNTS_ADDRESSES[index] as `0x${string}`);
});

const config: Config = {
  L1: {
    rpcUrl: L1_RPC_URL,
    chainId: L1_CHAIN_ID,
    lineaRollupAddress: "0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48",
    lineaRollupProxyAdminAddress: "0x10b7ef80D4bA8df6b4Daed7B7638cd88C6d52F02",
    tokenBridgeAddress: "0x",
    l1TokenAddress: "0x",
    accountManager: new EnvironmentBasedAccountManager(
      createWalletClient({
        chain: sepolia,
        transport: http(L1_RPC_URL.toString()),
      }),
      L1_WHALE_ACCOUNTS,
      L1_CHAIN_ID,
    ),
    dummyContractAddress: "0x",
    forcedTransactionGatewayAddress: "0x",
  },
  L2: {
    rpcUrl: L2_RPC_URL,
    chainId: L2_CHAIN_ID,
    l2MessageServiceAddress: "0x33bf916373159A8c1b54b025202517BfDbB7863D",
    tokenBridgeAddress: "0x",
    l2TokenAddress: "0x",
    l2TestContractAddress: "0x",
    l2SparseMerkleProofAddress: "0x",
    l2LineaSequencerUptimeFeedAddress: "0x",
    accountManager: new EnvironmentBasedAccountManager(
      createWalletClient({
        chain: lineaDevnet,
        transport: http(L2_RPC_URL.toString()),
      }),
      L2_WHALE_ACCOUNTS,
      L2_CHAIN_ID,
    ),
    dummyContractAddress: "0x",
    opcodeTesterAddress: "0x",
  },
};

export default config;
