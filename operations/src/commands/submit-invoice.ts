import { Command, Flags } from "@oclif/core";
import {
  Address,
  Client,
  createPublicClient,
  formatEther,
  Hex,
  http,
  parseEther,
  parseEventLogs,
  parseSignature,
  serializeTransaction,
  TransactionSerializable,
} from "viem";
import { linea, lineaSepolia } from "viem/chains";
import { getBlock } from "viem/actions";
import { Agent } from "https";
import { GetCostAndUsageCommandInput } from "@aws-sdk/client-cost-explorer";
import { formatInTimeZone, fromZonedTime } from "date-fns-tz";
import { addDays } from "date-fns";
import { Result } from "neverthrow";
import { computeInvoicePeriod, InvoicePeriod } from "../utils/submit-invoice/time.js";
import { generateQueryParameters, getDuneClient, runDuneQuery } from "../utils/common/dune.js";
import { estimateTransactionGas, sendRawTransaction } from "../utils/common/transactions.js";
import { getWeb3SignerSignature } from "../utils/common/signature.js";
import { INVOICE_PROCESSED_EVENT_ABI } from "../utils/submit-invoice/abi.js";
import { buildHttpsAgent } from "../utils/common/https-agent.js";
import { createAwsCostExplorerClient, flattenResultsByTime, getDailyAwsCosts } from "../utils/common/aws.js";
import { computeSubmitInvoiceCalldata, getLastInvoiceDate } from "../utils/submit-invoice/contract.js";
import { validateUrl } from "../utils/common/validation.js";
import { address, hexString } from "../utils/common/custom-flags.js";
import { awsCostsApiFilters } from "../utils/submit-invoice/custom-flags.js";
import { fetchEthereumPrice } from "../utils/common/coingecko.js";

export default class SubmitInvoice extends Command {
  static examples = [
    `<%= config.bin %> <%= command.id %> 
      --senderAddress=0xYourSenderAddress
      --contractAddress=0xYourContractAddress
      --periodDays=10
      --reportingLagDays=2
      --rpcUrl=https://mainnet.infura.io/v3/YOUR-PROJECT-ID
      --web3SignerUrl=http://localhost:8545
      --web3SignerPublicKey=0xYourWeb3SignerPublicKey
      --duneApiKey=YOUR_DUNE_KEY
      --duneQueryId=12345
      --tls
      --web3SignerKeystorePath=/path/to/keystore.p12
      --web3SignerKeystorePassphrase=yourPassphrase
      --web3SignerTrustedStorePath=/path/to/ca.p12
      --web3SignerTrustedStorePassphrase=yourTrustedStorePassphrase
      --awsCostsApiFilters='{"Granularity":"DAILY","Metrics":["AmortizedCost"],"GroupBy":[]}'
      --coingeckoApiBaseUrl=https://api.coingecko.com/api/v3
      --coingeckoApiKey=YOUR_COINGECKO_KEY
    `,
    // Dry run
    `<%= config.bin %> <%= command.id %>
      --senderAddress=0xYourSenderAddress
      --contractAddress=0xYourContractAddress
      --periodDays=10
      --reportingLagDays=2
      --rpcUrl=https://mainnet.infura.io/v3/YOUR-PROJECT-ID
      --web3SignerUrl=http://localhost:8545
      --web3SignerPublicKey=0xYourWeb3SignerPublicKey
      --duneApiKey=YOUR_DUNE_KEY
      --duneQueryId=12345
      --tls
      --web3SignerKeystorePath=/path/to/keystore.p12
      --web3SignerKeystorePassphrase=yourPassphrase
      --web3SignerTrustedStorePath=/path/to/ca.p12
      --web3SignerTrustedStorePassphrase=yourTrustedStorePassphrase
      --awsCostsApiFilters='{"Granularity":"DAILY","Metrics":["AmortizedCost"],"GroupBy":[]}'
      --coingeckoApiBaseUrl=https://api.coingecko.com/api/v3
      --coingeckoApiKey=YOUR_COINGECKO_KEY
      --dryRun
    `,
  ];

  static strict = true;

