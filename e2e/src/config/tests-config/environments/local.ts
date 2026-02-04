import { ethers } from "ethers";
import path from "path";
import { GenesisBasedAccountManager } from "../accounts/genesis-based-account-manager";
import { Config } from "../types";

const L1_RPC_URL = new URL("http://localhost:8445");
const L2_RPC_URL = new URL("http://localhost:9045");
const L2_BESU_NODE_RPC_URL = new URL("http://localhost:9045");
const L2_BESU_FOLLOWER_NODE_RPC_URL = new URL("http://localhost:9245");
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
    accountManager: new GenesisBasedAccountManager({
      provider: new ethers.JsonRpcProvider(L1_RPC_URL.toString()),
      genesisFilePath: path.resolve(
        process.env.LOCAL_L1_GENESIS ||
          path.resolve(__dirname, "../../../../..", "docker/config/l1-node/el", "genesis.json"),
      ),
      excludeAddresses: [
        "0x70997970C51812dc3A010C7d01b50e0d17dc79C8", // Finalization operator
        "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC", // Data submission operator
        "0x90F79bf6EB2c4f870365E785982E1f101E93b906", // L1 Security council
        "0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc", // L1 Postman account
      ],
      priorityAddresses: [
        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", // L1 deployer account
      ],
    }),
  },
  L2: {
    rpcUrl: L2_RPC_URL,
    besuNodeRpcUrl: L2_BESU_NODE_RPC_URL,
    besuFollowerNodeRpcUrl: L2_BESU_FOLLOWER_NODE_RPC_URL,
    chainId: 1337,
    l2MessageServiceAddress: "0xe537D669CA013d86EBeF1D64e40fC74CADC91987",
    l2TestContractAddress: "0x997FC3aF1F193Cbdc013060076c67A13e218980e", // Nonce 10
    dummyContractAddress: "0xE4392c8ecC46b304C83cDB5edaf742899b1bda93", // Nonce 9
    tokenBridgeAddress: "0x5C95Bcd50E6D1B4E3CDC478484C9030Ff0a7D493",
    l2TokenAddress: "0xCC1B08B17301e090cbb4c1F5598Cbaa096d591FB",
    l2SparseMerkleProofAddress: "0x670365526A9971E4A225c38538c5D7Ac248e4087", // Nonce 13
    l2LineaSequencerUptimeFeedAddress: "0x7917AbB0cDbf3D3C4057d6a2808eE85ec16260C1", // Nonce 12
    accountManager: new GenesisBasedAccountManager({
      provider: new ethers.JsonRpcProvider(L2_RPC_URL.toString()),
      genesisFilePath: path.resolve(
        process.env.LOCAL_L2_GENESIS ||
          path.resolve(__dirname, "../../../../..", "docker/config/l2-genesis-initialization", "genesis-besu.json"),
      ),
      excludeAddresses: [
        "0xfe3b557e8fb62b89f4916b721be55ceb828dbd73", // Used for Opcode testing
        "0xf17f52151EbEF6C7334FAD080c5704D77216b732", // L2 Security council
        "0xd42e308fc964b71e18126df469c21b0d7bcb86cc", // Message anchorer
        "0xc8c92fe825d8930b9357c006e0af160dfa727a62", // L2 postman account
      ],
      priorityAddresses: [
        "0x1b9abeec3215d8ade8a33607f2cf0f4f60e5f0d0", // L2 deployer account
        "0x54d450f4d728da50f1271a1700b42657940324aa", // Liveness signer account
      ],
    }),
    shomeiEndpoint: SHOMEI_ENDPOINT,
    shomeiFrontendEndpoint: SHOMEI_FRONTEND_ENDPOINT,
    sequencerEndpoint: SEQUENCER_ENDPOINT,
    transactionExclusionEndpoint: TRANSACTION_EXCLUSION_ENDPOINT,
    opcodeTesterAddress: "0xa50a51c09a5c451C52BB714527E1974b686D8e77",
  },
};

export default config;
