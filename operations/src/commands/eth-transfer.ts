import { ethers, TransactionLike } from "ethers";
import { Command, Flags } from "@oclif/core";
import { validateEthereumAddress, validateHexString, validateUrl, get1559Fees } from "../utils/common/index.js";
import {
  validateETHThreshold,
  calculateRewards,
  validateConfig,
  estimateTransactionGas,
  executeTransaction,
  getWeb3SignerSignature,
  DEFAULT_GAS_ESTIMATION_PERCENTILE,
  DEFAULT_MAX_FEE_PER_GAS,
  WEB3_SIGNER_PUBLIC_KEY_LENGTH,
  getWeb3SignerHttpsAgent,
} from "../utils/eth-transfer/index.js";
import { Agent } from "https";

export default class EthTransfer extends Command {
  static examples = [
    // Example 1: Basic usage with required flags
    `<%= config.bin %> <%= command.id %> 
      --sender-address=0xYourSenderAddress
      --destination-address=0xYourDestinationAddress
      --threshold=10
      --blockchain-rpc-url=https://mainnet.infura.io/v3/YOUR-PROJECT-ID
      --web3-signer-url=http://localhost:8545
      --web3-signer-public-key=0xYourWeb3SignerPublicKey
      --web3-signer-keystore-path=/path/to/keystore.p12
      --web3-signer-passphrase=yourPassphrase
      --web3-signer-trusted-store-path=/path/to/ca.p12
      `,

    // Example 2: Including optional flags with custom values
    `<%= config.bin %> <%= command.id %>
      --sender-address=0xYourSenderAddress
      --destination-address=0xYourDestinationAddress
      --threshold=5
      --blockchain-rpc-url=https://mainnet.infura.io/v3/YOUR-PROJECT-ID
      --web3-signer-url=http://localhost:8545
      --web3-signer-public-key=0xYourWeb3SignerPublicKey
      --max-fee-per-gas=150000000000
      --gas-estimation-percentile=20
      --web3-signer-keystore-path=/path/to/keystore.p12
      --web3-signer-passphrase=yourPassphrase
      --web3-signer-trusted-store-path=/path/to/ca.p12
      `,

    // Example 3: Using the dry-run flag to simulate the transaction
    `<%= config.bin %> <%= command.id %>
      --sender-address=0xYourSenderAddress
      --destination-address=0xYourDestinationAddress
      --threshold=1.5
      --blockchain-rpc-url=http://127.0.0.1:8545
      --web3-signer-url=http://127.0.0.1:8546
      --web3-signer-public-key=0xYourWeb3SignerPublicKey
      --dry-run
      --web3-signer-keystore-path=/path/to/keystore.p12
      --web3-signer-passphrase=yourPassphrase
      --web3-signer-trusted-store-path=/path/to/ca.p12
      `,
  ];

  static flags = {
    "sender-address": Flags.string({
      char: "s",
      description: "Sender address",
      required: true,
      parse: async (input) => validateEthereumAddress("sender-address", input),
      env: "ETH_TRANSFER_SENDER_ADDRESS",
    }),
    "destination-address": Flags.string({
      char: "d",
      description: "Destination address",
      required: true,
      parse: async (input) => validateEthereumAddress("destination-address", input),
      env: "ETH_TRANSFER_DESTINATION_ADDRESS",
    }),
    threshold: Flags.string({
      char: "t",
      description: "Balance threshold of Validator address",
      required: true,
      parse: async (input) => validateETHThreshold(input),
      env: "ETH_TRANSFER_THRESHOLD",
    }),
    "blockchain-rpc-url": Flags.string({
      description: "Blockchain RPC URL",
      required: true,
      parse: async (input) => validateUrl("blockchain-rpc-url", input, ["http:", "https:"]),
      env: "ETH_TRANSFER_BLOCKCHAIN_RPC_URL",
    }),
    "web3-signer-url": Flags.string({
      description: "Web3 Signer URL",
      required: true,
      parse: async (input) => validateUrl("web3-signer-url", input, ["http:", "https:"]),
      env: "ETH_TRANSFER_WEB3_SIGNER_URL",
    }),
    "web3-signer-public-key": Flags.string({
      description: "Web3 Signer Public Key",
      required: true,
      parse: async (input) => validateHexString("web3-signer-public-key", input, WEB3_SIGNER_PUBLIC_KEY_LENGTH),
      env: "ETH_TRANSFER_WEB3_SIGNER_PUBLIC_KEY",
    }),
    "dry-run": Flags.boolean({
      description: "Dry run flag",
      required: false,
      default: false,
      env: "ETH_TRANSFER_DRY_RUN",
    }),
    "max-fee-per-gas": Flags.string({
      description: "MaxFeePerGas in wei",
      required: false,
      default: DEFAULT_MAX_FEE_PER_GAS,
      env: "ETH_TRANSFER_MAX_FEE_PER_GAS",
    }),
    "gas-estimation-percentile": Flags.integer({
      description: "Gas estimation percentile (0-100)",
      required: false,
      default: DEFAULT_GAS_ESTIMATION_PERCENTILE,
      env: "ETH_TRANSFER_GAS_ESTIMATION_PERCENTILE",
    }),
    tls: Flags.boolean({
      description: "Enable TLS",
      required: false,
      default: false,
      env: "ETH_TRANSFER_TLS",
    }),
    "web3-signer-keystore-path": Flags.string({
      description: "Path to the web3 signer keystore file",
      required: false,
      env: "ETH_TRANSFER_WEB3_SIGNER_KEYSTORE_PATH",
    }),
    "web3-signer-passphrase": Flags.string({
      description: "Passphrase for the web3 signer keystore",
      required: false,
      env: "ETH_TRANSFER_WEB3_SIGNER_KEYSTORE_PASSPHRASE",
    }),
    "web3-signer-trusted-store-path": Flags.string({
      description: "Path to the web3 signer trusted store file",
      required: false,
      env: "ETH_TRANSFER_WEB3_SIGNER_TRUSTED_STORE_PATH",
    }),
  };