  static flags = {
    senderAddress: address({
      char: "s",
      description: "Sender address",
      required: true,
      env: "SUBMIT_INVOICE_SENDER_ADDRESS",
    }),
    contractAddress: address({
      char: "c",
      description: "Contract address",
      required: true,
      env: "SUBMIT_INVOICE_CONTRACT_ADDRESS",
    }),
    periodDays: Flags.integer({
      char: "p",
      description: "Period in days for invoice generation",
      required: true,
      parse: async (input) => parseInt(input),
      env: "SUBMIT_INVOICE_PERIOD_DAYS",
    }),
    reportingLagDays: Flags.integer({
      char: "r",
      description: "Reporting lag in days for invoice generation",
      required: true,
      parse: async (input) => parseInt(input),
      env: "SUBMIT_INVOICE_REPORTING_LAG_DAYS",
    }),
    rpcUrl: Flags.string({
      description: "Blockchain RPC URL",
      required: true,
      parse: async (input) => validateUrl("rpcUrl", input, ["http:", "https:"]),
      env: "SUBMIT_INVOICE_BLOCKCHAIN_RPC_URL",
    }),
    web3SignerUrl: Flags.string({
      description: "Web3 Signer URL",
      required: true,
      parse: async (input) => validateUrl("web3SignerUrl", input, ["http:", "https:"]),
      env: "SUBMIT_INVOICE_WEB3_SIGNER_URL",
    }),
    web3SignerPublicKey: hexString({
      description: "Web3 Signer Public Key",
      required: true,
      env: "SUBMIT_INVOICE_WEB3_SIGNER_PUBLIC_KEY",
    }),
    dryRun: Flags.boolean({
      description: "Dry run flag",
      required: false,
      default: false,
      env: "SUBMIT_INVOICE_DRY_RUN",
    }),
    tls: Flags.boolean({
      description: "Enable TLS",
      required: false,
      default: false,
      env: "SUBMIT_INVOICE_TLS",
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
      env: "SUBMIT_INVOICE_WEB3_SIGNER_KEYSTORE_PATH",
    }),
    web3SignerKeystorePassphrase: Flags.string({
      description: "Passphrase for the web3 signer keystore",
      required: false,
      env: "SUBMIT_INVOICE_WEB3_SIGNER_KEYSTORE_PASSPHRASE",
    }),
    web3SignerTrustedStorePath: Flags.string({
      description: "Path to the web3 signer trusted store file",
      required: false,
      env: "SUBMIT_INVOICE_WEB3_SIGNER_TRUSTED_STORE_PATH",
    }),
    web3SignerTrustedStorePassphrase: Flags.string({
      description: "Passphrase for the web3 signer trusted store file",
      required: false,
      env: "SUBMIT_INVOICE_WEB3_SIGNER_TRUSTED_STORE_PASSPHRASE",
    }),
    duneApiKey: Flags.string({
      description: "Dune API Key",
      required: true,
      env: "SUBMIT_INVOICE_DUNE_API_KEY",
    }),
    duneQueryId: Flags.integer({
      description: "Dune Query ID",
      required: true,
      parse: async (input) => parseInt(input),
      env: "SUBMIT_INVOICE_DUNE_QUERY_ID",
    }),
    awsCostsApiFilters: awsCostsApiFilters({
      description: "AWS Costs API Filters as JSON string",
      required: true,
      env: "SUBMIT_INVOICE_AWS_COSTS_API_FILTERS",
    }),
    coingeckoApiBaseUrl: Flags.string({
      description: "CoinGecko API Base URL",
      required: true,
      parse: async (input) => validateUrl("coingeckoApiBaseUrl", input, ["http:", "https:"]),
      env: "SUBMIT_INVOICE_COINGECKO_API_BASE_URL",
    }),
    coingeckoApiKey: Flags.string({
      description: "CoinGecko API Key",
      required: true,
      env: "SUBMIT_INVOICE_COINGECKO_API_KEY",
    }),
    isTestnet: Flags.boolean({
      description: "Whether to use the testnet chain (Linea Sepolia)",
      required: false,
      default: false,
      env: "SUBMIT_INVOICE_IS_TESTNET",
    }),
  };

