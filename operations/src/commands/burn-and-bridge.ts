import { Command, Flags } from "@oclif/core";
import {
  Address,
  Client,
  createPublicClient,
  formatEther,
  formatUnits,
  Hex,
  http,
  parseEventLogs,
  parseSignature,
  serializeTransaction,
  TransactionSerializable,
} from "viem";
import { linea, lineaSepolia } from "viem/chains";
import { Agent } from "https";
import { fromZonedTime } from "date-fns-tz";
import { Result } from "neverthrow";
import { addSeconds } from "date-fns";
import { getBalance } from "viem/actions";
import { estimateTransactionGas, sendTransaction } from "../utils/common/transactions.js";
import { getWeb3SignerSignature } from "../utils/common/signature.js";
import { buildHttpsAgent } from "../utils/common/https-agent.js";
import { validateUrl } from "../utils/common/validation.js";
import { address, hexString } from "../utils/common/custom-flags.js";
import {
  computeBurnAndBridgeCalldata,
  computeSwapCalldata,
  getInvoiceArrears,
  getMinimumFee,
  getQuote,
} from "../utils/burn-and-bridge/contract.js";
import { LINEA_TOKEN_ADDRESS, WETH_TOKEN_ADDRESS } from "../utils/burn-and-bridge/constants.js";
import { ETH_BURNT_SWAPPED_AND_BRIDGED_EVENT_ABI } from "../utils/burn-and-bridge/abi.js";

export default class BurnAndBridge extends Command {
  static examples = [
    `<%= config.bin %> <%= command.id %> 
      --senderAddress=0xYourSenderAddress
      --rollupRevenueVaultContractAddress=0xYourContractAddress
      --quoteContractAddress=0xYourQuoteContractAddress
      --rpcUrl=https://mainnet.infura.io/v3/YOUR-PROJECT-ID
      --web3SignerUrl=http://localhost:8545
      --web3SignerPublicKey=0xYourWeb3SignerPublicKey
      --tls
      --web3SignerKeystorePath=/path/to/keystore.p12
      --web3SignerKeystorePassphrase=yourPassphrase
      --web3SignerTrustedStorePath=/path/to/ca.p12
      --web3SignerTrustedStorePassphrase=yourTrustedStorePassphrase
      --swapAmountSlippageBps=50
      --swapDeadlineInSeconds=300
      --poolTickSpacing=50
    `,
    // Dry run
    `<%= config.bin %> <%= command.id %>
      --senderAddress=0xYourSenderAddress
      --rollupRevenueVaultContractAddress=0xYourContractAddress
      --quoteContractAddress=0xYourQuoteContractAddress
      --rpcUrl=https://mainnet.infura.io/v3/YOUR-PROJECT-ID
      --web3SignerUrl=http://localhost:8545
      --web3SignerPublicKey=0xYourWeb3SignerPublicKey
      --tls
      --web3SignerKeystorePath=/path/to/keystore.p12
      --web3SignerKeystorePassphrase=yourPassphrase
      --web3SignerTrustedStorePath=/path/to/ca.p12
      --web3SignerTrustedStorePassphrase=yourTrustedStorePassphrase
      --swapAmountSlippageBps=50
      --swapDeadlineInSeconds=300
      --poolTickSpacing=50
      --dryRun
    `,
  ];

  static strict = true;