  public async run(): Promise<void> {
    const { flags } = await this.parse(EthTransfer);

    const {
      senderAddress,
      destinationAddress,
      threshold,
      blockchainRpcUrl,
      web3SignerUrl,
      web3SignerPublicKey,
      maxFeePerGas,
      gasEstimationPercentile,
      dryRun,
      tls,
      web3SignerKeystorePath,
      web3SignerPassphrase,
      web3SignerTrustedStorePath,
    } = validateConfig(flags);

    const provider = new ethers.JsonRpcProvider(blockchainRpcUrl);

    const [{ chainId }, senderBalance, fees, nonce] = await Promise.all([
      provider.getNetwork(),
      provider.getBalance(senderAddress),
      get1559Fees(provider, maxFeePerGas, gasEstimationPercentile),
      provider.getTransactionCount(senderAddress),
    ]);

    if (senderBalance <= ethers.parseEther(threshold)) {
      this.log(`Sender balance (${ethers.formatEther(senderBalance)} ETH) is less than threshold. No action needed.`);
      return;
    }

    const rewards = calculateRewards(senderBalance);

    if (rewards === 0n) {
      this.log(`No rewards to send.`);
      return;
    }

    const transactionRequest: TransactionLike = {
      to: destinationAddress,
      value: rewards,
      type: 2,
      chainId,
      maxFeePerGas: fees.maxFeePerGas,
      maxPriorityFeePerGas: fees.maxPriorityFeePerGas ?? null,
      nonce: nonce,
    };

    const transactionGasLimit = await estimateTransactionGas(provider, {
      ...transactionRequest,
      from: senderAddress,
    } as ethers.TransactionRequest);

    const transaction: TransactionLike = {
      ...transactionRequest,
      gasLimit: transactionGasLimit,
    };

    let httpsAgent: Agent | undefined;
    if (tls) {
      this.log(`Using TLS for secure communication with Web3 Signer.`);
      httpsAgent = getWeb3SignerHttpsAgent(web3SignerKeystorePath, web3SignerPassphrase, web3SignerTrustedStorePath);
    }

    const signature = await getWeb3SignerSignature(web3SignerUrl, web3SignerPublicKey, transaction, httpsAgent);

    if (dryRun) {
      this.log("Dry run enabled: Skipping transaction submission to blockchain.");
      this.log(`Here is the expected rewards: ${ethers.formatEther(rewards)} ETH`);
      return;
    }

    const receipt = await executeTransaction(provider, {
      ...transaction,
      signature,
    });

    if (!receipt) {
      throw new Error(`Transaction receipt not found for this transaction ${JSON.stringify(transaction)}`);
    }

    if (receipt.status === 0) {
      throw new Error(`Transaction reverted. Receipt: ${JSON.stringify(receipt)}`);
    }

    this.log(
      `Transaction succeeded. Rewards sent: ${ethers.formatEther(rewards)} ETH. Receipt: ${JSON.stringify(receipt)}`,
    );
    this.log(`Rewards sent: ${ethers.formatEther(rewards)} ETH`);
  }
}
