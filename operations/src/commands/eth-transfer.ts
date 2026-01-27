import { Command, Flags } from "@oclif/core";
import { Agent } from "https";
import { Result } from "neverthrow";
import {
  Client,
  createPublicClient,
  formatEther,
  Hex,
  http,
  parseEther,
  parseSignature,
  serializeTransaction,
  TransactionSerializable,
} from "viem";
import { linea } from "viem/chains";

import { address, hexString } from "../utils/common/custom-flags.js";
import { buildHttpsAgent } from "../utils/common/https-agent.js";
import { getWeb3SignerSignature } from "../utils/common/signature.js";
import { estimateTransactionGas, sendRawTransaction } from "../utils/common/transactions.js";
import { validateUrl } from "../utils/common/validation.js";
import { calculateRewards } from "../utils/eth-transfer/rewards.js";
import { validateETHThreshold } from "../utils/eth-transfer/validation.js";

export default class EthTransfer extends Command {
  static examples = [
    // Example 1: Basic usage with required flags
    `<%= config.bin %> <%= command.id %> 
      --senderAddress=0xYourSenderAddress
      --destinationAddress=0xYourDestinationAddress
      --threshold=1.5
      --blockchainRpcUrl=http://127.0.0.1:8545
      --web3SignerUrl=http://127.0.0.1:8546
      --web3SignerPublicKey=0xYourWeb3SignerPublicKey
      --tls
      --web3SignerKeystorePath=/path/to/keystore.p12
      --web3SignerKeystorePassphrase=yourPassphrase
      --web3SignerTrustedStorePath=/path/to/ca.p12
      --web3SignerTrustedStorePassphrase=yourTrustedStorePassphrase
      `,

    // Example 2: Using the dry-run flag to simulate the transaction
    `<%= config.bin %> <%= command.id %>
      --senderAddress=0xYourSenderAddress
      --destinationAddress=0xYourDestinationAddress
      --threshold=1.5
      --blockchainRpcUrl=http://127.0.0.1:8545
      --web3SignerUrl=http://127.0.0.1:8546
      --web3SignerPublicKey=0xYourWeb3SignerPublicKey
      --dryRun
      --tls
      --web3SignerKeystorePath=/path/to/keystore.p12
      --web3SignerKeystorePassphrase=yourPassphrase
      --web3SignerTrustedStorePath=/path/to/ca.p12
      --web3SignerTrustedStorePassphrase=yourTrustedStorePassphrase
      `,
  ];

