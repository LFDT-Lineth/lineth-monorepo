import "@nomicfoundation/hardhat-foundry";
import "@nomicfoundation/hardhat-toolbox";
import "@openzeppelin/hardhat-upgrades";
import * as dotenv from "dotenv";
import "hardhat-deploy";
import "hardhat-storage-layout";
// import "hardhat-tracer"; // This plugin does not work with the latest hardhat version
import { HardhatUserConfig, subtask } from "hardhat/config";
import { TASK_DEPLOY_RUN_DEPLOY } from "hardhat-deploy";
import { createRequire } from "node:module";
import "solidity-docgen";

import { getBlockchainNode, getL2BlockchainNode } from "./common";
import { SupportedChainIds } from "./common/supportedNetworks";
import { overrides } from "./hardhat_overrides";
import { resolveDeployerAccounts } from "./scripts/hardhat/deployer-accounts";
import "./scripts/operational/tasks/getCurrentFinalizedBlockNumberTask";
import "./scripts/operational/tasks/grantContractRolesTask";
import "./scripts/operational/tasks/renounceContractRolesTask";
import "./scripts/operational/tasks/setRateLimitTask";
import "./scripts/operational/tasks/setVerifierAddressTask";
import "./scripts/operational/tasks/setMessageServiceOnTokenBridgeTask";
import "./scripts/operational/yieldBoost/addLidoStVaultYieldProvider";
import "./scripts/operational/yieldBoost/prepareInitiateOssification";
import "./scripts/operational/yieldBoost/testing/addAndClaimMessage";
import "./scripts/operational/yieldBoost/testing/addAndClaimMessageForLST";
import "./scripts/operational/yieldBoost/testing/unstakePermissionless";

dotenv.config();

const requireFromConfig = createRequire(__filename);

/** Lazy `require` avoids HH9 (signer-ui-bridge pulls in `hardhat`) and avoids native `import()` of `.ts`, which uses Node ESM resolution (directory `common/` vs `common.ts`, CJS `hardhat`, type-only `ethers` exports). */
subtask(TASK_DEPLOY_RUN_DEPLOY).setAction(async (args, hre, runSuper) => {
  const { signerUiHardhatDeployRunSubtaskAction } = requireFromConfig(
    "./scripts/hardhat/signer-ui-bridge.ts",
  ) as typeof import("./scripts/hardhat/signer-ui-bridge");
  return signerUiHardhatDeployRunSubtaskAction(args, hre, runSuper);
});

const BLOCKCHAIN_TIMEOUT = parseInt(process.env.BLOCKCHAIN_TIMEOUT_MS ?? "300000");

/**
 * `HARDHAT_SIGNER_UI=true` → no local keys (browser wallet via signer-ui bridge).
 * If `DEPLOYER_PRIVATE_KEY` is unset → `[]` so `hardhat compile`, `clean`, etc. work without secrets
 * (same as typical Hardhat: RPC-only until you sign). Deploy/sign on a live network then needs a key
 * or `HARDHAT_SIGNER_UI=true`. If a key *is* set, it must be valid non-zero hex (all-zero breaks
 * LocalAccountsProvider / @ethereumjs/util).
 */
function deployerAccounts(): string[] {
  return resolveDeployerAccounts();
}

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
    overrides: overrides,
  },
  namedAccounts: {
    deployer: {
      default: 0,
    },
  },
  networks: {
    hardhat: {
      hardfork: "osaka",
      // NB: Remove when ready for Deploying to Mainnet
      allowUnlimitedContractSize: true,
    },
    mainnet: {
      accounts: deployerAccounts(),
      url: "https://mainnet.infura.io/v3/" + process.env.INFURA_API_KEY,
    },
    sepolia: {
      accounts: deployerAccounts(),
      url: "https://sepolia.infura.io/v3/" + process.env.INFURA_API_KEY,
    },
    hoodi: {
      accounts: deployerAccounts(),
      url: "https://hoodi.infura.io/v3/" + process.env.INFURA_API_KEY,
      chainId: SupportedChainIds.HOODI,
    },
    linea_mainnet: {
      accounts: deployerAccounts(),
      url: "https://linea-mainnet.infura.io/v3/" + process.env.INFURA_API_KEY,
      chainId: 59144,
    },
    linea_sepolia: {
      accounts: deployerAccounts(),
      url: "https://linea-sepolia.infura.io/v3/" + process.env.INFURA_API_KEY,
      chainId: SupportedChainIds.LINEA_SEPOLIA,
    },
    custom: {
      accounts: deployerAccounts(),
      url: process.env.CUSTOM_RPC_URL ? process.env.CUSTOM_RPC_URL : "",
    },
    zkevm_dev: {
      gasPrice: 1322222229,
      url: blockchainNode,
      accounts: deployerAccounts(),
      timeout: BLOCKCHAIN_TIMEOUT,
      // docker L1 is 31648428 (docker/config/l1-node/el/genesis.json);
      // hosted devnet (e.g. rpc.devnet.linea.build) uses 59139.
      // Set ZKEVM_DEV_CHAIN_ID to enforce chain-ID validation and prevent wrong-chain deployments.
      // Omitting it disables validation (Hardhat HH101) and preserves flexibility across environments.
      ...(process.env.ZKEVM_DEV_CHAIN_ID ? { chainId: parseInt(process.env.ZKEVM_DEV_CHAIN_ID, 10) } : {}),
    },
    l2: {
      url: l2BlockchainNode ?? "",
      accounts: deployerAccounts(),
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
    // Must provide single API key to use Etherscan V2 - https://github.com/NomicFoundation/hardhat/pull/6727
    // Multiple API keys -> Will use Etherscan V1
    apiKey: process.env.ETHERSCAN_API_KEY ?? "",
    customChains: [
      {
        network: "linea_sepolia",
        chainId: SupportedChainIds.LINEA_SEPOLIA,
        urls: {
          apiURL: `https://api.etherscan.io/v2/api?chainid=${SupportedChainIds.LINEA_SEPOLIA}`,
          browserURL: "https://sepolia.lineascan.build/",
        },
      },
      {
        network: "linea_mainnet",
        chainId: SupportedChainIds.LINEA,
        urls: {
          apiURL: `https://api.etherscan.io/v2/api?chainid=${SupportedChainIds.LINEA}`,
          browserURL: "https://lineascan.build/",
        },
      },
      {
        network: "hoodi",
        chainId: SupportedChainIds.HOODI,
        urls: {
          apiURL: `https://api.etherscan.io/v2/api?chainid=${SupportedChainIds.HOODI}`,
          browserURL: "https://hoodi.etherscan.io/",
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
