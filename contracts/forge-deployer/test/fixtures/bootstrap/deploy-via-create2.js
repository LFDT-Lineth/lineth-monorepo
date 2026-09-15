// Custom bootstrap script: deploys a simple contract through the deterministic
// CREATE2 proxy via a normal signed call from the deployer, at the runner-provided
// starting nonce. The EIP-7997 factory takes raw calldata salt(32) ++ initCode
// (no function selector). The proxy is Anvil-preinstalled on L2 in this scenario.
const { ethers } = require("ethers");

const FACTORY = "0x4e59b44847b379578588920cA78FbF26c0B4956C";
const SALT = "0x00000000000000000000000000000000000000000000000000000000000000cd";
const INITCODE = "0x6001600c60003960016000f300";

async function main() {
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY, provider);
  const nonce = Number(process.env.BOOTSTRAP_START_NONCE);
  const data = ethers.concat([SALT, INITCODE]);
  const tx = await wallet.sendTransaction({ to: FACTORY, data, nonce, gasPrice: 0n });
  const receipt = await tx.wait();
  const initCodeHash = ethers.keccak256(INITCODE);
  const addr = ethers.getCreate2Address(FACTORY, SALT, initCodeHash);
  const code = await provider.getCode(addr);
  console.log(`bootstrap create2 deployed at ${addr} status=${receipt.status} codeBytes=${(code.length - 2) / 2}`);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