  static flags = {
    senderAddress: address({
      char: "s",
      description: "Sender address",
      required: true,
      env: "ETH_TRANSFER_SENDER_ADDRESS",
    }),
    destinationAddress: address({
      char: "d",
      description: "Destination address",
      required: true,
      env: "ETH_TRANSFER_DESTINATION_ADDRESS",
    }),
    threshold: Flags.string({
      char: "t",
      description: "Balance threshold of Validator address",
      required: true,
      parse: async (input) => validateETHThreshold(input),
      env: "ETH_TRANSFER_THRESHOLD",
    }),
    blockchainRpcUrl: Flags.string({
      description: "Blockchain RPC URL",
      required: true,
      parse: async (input) => validateUrl("blockchain-rpc-url", input, ["http:", "https:"]),
      env: "ETH_TRANSFER_BLOCKCHAIN_RPC_URL",
    }),
    web3SignerUrl: Flags.string({
      description: "Web3 Signer URL",
      required: true,
      parse: async (input) => validateUrl("web3-signer-url", input, ["http:", "https:"]),
      env: "ETH_TRANSFER_WEB3_SIGNER_URL",
    }),
    web3SignerPublicKey: hexString({
      description: "Web3 Signer Public Key",
      required: true,
      env: "ETH_TRANSFER_WEB3_SIGNER_PUBLIC_KEY",
    }),
    dryRun: Flags.boolean({
      description: "Dry run flag",
      required: false,
      default: false,
      env: "ETH_TRANSFER_DRY_RUN",
    }),
    tls: Flags.boolean({
      description: "Enable TLS",
      required: false,
      default: false,
      env: "ETH_TRANSFER_TLS",
      relationships: [
        {
          type: "all",
          flags: [
            { name: "web3SignerKeystorePath", when: async (flags) => flags["tls"] === true },
            { name: "web3SignerKeystorePassphrase", when: async (flags) => flags["tls"] === true },
            { name: "web3SignerTrustedStorePath", when: async (flags) => flags["tls"] === true },
            { name: "web3SignerTrustedStorePassphrase", when: async (flags) => flags["tls"] === true },
          ],
        },
      ],
    }),
    web3SignerKeystorePath: Flags.string({
      description: "Path to the web3 signer keystore file",
      required: false,
      env: "ETH_TRANSFER_WEB3_SIGNER_KEYSTORE_PATH",
    }),
    web3SignerKeystorePassphrase: Flags.string({
      description: "Passphrase for the web3 signer keystore",
      required: false,
      env: "ETH_TRANSFER_WEB3_SIGNER_KEYSTORE_PASSPHRASE",
    }),
    web3SignerTrustedStorePath: Flags.string({
      description: "Path to the web3 signer trusted store file",
      required: false,
      env: "ETH_TRANSFER_WEB3_SIGNER_TRUSTED_STORE_PATH",
    }),
    web3SignerTrustedStorePassphrase: Flags.string({
      description: "Passphrase for the web3 signer trusted store file",
      required: false,
      env: "ETH_TRANSFER_WEB3_SIGNER_TRUSTED_STORE_PASSPHRASE",
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
      dryRun,
      tls,
      web3SignerKeystorePath,
      web3SignerKeystorePassphrase,
      web3SignerTrustedStorePath,
      web3SignerTrustedStorePassphrase,
    } = flags;

    const client = createPublicClient({
      chain: linea,
      transport: http(blockchainRpcUrl, { batch: true, retryCount: 3 }),
    });

    const [senderBalance, nonce] = await Promise.all([
      client.getBalance({ address: senderAddress }),
      client.getTransactionCount({ address: senderAddress }),
    ]);

    if (senderBalance <= parseEther(threshold)) {
      this.log(`Sender balance (${formatEther(senderBalance)} ETH) is less than threshold. No action needed.`);
      return;
    }

    const rewards = calculateRewards(senderBalance);

    if (rewards === 0n) {
      this.log(`No rewards to send.`);
      return;
    }

    const { gasLimit, baseFeePerGas, priorityFeePerGas } = this.unwrapOrError(
      await estimateTransactionGas(client, {
        to: destinationAddress,
        account: senderAddress,
        value: rewards,
        nonce,
      }),
      "Failed to estimate gas",
    );

    this.log(
      `Gas estimation: gasLimit=${gasLimit} baseFeePerGas=${baseFeePerGas} priorityFeePerGas=${priorityFeePerGas}`,
    );

    const httpsAgent = this.buildHttpsAgentIfNeeded({
      tls,
      web3SignerKeystorePath,
      web3SignerKeystorePassphrase,
      web3SignerTrustedStorePath,
      web3SignerTrustedStorePassphrase,
    });

    const transactionToSerialize: TransactionSerializable = {
      to: destinationAddress,
      type: "eip1559",
      value: rewards,
      chainId: linea.id,
      gas: gasLimit,
      maxFeePerGas: baseFeePerGas + priorityFeePerGas,
      maxPriorityFeePerGas: priorityFeePerGas,
    };

    const signature = await this.signTransaction(
      web3SignerUrl,
      web3SignerPublicKey,
      transactionToSerialize,
      httpsAgent,
    );

    if (dryRun) {
      this.log("Dry run enabled: Skipping transaction submission to blockchain.");
      this.log(`Here is the expected rewards: ${formatEther(rewards)} ETH`);
      return;
    }

    await this.broadcastTransaction(client, transactionToSerialize, signature);
  }

  /**
   * Build https agent if TLS is enabled and all required parameters are provided.
   * @param params Parameters for building the HTTPS agent.
   * @returns The built HTTPS agent or undefined if not needed.
   */
  private buildHttpsAgentIfNeeded(params: {
    tls: boolean;
    web3SignerKeystorePath?: string | undefined;
    web3SignerKeystorePassphrase?: string | undefined;
    web3SignerTrustedStorePath?: string | undefined;
    web3SignerTrustedStorePassphrase?: string | undefined;
  }): Agent | undefined {
    if (
      params.tls &&
      params.web3SignerKeystorePath &&
      params.web3SignerKeystorePassphrase &&
      params.web3SignerTrustedStorePath &&
      params.web3SignerTrustedStorePassphrase
    ) {
      this.log("Using TLS for Web3 Signer communication.");
      return buildHttpsAgent(
        params.web3SignerKeystorePath,
        params.web3SignerKeystorePassphrase,
        params.web3SignerTrustedStorePath,
        params.web3SignerTrustedStorePassphrase,
      );
    }
    return undefined;
  }

  /**
   * Sign the transaction using Web3 Signer.
   * @param web3SignerUrl Web3 Signer URL.
   * @param web3SignerPublicKey Web3 Signer Public Key.
   * @param transactionToSerialize Transaction to be serialized and signed.
   * @param httpsAgent Optional HTTPS Agent for secure communication.
   * @returns The signature as a hex string.
   */
  public async signTransaction(
    web3SignerUrl: string,
    web3SignerPublicKey: string,
    transactionToSerialize: TransactionSerializable,
    httpsAgent?: Agent,
  ): Promise<Hex> {
    return this.unwrapOrError(
      await getWeb3SignerSignature(web3SignerUrl, web3SignerPublicKey, transactionToSerialize, httpsAgent),
      "Failed to get signature from Web3 Signer",
    );
  }

  /**
   * Broadcast the signed transaction to the network.
   * @param client Viem Client.
   * @param tx Transaction to be broadcasted.
   * @param signature Signature of the transaction.
   */
  private async broadcastTransaction(client: Client, tx: TransactionSerializable, signature: Hex) {
    const signed = serializeTransaction(tx, parseSignature(signature));

    this.log(`Broadcasting submitInvoice transaction to the network...`);

    const receipt = this.unwrapOrError(await sendRawTransaction(client, signed), "Failed to send transaction");

    if (receipt.status === "reverted") {
      this.error(`Transaction failed. transactionHash=${receipt.transactionHash}`);
    }

    this.log(
      `Transaction succeed: transactionHash=${receipt.transactionHash} rewards=${formatEther(tx.value ?? 0n)} ETH`,
    );
  }

  /**
   * Unwrap a Result or throw an error with a custom message.
   * @param result The Result to unwrap.
   * @param message The error message to use if unwrapping fails.
   * @returns The unwrapped value.
   */
  private unwrapOrError<T, E extends Error = Error>(result: Result<T, E>, message: string): T {
    return result.match(
      (value) => value,
      (error) => this.error(`${message}. message=${error.message}`),
    );
  }
}