  static flags = {
    senderAddress: address({
      char: "s",
      description: "Sender address",
      required: true,
      env: "BURN_AND_BRIDGE_SENDER_ADDRESS",
    }),
    rollupRevenueVaultContractAddress: address({
      description: "Rollup Revenue Vault Contract address",
      required: true,
      env: "BURN_AND_BRIDGE_ROLLUP_REVENUE_VAULT_CONTRACT_ADDRESS",
    }),
    l2MessageServiceContractAddress: address({
      description: "L2 Message Service Contract address",
      required: true,
      env: "BURN_AND_BRIDGE_L2_MESSAGE_SERVICE_CONTRACT_ADDRESS",
    }),
    quoteContractAddress: address({
      description: "Quote Contract address",
      required: true,
      env: "BURN_AND_BRIDGE_QUOTE_CONTRACT_ADDRESS",
    }),
    rpcUrl: Flags.string({
      description: "Blockchain RPC URL",
      required: true,
      parse: async (input) => validateUrl("blockchain-rpc-url", input, ["http:", "https:"]),
      env: "BURN_AND_BRIDGE_BLOCKCHAIN_RPC_URL",
    }),
    web3SignerUrl: Flags.string({
      description: "Web3 Signer URL",
      required: true,
      parse: async (input) => validateUrl("web3-signer-url", input, ["http:", "https:"]),
      env: "BURN_AND_BRIDGE_WEB3_SIGNER_URL",
    }),
    web3SignerPublicKey: hexString({
      description: "Web3 Signer Public Key",
      required: true,
      env: "BURN_AND_BRIDGE_WEB3_SIGNER_PUBLIC_KEY",
    }),
    dryRun: Flags.boolean({
      description: "Dry run flag",
      required: false,
      default: false,
      env: "BURN_AND_BRIDGE_DRY_RUN",
    }),
    tls: Flags.boolean({
      description: "Enable TLS",
      required: false,
      default: false,
      env: "BURN_AND_BRIDGE_TLS",
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
      env: "BURN_AND_BRIDGE_WEB3_SIGNER_KEYSTORE_PATH",
    }),
    web3SignerKeystorePassphrase: Flags.string({
      description: "Passphrase for the web3 signer keystore",
      required: false,
      env: "BURN_AND_BRIDGE_WEB3_SIGNER_KEYSTORE_PASSPHRASE",
    }),
    web3SignerTrustedStorePath: Flags.string({
      description: "Path to the web3 signer trusted store file",
      required: false,
      env: "BURN_AND_BRIDGE_WEB3_SIGNER_TRUSTED_STORE_PATH",
    }),
    web3SignerTrustedStorePassphrase: Flags.string({
      description: "Passphrase for the web3 signer trusted store file",
      required: false,
      env: "BURN_AND_BRIDGE_WEB3_SIGNER_TRUSTED_STORE_PASSPHRASE",
    }),
    swapAmountSlippageBps: Flags.integer({
      description: "Allowed slippage in basis points for the swap",
      required: true,
      env: "BURN_AND_BRIDGE_SWAP_AMOUNT_SLIPPAGE_BPS",
    }),
    swapDeadlineInSeconds: Flags.integer({
      description: "Swap deadline in seconds",
      required: true,
      env: "BURN_AND_BRIDGE_SWAP_DEADLINE_IN_SECONDS",
    }),
    poolTickSpacing: Flags.integer({
      description: "Tick spacing of the pool used for quote",
      required: true,
      env: "BURN_AND_BRIDGE_POOL_TICK_SPACING",
    }),
    isTestnet: Flags.boolean({
      description: "Whether to use the testnet chain (Linea Sepolia)",
      required: false,
      default: false,
      env: "BURN_AND_BRIDGE_IS_TESTNET",
    }),
  };

