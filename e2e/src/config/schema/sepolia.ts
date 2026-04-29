import { createWalletClient, http, isAddress, isHex } from "viem";
import { lineaSepolia, sepolia } from "viem/chains";

import { Config } from "./config-schema";
import Account from "../accounts/account";
import { EnvironmentBasedAccountManager } from "../accounts/environment-based-account-manager";

const L1_RPC_URL = new URL(`https://sepolia.infura.io/v3/${process.env.INFURA_PROJECT_ID}`);
const L2_RPC_URL = new URL(`https://linea-sepolia.infura.io/v3/${process.env.INFURA_PROJECT_ID}`);
const L1_CHAIN_ID = 11155111;
const L2_CHAIN_ID = 59141;
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
    lineaRollupAddress: "0xB218f8A4Bc926cF1cA7b3423c154a0D627Bdb7E5",
    lineaRollupProxyAdminAddress: "0xa89E358Ef34921ebA90f328901B7381F86b1db52",
    tokenBridgeAddress: "0x5A0a48389BB0f12E5e017116c1105da97E129142",
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
    l2MessageServiceAddress: "0x971e727e956690b9957be6d51Ec16E73AcAC83A7",
    tokenBridgeAddress: "0x93DcAdf238932e6e6a85852caC89cBd71798F463",
    l2TokenAddress: "0x",
    l2TestContractAddress: "0x",
    l2SparseMerkleProofAddress: "0x",
    l2LineaSequencerUptimeFeedAddress: "0xFD56cb560cf858B86897dd6415Ba8EEa70110355",
    accountManager: new EnvironmentBasedAccountManager(
      createWalletClient({
        chain: lineaSepolia,
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
