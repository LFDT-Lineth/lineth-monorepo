import { localL1Network, localL2Network } from "@/constants";
import { test as setup } from "@playwright/test";
import {
  createPublicClient,
  createWalletClient,
  encodeFunctionData,
  erc20Abi,
  formatEther,
  http,
  parseEther,
} from "viem";
import { mnemonicToAccount, privateKeyToAccount } from "viem/accounts";
import { waitForTransactionReceipt } from "viem/actions";
import { sendTransactionsToGenerateTrafficWithInterval } from "./utils/utils";
import {
  L1_TEST_ERC2O_CONTRACT_ADDRESS,
  L2_ACCOUNT_PRIVATE_KEY,
  L2_TEST_ERC2O_CONTRACT_ADDRESS,
  LOCAL_L1_NETWORK,
  LOCAL_L2_NETWORK,
} from "./constants";
import { estimateGas } from "viem/linea";

setup.setTimeout(200_000);

setup("Global setup", async () => {
  console.log("Generating L1 traffic...");
  await generateL1Traffic();

  console.log("Generating L2 traffic...");
  await generateL2Traffic();

  console.log("Sending ERC20 token to account...");
  await sendERC20TokenToAccount();
});

async function generateL2Traffic() {
  // FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
  const pollingAccount = privateKeyToAccount("0xb17202c37cce9498e6f7dcdc1abd207802d09b5eee96677ea219ac867a198b91");

  const walletClient = createWalletClient({
    chain: localL2Network,
    transport: http(LOCAL_L2_NETWORK.rpcUrl),
    account: pollingAccount,
  });
  await sendTransactionsToGenerateTrafficWithInterval(walletClient, 1_000);
}

async function generateL1Traffic() {
  // FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
  const pollingAccount = mnemonicToAccount(process.env.E2E_TEST_SEED_PHRASE, { accountIndex: 0, addressIndex: 7 });

  const walletClient = createWalletClient({
    chain: localL1Network,
    transport: http(LOCAL_L1_NETWORK.rpcUrl),
    account: pollingAccount,
  });
  await sendTransactionsToGenerateTrafficWithInterval(walletClient, 1_000);
}

async function sendERC20TokenToAccount() {
  const l1Account = mnemonicToAccount(process.env.E2E_TEST_SEED_PHRASE, { accountIndex: 0, addressIndex: 6 });

  const walletClient = createWalletClient({
    chain: localL1Network,
    transport: http(LOCAL_L1_NETWORK.rpcUrl),
    account: l1Account,
  });

  const publicClient = createPublicClient({
    chain: localL1Network,
    transport: http(LOCAL_L1_NETWORK.rpcUrl),
  });

  const { maxPriorityFeePerGas, maxFeePerGas } = await publicClient.estimateFeesPerGas({ type: "eip1559" });

  const transactionHash = await walletClient.sendTransaction({
    to: L1_TEST_ERC2O_CONTRACT_ADDRESS,
    value: 0n,
    data: encodeFunctionData({
      abi: [
        {
          inputs: [
            {
              internalType: "address",
              name: "_to",
              type: "address",
            },
            {
              internalType: "uint256",
              name: "_amount",
              type: "uint256",
            },
          ],
          name: "mint",
          outputs: [],
          stateMutability: "nonpayable",
          type: "function",
        },
      ],
      functionName: "mint",
      args: [l1Account.address, parseEther("1000")],
    }),
    maxPriorityFeePerGas: maxPriorityFeePerGas,
    maxFeePerGas: maxFeePerGas,
  });

  await waitForTransactionReceipt(walletClient, { hash: transactionHash, confirmations: 1 });

  const l2Account = privateKeyToAccount(L2_ACCOUNT_PRIVATE_KEY);

  const l2WalletClient = createWalletClient({
    chain: localL2Network,
    transport: http(LOCAL_L2_NETWORK.rpcUrl),
    account: l2Account,
  });

  const l2PublicClient = createPublicClient({
    chain: localL2Network,
    transport: http(LOCAL_L2_NETWORK.rpcUrl),
  });

  const { priorityFeePerGas, baseFeePerGas, gasLimit } = await estimateGas(l2WalletClient, {
    account: l2WalletClient.account,
    to: L2_TEST_ERC2O_CONTRACT_ADDRESS,
    value: 0n,
    data: encodeFunctionData({
      abi: [
        {
          inputs: [
            {
              internalType: "address",
              name: "_to",
              type: "address",
            },
            {
              internalType: "uint256",
              name: "_amount",
              type: "uint256",
            },
          ],
          name: "mint",
          outputs: [],
          stateMutability: "nonpayable",
          type: "function",
        },
      ],
      functionName: "mint",
      args: [l2Account.address, parseEther("1000")],
    }),
  });

  const transactionHashL2 = await l2WalletClient.sendTransaction({
    to: L2_TEST_ERC2O_CONTRACT_ADDRESS,
    value: 0n,
    data: encodeFunctionData({
      abi: [
        {
          inputs: [
            {
              internalType: "address",
              name: "_to",
              type: "address",
            },
            {
              internalType: "uint256",
              name: "_amount",
              type: "uint256",
            },
          ],
          name: "mint",
          outputs: [],
          stateMutability: "nonpayable",
          type: "function",
        },
      ],
      functionName: "mint",
      args: [l2Account.address, parseEther("1000")],
    }),
    maxPriorityFeePerGas: priorityFeePerGas,
    maxFeePerGas: priorityFeePerGas + baseFeePerGas,
    gas: gasLimit,
  });

  await waitForTransactionReceipt(l2WalletClient, { hash: transactionHashL2, confirmations: 1 });

  const l1Balance = await publicClient.readContract({
    address: L1_TEST_ERC2O_CONTRACT_ADDRESS,
    abi: erc20Abi,
    functionName: "balanceOf",
    args: [l1Account.address],
  });

  const l2Balance = await l2PublicClient.readContract({
    address: L2_TEST_ERC2O_CONTRACT_ADDRESS,
    abi: erc20Abi,
    functionName: "balanceOf",
    args: [l2Account.address],
  });

  console.log(`L1 Token balance after minting: ${formatEther(l1Balance)} tokens`);
  console.log(`L2 Token balance after minting: ${formatEther(l2Balance)} tokens`);
}