  public async run(): Promise<void> {
    const { flags } = await this.parse(BurnAndBridge);
    const {
      senderAddress,
      rpcUrl,
      rollupRevenueVaultContractAddress,
      l2MessageServiceContractAddress,
      quoteContractAddress,
      web3SignerUrl,
      web3SignerPublicKey,
      web3SignerKeystorePath,
      web3SignerKeystorePassphrase,
      web3SignerTrustedStorePath,
      web3SignerTrustedStorePassphrase,
      tls,
      dryRun,
      swapAmountSlippageBps,
      swapDeadlineInSeconds,
      poolTickSpacing,
      isTestnet,
    } = flags;

    const chain = isTestnet ? lineaSepolia : linea;

    const client = createPublicClient({
      chain,
      transport: http(rpcUrl, { batch: true, retryCount: 3 }),
    });

    /******************************
          FETCH CHAIN DATA
     ******************************/
    const { invoiceArrearsInWei, vaultBalance, minimumFeeInWei } = await this.fetchChainData(
      client,
      rollupRevenueVaultContractAddress,
      l2MessageServiceContractAddress,
    );

    this.log(
      `Fetched chain data: invoiceArrears=${formatEther(invoiceArrearsInWei)} ETH, vaultBalance=${formatEther(
        vaultBalance,
      )} ETH, minimumFee=${formatEther(minimumFeeInWei)} ETH`,
    );

    /******************************
        CHECK IF SWAP NEEDED
     ******************************/

    // If arrears are greater than the balance, we can only pay off arrears and then just use empty swapData
    // If arrears == balance - minimumFee, then there is no ETH left to swap and burn. use empty swapData
    // If arrears == 0, and balance <= minimumFee, stop the script.
    // Otherwise, normal case.
    const shouldSwap = this.shouldSwap(invoiceArrearsInWei, vaultBalance, minimumFeeInWei);

    if (!shouldSwap) {
      this.log("Skipping swap due to insufficient funds.");
    }

    /******************************
        COMPUTE TRANSACTION DATA
     ******************************/

    const minLineaOut = shouldSwap
      ? await this.computeMinLineaOut(
          client,
          quoteContractAddress,
          vaultBalance,
          minimumFeeInWei,
          swapAmountSlippageBps,
          poolTickSpacing,
        )
      : undefined;

    const deadline = this.computeSwapDeadline(swapDeadlineInSeconds);
    const swapCalldata = shouldSwap ? computeSwapCalldata(minLineaOut!, deadline) : "0x";
    const burnAndBridgeCalldata = computeBurnAndBridgeCalldata(swapCalldata);

    /******************************
      ESTIMATE GAS & SIGN TRANSACTION
     ******************************/

    const { gasLimit, baseFeePerGas, priorityFeePerGas } = this.unwrapOrError(
      await estimateTransactionGas(client, {
        to: rollupRevenueVaultContractAddress,
        account: senderAddress,
        value: 0n,
        data: burnAndBridgeCalldata,
      }),
      "Failed to estimate gas for burn and bridge transaction",
    );

    this.log(
      `Gas estimation for burnAndBridge transaction: gasLimit=${gasLimit} baseFeePerGas=${baseFeePerGas} priorityFeePerGas=${priorityFeePerGas}`,
    );

    const httpsAgent = this.buildHttpsAgentIfNeeded({
      tls,
      web3SignerKeystorePath,
      web3SignerKeystorePassphrase,
      web3SignerTrustedStorePath,
      web3SignerTrustedStorePassphrase,
    });

    const transactionToSerialize: TransactionSerializable = {
      to: rollupRevenueVaultContractAddress,
      type: "eip1559",
      value: 0n,
      data: burnAndBridgeCalldata,
      chainId: chain.id,
      gas: gasLimit,
      maxFeePerGas: baseFeePerGas + priorityFeePerGas,
      maxPriorityFeePerGas: priorityFeePerGas,
    };

    const signature = await this.signBurnAndBridgeTransaction(
      web3SignerUrl,
      web3SignerPublicKey,
      transactionToSerialize,
      httpsAgent,
    );

    if (dryRun) {
      this.log(`Dry run mode - transaction not submitted.`);
      return;
    }
    /******************************
          BROADCAST TRANSACTION
     ******************************/

    await this.broadcastTransaction(client, transactionToSerialize, signature);
  }