  public async run(): Promise<void> {
    const { flags } = await this.parse(SubmitInvoice);
    const {
      senderAddress,
      rpcUrl,
      contractAddress,
      periodDays,
      reportingLagDays,
      web3SignerUrl,
      web3SignerPublicKey,
      web3SignerKeystorePath,
      web3SignerKeystorePassphrase,
      web3SignerTrustedStorePath,
      web3SignerTrustedStorePassphrase,
      tls,
      dryRun,
      awsCostsApiFilters,
      duneApiKey,
      duneQueryId,
      coingeckoApiBaseUrl,
      coingeckoApiKey,
      isTestnet,
    } = flags;

    const chain = isTestnet ? lineaSepolia : linea;

    const client = createPublicClient({
      chain,
      transport: http(rpcUrl, { batch: true, retryCount: 3 }),
    });

    /******************************
      INVOICE PERIOD COMPUTATION
     ******************************/

    const invoicePeriod = await this.getInvoicePeriod(client, contractAddress, periodDays, reportingLagDays);

    if (!invoicePeriod) {
      this.warn("No invoice to process at this time.");
      return;
    }

    const invoicePeriodStartDateStr = invoicePeriod.startDate.toISOString();
    const invoicePeriodEndDateStr = invoicePeriod.endDate.toISOString();

    this.log(`Invoice period to process: startDate=${invoicePeriodStartDateStr} endDate=${invoicePeriodEndDateStr}`);

    /******************************
      AWS HISTORICAL COSTS FETCHING
     ******************************/

    await this.getAWSCostsHistoricalData(invoicePeriod.startDate, awsCostsApiFilters);

    /******************************
            AWS COSTS FETCHING
     ******************************/

    const awsCostsInUsd = await this.getAWSCosts(invoicePeriod, awsCostsApiFilters);

    if (awsCostsInUsd === undefined) {
      this.warn("AWS costs are undefined, likely due to data still being estimated. Stopping invoice processing.");
      return;
    }

    this.log(
      `Total AWS costs costsInUsd=${awsCostsInUsd} for the period: startDate=${invoicePeriodStartDateStr} endDate=${invoicePeriodEndDateStr}`,
    );

    /******************************
        ONCHAIN COSTS FETCHING
     ******************************/

    const onChainCostsInEth = await this.getOnChainCosts(duneApiKey, duneQueryId, invoicePeriod);

    this.log(
      `Total on-chain costs costsInEth=${onChainCostsInEth} for the period: startDate=${invoicePeriodStartDateStr} endDate=${invoicePeriodEndDateStr}`,
    );

    /******************************
        TOTAL COSTS COMPUTATION
     ******************************/

    const ethPriceResponse = this.unwrapOrError(
      await fetchEthereumPrice(coingeckoApiBaseUrl, coingeckoApiKey),
      "Failed to fetch Ethereum price from CoinGecko",
    );

    if (!ethPriceResponse?.ethereum || !ethPriceResponse?.ethereum?.usd) {
      this.error("Ethereum price data is missing in the CoinGecko response.");
    }

    const etherPriceInUsd = ethPriceResponse.ethereum.usd;

    if (!etherPriceInUsd || etherPriceInUsd === 0) {
      this.error(`Invalid Ethereum price fetched from CoinGecko. priceFetchedInUsd=${etherPriceInUsd}`);
    }

    this.log(`Fetched Ethereum price: etherPriceInUsd=${etherPriceInUsd}`);

    const awsCostsInEth = awsCostsInUsd / etherPriceInUsd;
    const totalCostsInEth = parseEther((awsCostsInEth + onChainCostsInEth).toString());

    if (totalCostsInEth === 0n) {
      this.warn("No costs to process at this time.");
      this.warn("Please check your Dune query and AWS costs API calls.");
      return;
    }

    this.log(`Total costs to invoice: costsInEth=${formatEther(totalCostsInEth)} etherPriceInUsd=${etherPriceInUsd}`);

    /******************************
      TRANSACTION GAS ESTIMATION
     ******************************/

    const submitInvoiceCalldata = computeSubmitInvoiceCalldata(
      BigInt(Math.floor(invoicePeriod.startDate.getTime() / 1000)),
      BigInt(Math.floor(invoicePeriod.endDate.getTime() / 1000)),
      totalCostsInEth,
    );

    const { gasLimit, baseFeePerGas, priorityFeePerGas } = this.unwrapOrError(
      await estimateTransactionGas(client, {
        to: contractAddress,
        account: senderAddress,
        value: 0n,
        data: submitInvoiceCalldata,
      }),
      "Failed to estimate gas for submitInvoice transaction",
    );

    this.log(
      `Gas estimation for submitInvoice transaction: gasLimit=${gasLimit} baseFeePerGas=${baseFeePerGas} priorityFeePerGas=${priorityFeePerGas}`,
    );

    /******************************
          SIGNING TRANSACTION
     ******************************/

    const senderAddressNonce = await client.getTransactionCount({ address: senderAddress });

    const transactionToSerialize: TransactionSerializable = {
      to: contractAddress,
      type: "eip1559",
      value: 0n,
      data: submitInvoiceCalldata,
      chainId: chain.id,
      gas: gasLimit,
      maxFeePerGas: baseFeePerGas + priorityFeePerGas,
      maxPriorityFeePerGas: priorityFeePerGas,
      nonce: senderAddressNonce,
    };

    const httpsAgent = this.buildHttpsAgentIfNeeded({
      tls,
      web3SignerKeystorePath,
      web3SignerKeystorePassphrase,
      web3SignerTrustedStorePath,
      web3SignerTrustedStorePassphrase,
    });

    const signature = await this.signSubmitInvoiceTransaction(
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
        BROADCASTING TRANSACTION
     ******************************/

    await this.broadcastTransaction(client, transactionToSerialize, signature);
  }

  /**
   * Compute the invoice period based on the last invoice date, current date, period days and reporting lag days.
   * @param client Viem Client.
   * @param contractAddress Rollup Revenue Vault contract address.
   * @param periodDays Number of days in the invoice period.
   * @param reportingLagDays Number of days to wait before reporting.
   * @returns The start and end dates of the invoice period, or null if no invoice is to be processed.
   */
  private async getInvoicePeriod(
    client: Client,
    contractAddress: Address,
    periodDays: number,
    reportingLagDays: number,
  ): Promise<InvoicePeriod | null> {
    const lastInvoiceDate = this.unwrapOrError(
      await getLastInvoiceDate(client, contractAddress),
      "Failed to retrieve the last invoice date from the contract",
    );

    this.log(`Last invoice date (timestamp in seconds): lastInvoiceDate=${lastInvoiceDate}`);

    const currentTimestampInSeconds = Math.floor(Date.now() / 1000);
    return computeInvoicePeriod(Number(lastInvoiceDate), currentTimestampInSeconds, periodDays, reportingLagDays);
  }

  /**
   * Fetch AWS costs for the given invoice period using the provided AWS Costs API filters.
   * Returns undefined if the data is still being estimated.
   * @param invoicePeriod Invoice period with start and end dates.
   * @param awsCostsApiFilters AWS Costs API filters to apply.
   * @returns The total AWS costs for the specified invoice period.
   */
  private async getAWSCosts(
    invoicePeriod: InvoicePeriod,
    awsCostsApiFilters: GetCostAndUsageCommandInput,
  ): Promise<number | undefined> {
    const awsClient = createAwsCostExplorerClient({ region: "us-east-1" });
    const startDateStr = formatInTimeZone(invoicePeriod.startDate, "UTC", "yyyy-MM-dd");
    const endDateStr = formatInTimeZone(invoicePeriod.endDate, "UTC", "yyyy-MM-dd");
    const awsEndDateStr = formatInTimeZone(addDays(invoicePeriod.endDate, 1), "UTC", "yyyy-MM-dd");

    this.log(`Fetching AWS costs for the invoice period startDate=${startDateStr} endDate=${endDateStr}`);

    if (!awsCostsApiFilters.Metrics || awsCostsApiFilters.Metrics.length !== 1) {
      this.error("AWS Costs API Filters must specify one metric.");
    }

    const { ResultsByTime } = this.unwrapOrError(
      await getDailyAwsCosts(awsClient, {
        ...awsCostsApiFilters,
        TimePeriod: {
          Start: startDateStr,
          End: awsEndDateStr,
        },
      }),
      "Failed to fetch AWS costs",
    );

    if (!Array.isArray(ResultsByTime) || ResultsByTime.length === 0) {
      this.error(`No AWS cost data returned for the specified period. startDate=${startDateStr} endDate=${endDateStr}`);
    }

    const estimated = ResultsByTime[0]?.Estimated;
    if (estimated === undefined || estimated === true) {
      this.warn(
        `AWS cost data for the specified period is still under estimation. startDate=${startDateStr} endDate=${endDateStr}`,
      );
      return;
    }

    const metric = awsCostsApiFilters.Metrics[0];
    const firstResult = ResultsByTime[0];
    const totalForMetric = firstResult?.Total?.[metric];

    if (!totalForMetric || !totalForMetric?.Amount) {
      this.error(
        `AWS cost data does not contain the specified metric or Amount field. metric=${metric} startDate=${startDateStr} endDate=${endDateStr}`,
      );
    }

    return parseFloat(totalForMetric.Amount);
  }

  /**
   * Fetch on-chain costs for the given invoice period using Dune Analytics.
   * @param duneApiKey Dune API key.
   * @param duneQueryId Dune query ID.
   * @param invoicePeriod Invoice period with start and end dates.
   * @returns The total on-chain costs in ETH for the specified invoice period.
   */
  private async getOnChainCosts(
    duneApiKey: string,
    duneQueryId: number,
    invoicePeriod: InvoicePeriod,
  ): Promise<number> {
    const duneClient = getDuneClient(duneApiKey);
    const { result } = this.unwrapOrError(
      await runDuneQuery(
        duneClient,
        duneQueryId,
        generateQueryParameters({
          startDate: invoicePeriod.startDate,
          endDate: invoicePeriod.endDate,
        }),
      ),
      "Failed to run Dune query",
    );

    if (!result || !result.rows || result.rows.length === 0) {
      this.error(
        `No Dune query result returned for the specified period. startDate=${invoicePeriod.startDate.toISOString()} endDate=${invoicePeriod.endDate.toISOString()}`,
      );
    }
    return result.rows.reduce((acc, row) => acc + (row.total_costs_per_day as number), 0);
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
   * Sign the submit invoice transaction using Web3 Signer.
   * @param web3SignerUrl Web3 Signer URL.
   * @param web3SignerPublicKey Web3 Signer Public Key.
   * @param transactionToSerialize Transaction to be serialized and signed.
   * @param httpsAgent Optional HTTPS Agent for secure communication.
   * @returns The signature as a hex string.
   */
  public async signSubmitInvoiceTransaction(
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

    const startTxDate = fromZonedTime(new Date(), "UTC");
    const receipt = this.unwrapOrError(await sendRawTransaction(client, signed), "Failed to send transaction");

    const block = await getBlock(client, { blockNumber: receipt.blockNumber });

    const transactionConfirmationTime = block.timestamp - BigInt(Math.floor(startTxDate.getTime() / 1000));

    if (receipt.status === "reverted") {
      this.error(`Invoice submission failed. transactionHash=${receipt.transactionHash}`);
    }

    const [event] = parseEventLogs({
      abi: INVOICE_PROCESSED_EVENT_ABI,
      logs: receipt.logs,
      eventName: "InvoiceProcessed",
    });

    this.log(
      `Invoice successfully submitted: transactionHash=${receipt.transactionHash} transactionConfirmationTimeInSeconds=${transactionConfirmationTime} eventName=${event.eventName} receiver=${event.args.receiver} startTimestamp=${event.args.startTimestamp} endTimestamp=${event.args.endTimestamp} amountPaid=${event.args.amountPaid} amountRequested=${event.args.amountRequested}`,
    );
  }

  /**
   * Fetch AWS costs historical data for the specified invoice period.
   * @param startDate Invoice period start date.
   * @param awsCostsApiFilters AWS Costs API filters to apply.
   */
  private async getAWSCostsHistoricalData(
    startDate: Date,
    awsCostsApiFilters: GetCostAndUsageCommandInput,
  ): Promise<void> {
    const currentDate = new Date();
    const awsClient = createAwsCostExplorerClient({ region: "us-east-1" });
    const startDateStr = formatInTimeZone(startDate, "UTC", "yyyy-MM-dd");
    const endDateStr = formatInTimeZone(currentDate, "UTC", "yyyy-MM-dd");
    const awsEndDateStr = formatInTimeZone(addDays(currentDate, 1), "UTC", "yyyy-MM-dd");

    this.log(`Fetching AWS costs historical data. startDate=${startDateStr} endDate=${endDateStr}`);

    if (!awsCostsApiFilters.Metrics || awsCostsApiFilters.Metrics.length !== 1) {
      this.error("AWS Costs API Filters must specify one metric.");
    }

    const { ResultsByTime } = this.unwrapOrError(
      await getDailyAwsCosts(awsClient, {
        ...awsCostsApiFilters,
        TimePeriod: {
          Start: startDateStr,
          End: awsEndDateStr,
        },
      }),
      "Failed to fetch AWS costs historical data",
    );

    if (!Array.isArray(ResultsByTime) || ResultsByTime.length === 0) {
      this.error(
        `No AWS cost historical data returned for the specified period. startDate=${startDateStr} endDate=${endDateStr}`,
      );
    }

    const formattedResultsByTime = flattenResultsByTime(ResultsByTime, awsCostsApiFilters.Metrics[0]);

    this.log(`AWS Historical Costs Data:`);
    for (const dailyData of formattedResultsByTime) {
      this.log(
        `type=aws_historical_data_costs fetchAt=${endDateStr} date=${dailyData.date} amount=${dailyData.amount} estimated=${dailyData.estimated}`,
      );
    }
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
