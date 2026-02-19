const MAX_GAS_LIMIT = process.env.TX_GAS_LIMIT ? parseInt(process.env.TX_GAS_LIMIT) : 500000000;

function getBlockchainNode(): string {
  return process.env.L1_RPC_URL || "http://127.0.0.1:8545";
}

function getL2BlockchainNode(): string | undefined {
  return process.env.L2_RPC_URL;
}

export { MAX_GAS_LIMIT, getBlockchainNode, getL2BlockchainNode };
