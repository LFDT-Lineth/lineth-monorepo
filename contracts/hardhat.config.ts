// Load .env before any other module reads process.env (deploy scripts and helpers rely on it).
import "dotenv/config";

import hardhatToolboxViem from "@nomicfoundation/hardhat-toolbox-viem";
import hardhatUpgrades from "@openzeppelin/hardhat-upgrades";
import { configVariable, defineConfig } from "hardhat/config";
import hardhatIgnoreWarnings from "hardhat-ignore-warnings";

import { SupportedChainIds } from "./common/supportedNetworks.js";
import { getBlockchainNode } from "./common.js";
import linethDeployPlugin from "./hardhat-plugins/deploy/index.js";
import { normalizeSignerUiEnv, resolveDeployerAccounts } from "./hardhat-plugins/signer-ui/signer-mode.js";
import { overrides } from "./hardhat_overrides.js";
import { deployTasks, operationalTasks } from "./scripts/hardhat/tasks.js";

const BLOCKCHAIN_TIMEOUT = parseInt(process.env.BLOCKCHAIN_TIMEOUT_MS ?? "300000");

const useViaIR = process.env.ENABLE_VIA_IR === "true";

/**
 * `HARDHAT_SIGNER_UI=true` → no local keys (browser wallet via signer-ui bridge).
 * If `DEPLOYER_PRIVATE_KEY` is unset → `[]` so `hardhat build`, `clean`, etc. work without secrets.
 * If a key *is* set, it must be valid non-zero hex.
 */
normalizeSignerUiEnv();
const deployerAccounts = resolveDeployerAccounts();

// Infura endpoints keep the existing INFURA_API_KEY contract; resolved lazily, only when the network is used.
const infuraUrl = (subdomain: string) =>
  configVariable("INFURA_API_KEY", { format: `https://${subdomain}.infura.io/v3/{variable}` });

export default defineConfig({
  plugins: [hardhatToolboxViem, hardhatUpgrades, hardhatIgnoreWarnings, linethDeployPlugin],
  tasks: [...operationalTasks, ...deployTasks],
  paths: {
    sources: "./src",
    artifacts: "./build",
    tests: {
      nodejs: "./test/hardhat",
      solidity: "./test/foundry",
    },
  },
  solidity: {
    // NB: double check the autoupdate shell script version complies to the latest solidity version if you add a new one.
    /// @dev Please see the overrides file for a list of files not targetting the default EVM version of Prague.
    compilers: [
      {
        version: "0.8.33",
        settings: {
          viaIR: useViaIR,
          optimizer: {
            enabled: true,
            runs: 10_000,
          },
          evmVersion: "osaka",
        },
      },
    ],
    overrides,
  },
  // Replaces the Hardhat 2 compile-subtask filter. `func-mutability` is solc warning 2018: the
  // SafeExecutionConditions guards are intentionally non-`view` so multisig UIs (e.g. Safe{Wallet}) stage them
  // as transactions rather than read-only calls.
  warnings: {
    "src/operational/SafeExecutionConditions.sol": {
      "func-mutability": "off",
    },
  },
  networks: {
    default: {
      type: "edr-simulated",
      chainType: "l1",
      hardfork: "osaka",
      // NB: Remove when ready for Deploying to Mainnet
      allowUnlimitedContractSize: true,
    },
    mainnet: {
      type: "http",
      chainType: "l1",
      accounts: deployerAccounts,
      url: infuraUrl("mainnet"),
    },
    sepolia: {
      type: "http",
      chainType: "l1",
      accounts: deployerAccounts,
      url: infuraUrl("sepolia"),
    },
    hoodi: {
      type: "http",
      chainType: "l1",
      accounts: deployerAccounts,
      url: infuraUrl("hoodi"),
      chainId: SupportedChainIds.HOODI,
    },
    linea_mainnet: {
      type: "http",
      accounts: deployerAccounts,
      url: infuraUrl("linea-mainnet"),
      chainId: SupportedChainIds.LINEA,
    },
    linea_sepolia: {
      type: "http",
      accounts: deployerAccounts,
      url: infuraUrl("linea-sepolia"),
      chainId: SupportedChainIds.LINEA_SEPOLIA,
    },
    custom: {
      type: "http",
      accounts: deployerAccounts,
      url: configVariable("CUSTOM_RPC_URL"),
    },
    zkevm_dev: {
      type: "http",
      gasPrice: 1322222229,
      url: getBlockchainNode(),
      accounts: deployerAccounts,
      timeout: BLOCKCHAIN_TIMEOUT,
      // docker L1 is 31648428 (docker/config/l1-node/el/genesis.json);
      // hosted devnet (e.g. rpc.devnet.linea.build) uses 59139.
      // Set ZKEVM_DEV_CHAIN_ID to enforce chain-ID validation and prevent wrong-chain deployments.
      // Omitting it disables validation and preserves flexibility across environments.
      ...(process.env.ZKEVM_DEV_CHAIN_ID ? { chainId: parseInt(process.env.ZKEVM_DEV_CHAIN_ID, 10) } : {}),
    },
    l2: {
      type: "http",
      url: configVariable("L2_RPC_URL"),
      accounts: deployerAccounts,
    },
  },
  // Hoodi, Linea Sepolia and Linea mainnet use Hardhat's built-in chain descriptors and the Etherscan v2 API,
  // which replace the Hardhat 2 `etherscan.customChains` entries. Only Etherscan is enabled, as before.
  verify: {
    etherscan: {
      apiKey: configVariable("ETHERSCAN_API_KEY"),
    },
    blockscout: {
      enabled: false,
    },
    sourcify: {
      enabled: false,
    },
  },
  test: {
    solidity: {
      // Reduced during the Hardhat 3 migration; restore production values once the migration is stable.
      fuzz: {
        runs: 4,
      },
      invariant: {
        runs: 4,
        depth: 4,
        failOnRevert: true,
      },
    },
  },
});
