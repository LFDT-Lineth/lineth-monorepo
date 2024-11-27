import { ethers } from "ethers";
import { EnvironmentBasedAccountManager } from "../accounts/environment-based-account-manager";
import { Config } from "../types";
import Account from "../accounts/account";

const L1_RPC_URL = new URL(`https://sepolia.infura.io/v3/${process.env.INFURA_PROJECT_ID}`);
const L2_RPC_URL = new URL("https://rpc.devnet.linea.build");
const L1_CHAIN_ID = 11155111;
const L2_CHAIN_ID = 59139;

const L1_WHALE_ACCOUNTS_PRIVATE_KEYS: string[] = process.env.L1_WHALE_ACCOUNTS_PRIVATE_KEYS?.split(",") ?? [];
const L2_WHALE_ACCOUNTS_PRIVATE_KEYS: string[] = process.env.L2_WHALE_ACCOUNTS_PRIVATE_KEYS?.split(",") ?? [];
const L1_WHALE_ACCOUNTS_ADDRESSES: string[] = process.env.L1_WHALE_ACCOUNTS_ADDRESSES?.split(",") ?? [];
const L2_WHALE_ACCOUNTS_ADDRESSES: string[] = process.env.L2_WHALE_ACCOUNTS_ADDRESSES?.split(",") ?? [];

const L1_WHALE_ACCOUNTS: Account[] = L1_WHALE_ACCOUNTS_PRIVATE_KEYS.map((privateKey, index) => {
  return new Account(privateKey, L1_WHALE_ACCOUNTS_ADDRESSES[index]);
});

const L2_WHALE_ACCOUNTS: Account[] = L2_WHALE_ACCOUNTS_PRIVATE_KEYS.map((privateKey, index) => {
  return new Account(privateKey, L2_WHALE_ACCOUNTS_ADDRESSES[index]);
});

const config: Config = {
  L1: {
    rpcUrl: L1_RPC_URL,
    chainId: L1_CHAIN_ID,
    lineaRollupAddress: "0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48",
    lineaRollupProxyAdminAddress: "0x10b7ef80D4bA8df6b4Daed7B7638cd88C6d52F02",
    tokenBridgeAddress: "",
    l1TokenAddress: "",
    accountManager: new EnvironmentBasedAccountManager(
      new ethers.JsonRpcProvider(L1_RPC_URL.toString()),
      L1_WHALE_ACCOUNTS,
      L1_CHAIN_ID,
    ),
    dummyContractAddress: "",
  },
  L2: {
    rpcUrl: L2_RPC_URL,
    chainId: L2_CHAIN_ID,
    l2MessageServiceAddress: "0x33bf916373159A8c1b54b025202517BfDbB7863D",
    tokenBridgeAddress: "",
    l2TokenAddress: "",
    l2TestContractAddress: "",
    accountManager: new EnvironmentBasedAccountManager(
      new ethers.JsonRpcProvider(L2_RPC_URL.toString()),
      L2_WHALE_ACCOUNTS,
      L2_CHAIN_ID,
    ),
    dummyContractAddress: "",
  },
};

export default config;
