import { ethers } from "ethers";
import path from "path";
import { GenesisBasedAccountManager } from "../accounts/genesis-based-account-manager";
import { Config } from "../types";

const L1_RPC_URL = new URL("http://localhost:8445");
const L2_RPC_URL = new URL("http://localhost:8845");
const L2_BESU_NODE_RPC_URL = new URL("http://localhost:9045");
const SHOMEI_ENDPOINT = new URL("http://localhost:8998");
const SHOMEI_FRONTEND_ENDPOINT = new URL("http://localhost:8889");
const SEQUENCER_ENDPOINT = new URL("http://localhost:8545");
const TRANSACTION_EXCLUSION_ENDPOINT = new URL("http://localhost:8082");

const config: Config = {
  L1: {
    rpcUrl: L1_RPC_URL,
    chainId: 31648428,
    lineaRollupAddress: "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9",
    lineaRollupProxyAdminAddress: "0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0",
    dummyContractAddress: "0x610178dA211FEF7D417bC0e6FeD39F05609AD788",
    tokenBridgeAddress: "0x2279B7A0a67DB372996a5FaB50D91eAA73d2eBe6",
    l1TokenAddress: "0x8A791620dd6260079BF849Dc5567aDC3F2FdC318",
    accountManager: new GenesisBasedAccountManager(
      new ethers.JsonRpcProvider(L1_RPC_URL.toString()),
      path.resolve(
        process.env.LOCAL_L1_GENESIS ||
          path.resolve(__dirname, "../../../../..", "docker/config/l1-node/el", "genesis.json"),
      ),
    ),
  },
  L2: {
    rpcUrl: L2_RPC_URL,
    besuNodeRpcUrl: L2_BESU_NODE_RPC_URL,
    chainId: 1337,
    l2MessageServiceAddress: "0xe537D669CA013d86EBeF1D64e40fC74CADC91987",
    // Nonce 10
    l2TestContractAddress: "0x997FC3aF1F193Cbdc013060076c67A13e218980e",
    // Nonce 9
    dummyContractAddress: "0xE4392c8ecC46b304C83cDB5edaf742899b1bda93",
    tokenBridgeAddress: "0x5C95Bcd50E6D1B4E3CDC478484C9030Ff0a7D493",
    l2TokenAddress: "0xCC1B08B17301e090cbb4c1F5598Cbaa096d591FB",
    accountManager: new GenesisBasedAccountManager(
      new ethers.JsonRpcProvider(L2_RPC_URL.toString()),
      path.resolve(
        process.env.LOCAL_L2_GENESIS ||
          path.resolve(__dirname, "../../../../..", "docker/config", "linea-local-dev-genesis-PoA.json"),
      ),
    ),
    shomeiEndpoint: SHOMEI_ENDPOINT,
    shomeiFrontendEndpoint: SHOMEI_FRONTEND_ENDPOINT,
    sequencerEndpoint: SEQUENCER_ENDPOINT,
    transactionExclusionEndpoint: TRANSACTION_EXCLUSION_ENDPOINT,
    // Nonce 11
    opcodeTestContractAddress: "0xFCc2155b495B6Bf6701eb322D3a97b7817898306",
  },
};

export default config;
