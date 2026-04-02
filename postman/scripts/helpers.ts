import {
  Address,
  Hex,
  createPublicClient,
  createWalletClient,
  defineChain,
  encodeAbiParameters,
  http,
  keccak256,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";

export function encodeSendMessage(
  sender: Address,
  receiver: Address,
  fee: bigint,
  amount: bigint,
  messageNonce: bigint,
  calldata: Hex,
) {
  const data = encodeAbiParameters(
    [
      { name: "from", type: "address" },
      { name: "to", type: "address" },
      { name: "fee", type: "uint256" },
      { name: "value", type: "uint256" },
      { name: "nonce", type: "uint256" },
      { name: "calldata", type: "bytes" },
    ],
    [sender, receiver, fee, amount, messageNonce, calldata],
  );

  return keccak256(data);
}

export async function createChainContext(rpcUrl: string, signerPrivateKey: Hex) {
  const tempClient = createPublicClient({ transport: http(rpcUrl) });
  const chainId = await tempClient.getChainId();

  const chain = defineChain({
    id: chainId,
    name: "custom",
    nativeCurrency: { name: "Ether", symbol: "ETH", decimals: 18 },
    rpcUrls: { default: { http: [rpcUrl] } },
  });

  const publicClient = createPublicClient({ chain, transport: http(rpcUrl) });
  const account = privateKeyToAccount(signerPrivateKey);
  const walletClient = createWalletClient({ account, chain, transport: http(rpcUrl) });

  return { chainId, chain, publicClient, walletClient, account };
}
