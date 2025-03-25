import "@nomicfoundation/hardhat-toolbox";
import "@nomicfoundation/hardhat-foundry";
import "@openzeppelin/hardhat-upgrades";
import "@nomicfoundation/hardhat-foundry";
import * as dotenv from "dotenv";
import "hardhat-deploy";
import "hardhat-storage-layout";
// import "hardhat-tracer"; // This plugin does not work with the latest hardhat version
import { HardhatUserConfig } from "hardhat/config";
import { getBlockchainNode, getL2BlockchainNode } from "./common";
import "./scripts/operational/tasks/getCurrentFinalizedBlockNumberTask";
import "./scripts/operational/tasks/grantContractRolesTask";
import "./scripts/operational/tasks/renounceContractRolesTask";
import "./scripts/operational/tasks/setRateLimitTask";
import "./scripts/operational/tasks/setVerifierAddressTask";
import "./scripts/operational/tasks/setMessageServiceOnTokenBridgeTask";

import "solidity-docgen";

dotenv.config();

const BLOCKCHAIN_TIMEOUT = parseInt(process.env.BLOCKCHAIN_TIMEOUT_MS ?? "300000");
const EMPTY_HASH = "0x0000000000000000000000000000000000000000000000000000000000000000";

const blockchainNode = getBlockchainNode();
const l2BlockchainNode = getL2BlockchainNode();

const useViaIR = process.env.ENABLE_VIA_IR === "true";

const config: HardhatUserConfig = {
  paths: {
    artifacts: "./build",
    sources: "./src",
  },
  solidity: {
    // NB: double check the autoupdate shell script version complies to the latest solidity version if you add a new one.
    compilers: [
      {
        version: "0.8.28",
        settings: {
          viaIR: useViaIR,
          optimizer: {
            enabled: true,
            runs: 10_000,
          },
          evmVersion: "cancun",
        },
      },
      /**
       * Maintain for Mimc contract
       * src/libraries/Mimc.sol (0.8.25)
       * src/libraries/SparseMerkleProof.sol (0.8.25)
       */
      {
        version: "0.8.25",
        settings: {
          viaIR: useViaIR,
          optimizer: {
            enabled: true,
            runs: 10_000,
          },
          evmVersion: "cancun",
        },
      },
      {
        version: "0.8.19",
        settings: {
          viaIR: useViaIR,
          optimizer: {
            enabled: true,
            runs: 10_000,
          },
          evmVersion: "london",
        },
      },
    ],
  },
  namedAccounts: {
    deployer: {
      default: 0,
    },
  },
  networks: {
    hardhat: {
      hardfork: "cancun",
    },
    mainnet: {
      accounts: [process.env.MAINNET_PRIVATE_KEY || EMPTY_HASH],
      url: "https://mainnet.infura.io/v3/" + process.env.INFURA_API_KEY,
    },
    sepolia: {
      accounts: [process.env.SEPOLIA_PRIVATE_KEY || EMPTY_HASH],
      url: "https://sepolia.infura.io/v3/" + process.env.INFURA_API_KEY,
    },
    linea_mainnet: {
      accounts: [process.env.LINEA_MAINNET_PRIVATE_KEY || EMPTY_HASH],
      url: "https://linea-mainnet.infura.io/v3/" + process.env.INFURA_API_KEY,
    },
    linea_sepolia: {
      accounts: [process.env.LINEA_SEPOLIA_PRIVATE_KEY || EMPTY_HASH],
      url: "https://linea-sepolia.infura.io/v3/" + process.env.INFURA_API_KEY,
    },
    custom: {
      accounts: [process.env.CUSTOM_PRIVATE_KEY || EMPTY_HASH],
      url: process.env.CUSTOM_BLOCKCHAIN_URL ? process.env.CUSTOM_BLOCKCHAIN_URL : "",
    },
    zkevm_dev: {
      gasPrice: 1322222229,
      url: blockchainNode,
      accounts: [process.env.PRIVATE_KEY || EMPTY_HASH],
      timeout: BLOCKCHAIN_TIMEOUT,
    },
    l2: {
      url: l2BlockchainNode ?? "",
      accounts: [process.env.L2_PRIVATE_KEY || EMPTY_HASH],
      allowUnlimitedContractSize: true,
    },
  },
  gasReporter: {
    enabled: !!process.env.REPORT_GAS,
  },
  mocha: {
    timeout: 20000,
  },
  etherscan: {
    apiKey: {
      mainnet: process.env.ETHERSCAN_API_KEY ?? "",
      sepolia: process.env.ETHERSCAN_API_KEY ?? "",
      linea_sepolia: process.env.LINEASCAN_API_KEY ?? "",
      linea_mainnet: process.env.LINEASCAN_API_KEY ?? "",
    },
    customChains: [
      {
        network: "linea_sepolia",
        chainId: 59141,
        urls: {
          apiURL: "https://api-sepolia.lineascan.build/api",
          browserURL: "https://sepolia.lineascan.build/",
        },
      },
      {
        network: "linea_mainnet",
        chainId: 59144,
        urls: {
          apiURL: "https://api.lineascan.build/api",
          browserURL: "https://lineascan.build/",
        },
      },
    ],
  },
  docgen: {
    exclude: [
      "_testing",
      "bridging/token/utils/StorageFiller39.sol",
      "bridging/token/CustomBridgedToken.sol",
      "governance/TimeLock.sol",
      "security/access/PermissionsManager.sol",
      "security/reentrancy/TransientStorageReentrancyGuardUpgradeable.sol",
      "tokens",
      "verifiers",
    ],
    pages: "files",
    outputDir: "docs/api/",
    // For compatibility with docs.linea.build
    pageExtension: ".mdx",
    templates: "docs/docgen-templates",
  },
};

export default config;
