import { formatEther, formatUnits, parseEther, parseUnits } from "viem";
import "dotenv/config";

export const METAMASK_SEED_PHRASE = process.env.E2E_TEST_SEED_PHRASE;
export const METAMASK_PASSWORD = process.env.E2E_TEST_WALLET_PASSWORD;

export const LOCAL_L1_NETWORK = {
  name: "Local L1 Network",
  rpcUrl: `http://localhost:8445`,
  chainId: 31648428,
  symbol: "ETH",
  blockExplorerUrl: "https://etherscan.io",
};

export const LOCAL_L2_NETWORK = {
  name: "Local L2 Network",
  rpcUrl: `http://localhost:9045`,
  chainId: 1337,
  symbol: "ETH",
  blockExplorerUrl: "https://lineascan.build",
};

export const TEST_URL = "http://localhost:3000/";
export const WEI_AMOUNT = formatEther(parseEther("1"));
export const ERC20_AMOUNT = formatUnits(parseUnits("10", 18), 18).toString();
// Must be > minimum CCTP fee
export const USDC_AMOUNT = formatUnits(10n, 6).toString();
export const ETH_SYMBOL = "ETH";
export const ERC20_SYMBOL = "TERC20";
export const USDC_SYMBOL = "USDC";

export const POLLING_INTERVAL = 250;
export const PAGE_TIMEOUT = 10000;

export const L1_TEST_ERC2O_CONTRACT_ADDRESS = "0x8A791620dd6260079BF849Dc5567aDC3F2FdC318";
export const L2_TEST_ERC2O_CONTRACT_ADDRESS = "0xCC1B08B17301e090cbb4c1F5598Cbaa096d591FB";

// FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
export const L1_ACCOUNT_PRIVATE_KEY = "0x92db14e403b83dfe3df233f83dfa3a0d7096f21ca9b0d6d6b8d88b2b4ec1564e";
// FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
export const L2_ACCOUNT_PRIVATE_KEY = "0x004f78823f38639b9ef15392eb8024ace1d7b991ea820b0dd36a15d14d1a6785";

export const L1_ACCOUNT_METAMASK_NAME = "Account 2";
export const L2_ACCOUNT_METAMASK_NAME = "Account 3";