  /**
   * Sign the burn and bridge transaction using Web3 Signer.
   * @param web3SignerUrl Web3 Signer URL.
   * @param web3SignerPublicKey Web3 Signer Public Key.
   * @param transactionToSerialize Transaction to be serialized and signed.
   * @param httpsAgent Optional HTTPS Agent for secure communication.
   * @returns The signature as a hex string.
   */
  private async signBurnAndBridgeTransaction(
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
   * Compute the swap deadline as a bigint in seconds.
   * @param delayInSeconds Delay in seconds to add to the current time.
   * @returns The computed swap deadline as a bigint.
   */
  private computeSwapDeadline(delayInSeconds: number): bigint {
    const currentTime = fromZonedTime(Math.floor(new Date().getTime()), "UTC");
    const deadline = addSeconds(currentTime, delayInSeconds);
    return BigInt(Math.floor(deadline.getTime() / 1000));
  }

  /**
   * Compute the minimum LINEA tokens expected from the swap after accounting for slippage.
   * @param client Viem Client.
   * @param quoteContractAddress Quote Contract address.
   * @param rollupRevenueVaultContractBalance Balance of the Rollup Revenue Vault Contract.
   * @param minimumFeeInWei Minimum fee in wei.
   * @param swapAmountSlippageBps Allowed slippage in basis points for the swap.
   * @param poolTickSpacing Tick spacing of the pool used for quote.
   * @returns The minimum LINEA tokens expected from the swap as a bigint.
   */
  private async computeMinLineaOut(
    client: Client,
    quoteContractAddress: Address,
    rollupRevenueVaultContractBalance: bigint,
    minimumFeeInWei: bigint,
    swapAmountSlippageBps: number,
    poolTickSpacing: number,
  ): Promise<bigint> {
    const balanceAvailable = rollupRevenueVaultContractBalance - minimumFeeInWei;
    const ETH_BURNT_PERCENTAGE = 20n; // 20% of the ETH balance will be burnt
    const ethToBurn = (balanceAvailable * ETH_BURNT_PERCENTAGE) / 100n;

    const amountToBeSwappedInWei = balanceAvailable - ethToBurn;

    const [minAmountOut] = this.unwrapOrError(
      await getQuote(client, quoteContractAddress, {
        tokenIn: WETH_TOKEN_ADDRESS,
        tokenOut: LINEA_TOKEN_ADDRESS,
        amountIn: amountToBeSwappedInWei,
        tickSpacing: poolTickSpacing,
        sqrtPriceLimitX96: 0n,
      }),
      "Failed to get quote from quote contract",
    );

    const slippageBps = BigInt(swapAmountSlippageBps);
    const minLineaOut = (minAmountOut * (10_000n - slippageBps)) / 10_000n;

    this.log(
      `Minimum LINEA out (after slippage): minLineaOut=${formatUnits(minLineaOut, 18)} LINEA slippageBps=${swapAmountSlippageBps}`,
    );

    return minLineaOut;
  }

  /**
   * Check whether a swap should be performed based on invoice, balance, and minimum fee.
   * @param invoice Invoice amount in wei.
   * @param vaultBalance Vault balance amount in wei.
   * @param minfee Minimum fee amount in wei.
   * @returns Whether the swap should be performed.
   */
  private shouldSwap(invoice: bigint, vaultBalance: bigint, minfee: bigint): boolean {
    if (invoice > vaultBalance) return false;
    if (invoice === vaultBalance - minfee) return false;
    if (invoice === 0n && vaultBalance <= minfee) {
      this.error(
        "No funds available to perform burn and bridge. Invoice arrears is zero and balance is less than or equal to minimum fee.",
      );
    }
    return true;
  }

  /**
   * Fetch necessary chain data: invoice arrears, vault balance, and minimum fee.
   * @param client Viem Client.
   * @param vaultAddress Rollup Revenue Vault Contract address.
   * @param messageServiceAddress L2 Message Service Contract address.
   * @returns An object containing invoice arrears, vault balance, and minimum fee in wei.
   */
  private async fetchChainData(client: Client, vaultAddress: Address, messageServiceAddress: Address) {
    const invoiceArrearsInWei = this.unwrapOrError(
      await getInvoiceArrears(client, vaultAddress),
      "Failed to get invoice arrears",
    );

    const vaultBalance = await getBalance(client, { address: vaultAddress });

    const minimumFeeInWei = this.unwrapOrError(
      await getMinimumFee(client, messageServiceAddress),
      "Failed to get minimum fee",
    );

    return { invoiceArrearsInWei, vaultBalance, minimumFeeInWei };
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
   * Broadcast the signed transaction to the network.
   * @param client Viem Client.
   * @param tx Transaction to be broadcasted.
   * @param signature Signature of the transaction.
   */
  private async broadcastTransaction(client: Client, tx: TransactionSerializable, signature: Hex) {
    const signed = serializeTransaction(tx, parseSignature(signature));
    this.log("Broadcasting transaction...");
    const receipt = this.unwrapOrError(await sendTransaction(client, signed), "Failed to send transaction");

    if (receipt.status === "reverted") {
      this.error(`Burn and bridge failed. transactionHash=${receipt.transactionHash}`);
    }

    const [event] = parseEventLogs({
      abi: ETH_BURNT_SWAPPED_AND_BRIDGED_EVENT_ABI,
      logs: receipt.logs,
      eventName: "EthBurntSwappedAndBridged",
    });

    this.log(
      `Burn and bridge transaction successfully processed. transactionHash=${receipt.transactionHash} ethBurnt=${formatEther(event.args.ethBurnt)} lineaTokensBridged=${formatUnits(event.args.lineaTokensBridged, 18)}`,
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
