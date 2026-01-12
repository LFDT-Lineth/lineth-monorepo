import { DataSource } from "typeorm";
import { LineaSDK, Direction } from "@consensys/linea-sdk";
import { ExpressApiApplication, ILogger } from "@consensys/linea-shared-utils";
import { TypeOrmMessageRepository } from "../persistence/repositories/TypeOrmMessageRepository";
import { IPoller } from "../../../core/services/pollers/IPoller";
import {
  MessageAnchoringProcessor,
  MessageClaimingProcessor,
  MessageClaimingPersister,
  MessageSentEventProcessor,
  L2ClaimMessageTransactionSizeProcessor,
} from "../../../services/processors";
import { PostmanConfig, PostmanOptions } from "./config/config";
import { DB } from "../persistence/dataSource";
import {
  MessageSentEventPoller,
  MessageAnchoringPoller,
  MessageClaimingPoller,
  MessagePersistingPoller,
  DatabaseCleaningPoller,
  L2ClaimMessageTransactionSizePoller,
} from "../../../services/pollers";
import { DatabaseCleaner, LineaMessageDBService, EthereumMessageDBService } from "../../../services/persistence";
import { L2ClaimTransactionSizeCalculator } from "../../../services/L2ClaimTransactionSizeCalculator";
import { LineaTransactionValidationService } from "../../../services/LineaTransactionValidationService";
import { EthereumTransactionValidationService } from "../../../services/EthereumTransactionValidationService";
import { getConfig } from "./config/utils";
import { MessageStatusSubscriber } from "../persistence/subscribers/MessageStatusSubscriber";
import { PostmanWinstonLogger } from "../../../utils/PostmanWinstonLogger";
import { PostmanMetricsService } from "../api/metrics/PostmanMetricsService";
import { MessageMetricsUpdater } from "../api/metrics/MessageMetricsUpdater";
import {
  IMessageMetricsUpdater,
  ISponsorshipMetricsUpdater,
  ITransactionMetricsUpdater,
  LineaPostmanMetrics,
} from "../../../../src/core/metrics";
import { SponsorshipMetricsUpdater } from "../api/metrics/SponsorshipMetricsUpdater";
import { TransactionMetricsUpdater } from "../api/metrics/TransactionMetricsUpdater";
import { IMetricsService, IApplication } from "@consensys/linea-shared-utils";

export class PostmanServiceClient {
  // Metrics services
  private postmanMetricsService: IMetricsService<LineaPostmanMetrics>;
  private messageMetricsUpdater: IMessageMetricsUpdater;
  private sponsorshipMetricsUpdater: ISponsorshipMetricsUpdater;
  private transactionMetricsUpdater: ITransactionMetricsUpdater;

  // L1 -> L2 flow
  private l1MessageSentEventPoller: IPoller;
  private l2MessageAnchoringPoller: IPoller;
  private l2MessageClaimingPoller: IPoller;
  private l2MessagePersistingPoller: IPoller;
  private l2ClaimMessageTransactionSizePoller: IPoller;

  // L2 -> L1 flow
  private l2MessageSentEventPoller: IPoller;
  private l1MessageAnchoringPoller: IPoller;
  private l1MessageClaimingPoller: IPoller;
  private l1MessagePersistingPoller: IPoller;

  // Database Cleaner
  private databaseCleaningPoller: IPoller;

  private logger: ILogger;
  private db: DataSource;

  private l1L2AutoClaimEnabled: boolean;
  private l2L1AutoClaimEnabled: boolean;
  private api: IApplication;
  private config: PostmanConfig;

  /**
   * Initializes a new instance of the PostmanServiceClient.
   *
   * @param {PostmanOptions} options - Configuration options for the Postman service, including network details, database options, and logging configurations.
   */
  constructor(options: PostmanOptions) {
    const config = getConfig(options);
    this.config = config;

    this.logger = new PostmanWinstonLogger(PostmanServiceClient.name, config.loggerOptions);
    this.l1L2AutoClaimEnabled = config.l1L2AutoClaimEnabled;
    this.l2L1AutoClaimEnabled = config.l2L1AutoClaimEnabled;

    const lineaSdk = new LineaSDK({
      l1RpcUrlOrProvider: config.l1Config.rpcUrl,
      l2RpcUrlOrProvider: config.l2Config.rpcUrl,
      l1SignerPrivateKeyOrWallet: config.l1Config.claiming.signerPrivateKey,
      l2SignerPrivateKeyOrWallet: config.l2Config.claiming.signerPrivateKey,
      network: "custom",
      mode: "read-write",
      l1FeeEstimatorOptions: {
        gasFeeEstimationPercentile: config.l1Config.claiming.gasEstimationPercentile,
        maxFeePerGasCap: config.l1Config.claiming.maxFeePerGasCap,
        enforceMaxGasFee: config.l1Config.claiming.isMaxGasFeeEnforced,
      },
      l2FeeEstimatorOptions: {
        gasFeeEstimationPercentile: config.l2Config.claiming.gasEstimationPercentile,
        maxFeePerGasCap: config.l2Config.claiming.maxFeePerGasCap,
        enforceMaxGasFee: config.l2Config.claiming.isMaxGasFeeEnforced,
        enableLineaEstimateGas: config.l2Config.enableLineaEstimateGas,
      },
    });

    const l1Provider = lineaSdk.getL1Provider(config.l1Config.rpcUrl);
    const l2Provider = lineaSdk.getL2Provider(config.l2Config.rpcUrl);

    const l1Signer = lineaSdk.getL1Signer();
    const l2Signer = lineaSdk.getL2Signer();

    const lineaRollupClient = lineaSdk.getL1Contract(
      config.l1Config.messageServiceContractAddress,
      config.l2Config.messageServiceContractAddress,
    );

    const l2MessageServiceClient = lineaSdk.getL2Contract(config.l2Config.messageServiceContractAddress);

    const lineaRollupLogClient = lineaSdk.getL1ContractEventLogClient(config.l1Config.messageServiceContractAddress);
    const l2MessageServiceLogClient = lineaSdk.getL2ContractEventLogClient(
      config.l2Config.messageServiceContractAddress,
    );

    const l1GasProvider = lineaSdk.getL1GasProvider();

    this.db = DB.create(config.databaseOptions);

    const messageRepository = new TypeOrmMessageRepository(this.db);
    const lineaMessageDBService = new LineaMessageDBService(messageRepository);
    const ethereumMessageDBService = new EthereumMessageDBService(l1GasProvider, messageRepository);

    // Metrics services
    this.postmanMetricsService = new PostmanMetricsService();
    this.messageMetricsUpdater = new MessageMetricsUpdater(this.db.manager, this.postmanMetricsService);
    this.sponsorshipMetricsUpdater = new SponsorshipMetricsUpdater(this.postmanMetricsService);
    this.transactionMetricsUpdater = new TransactionMetricsUpdater(this.postmanMetricsService);

    // L1 -> L2 flow

    const l1MessageSentEventProcessor = new MessageSentEventProcessor(
      lineaMessageDBService,
      lineaRollupLogClient,
      l1Provider,
      {
        direction: Direction.L1_TO_L2,
        maxBlocksToFetchLogs: config.l1Config.listener.maxBlocksToFetchLogs,
        blockConfirmation: config.l1Config.listener.blockConfirmation,
        isEOAEnabled: config.l1Config.isEOAEnabled,
        isCalldataEnabled: config.l1Config.isCalldataEnabled,
        eventFilters: config.l1Config.listener.eventFilters,
      },
      new PostmanWinstonLogger(`L1${MessageSentEventProcessor.name}`, config.loggerOptions),
    );

    this.l1MessageSentEventPoller = new MessageSentEventPoller(
      l1MessageSentEventProcessor,
      l1Provider,
      lineaMessageDBService,
      {
        direction: Direction.L1_TO_L2,
        pollingInterval: config.l1Config.listener.pollingInterval,
        initialFromBlock: config.l1Config.listener.initialFromBlock,
        originContractAddress: config.l1Config.messageServiceContractAddress,
      },
      new PostmanWinstonLogger(`L1${MessageSentEventPoller.name}`, config.loggerOptions),
    );

    const l2MessageAnchoringProcessor = new MessageAnchoringProcessor(
      l2MessageServiceClient,
      l2Provider,
      lineaMessageDBService,
      {
        maxFetchMessagesFromDb: config.l1Config.listener.maxFetchMessagesFromDb,
        originContractAddress: config.l1Config.messageServiceContractAddress,
      },
      new PostmanWinstonLogger(`L2${MessageAnchoringProcessor.name}`, config.loggerOptions),
    );

    this.l2MessageAnchoringPoller = new MessageAnchoringPoller(
      l2MessageAnchoringProcessor,
      {
        direction: Direction.L1_TO_L2,
        pollingInterval: config.l2Config.listener.pollingInterval,
      },
      new PostmanWinstonLogger(`L2${MessageAnchoringPoller.name}`, config.loggerOptions),
    );

    const l2TransactionValidationService = new LineaTransactionValidationService(
      {
        profitMargin: config.l2Config.claiming.profitMargin,
        maxClaimGasLimit: BigInt(config.l2Config.claiming.maxClaimGasLimit),
        isPostmanSponsorshipEnabled: config.l2Config.claiming.isPostmanSponsorshipEnabled,
        maxPostmanSponsorGasLimit: config.l2Config.claiming.maxPostmanSponsorGasLimit,
      },
      l2Provider,
      l2MessageServiceClient,
      new PostmanWinstonLogger(`${LineaTransactionValidationService.name}`, config.loggerOptions),
    );

    const l2MessageClaimingProcessor = new MessageClaimingProcessor(
      l2MessageServiceClient,
      l2Signer,
      lineaMessageDBService,
      l2TransactionValidationService,
      {
        direction: Direction.L1_TO_L2,
        originContractAddress: config.l1Config.messageServiceContractAddress,
        maxNonceDiff: config.l2Config.claiming.maxNonceDiff,
        feeRecipientAddress: config.l2Config.claiming.feeRecipientAddress,
        profitMargin: config.l2Config.claiming.profitMargin,
        maxNumberOfRetries: config.l2Config.claiming.maxNumberOfRetries,
        retryDelayInSeconds: config.l2Config.claiming.retryDelayInSeconds,
        maxClaimGasLimit: BigInt(config.l2Config.claiming.maxClaimGasLimit),
        claimViaAddress: config.l2Config.claiming.claimViaAddress,
      },
      new PostmanWinstonLogger(`L2${MessageClaimingProcessor.name}`, config.loggerOptions),
    );

    this.l2MessageClaimingPoller = new MessageClaimingPoller(
      l2MessageClaimingProcessor,
      {
        direction: Direction.L1_TO_L2,
        pollingInterval: config.l2Config.listener.pollingInterval,
      },
      new PostmanWinstonLogger(`L2${MessageClaimingPoller.name}`, config.loggerOptions),
    );

    const l2MessageClaimingPersister = new MessageClaimingPersister(
      lineaMessageDBService,
      l2MessageServiceClient,
      this.sponsorshipMetricsUpdater,
      this.transactionMetricsUpdater,
      l2Provider,
      {
        direction: Direction.L1_TO_L2,
        messageSubmissionTimeout: config.l2Config.claiming.messageSubmissionTimeout,
        maxTxRetries: config.l2Config.claiming.maxTxRetries,
      },
      new PostmanWinstonLogger(`L2${MessageClaimingPersister.name}`, config.loggerOptions),
    );

    this.l2MessagePersistingPoller = new MessagePersistingPoller(
      l2MessageClaimingPersister,
      {
        direction: Direction.L1_TO_L2,
        pollingInterval: config.l2Config.listener.receiptPollingInterval,
      },
      new PostmanWinstonLogger(`L2${MessagePersistingPoller.name}`, config.loggerOptions),
    );

    const transactionSizeCalculator = new L2ClaimTransactionSizeCalculator(l2MessageServiceClient);
    const transactionSizeCompressor = new L2ClaimMessageTransactionSizeProcessor(
      lineaMessageDBService,
      l2MessageServiceClient,
      transactionSizeCalculator,
      {
        direction: Direction.L1_TO_L2,
        originContractAddress: config.l1Config.messageServiceContractAddress,
      },
      new PostmanWinstonLogger(`${L2ClaimMessageTransactionSizeProcessor.name}`, config.loggerOptions),
    );

    this.l2ClaimMessageTransactionSizePoller = new L2ClaimMessageTransactionSizePoller(
      transactionSizeCompressor,
      {
        pollingInterval: config.l2Config.listener.pollingInterval,
      },
      new PostmanWinstonLogger(`${L2ClaimMessageTransactionSizePoller.name}`, config.loggerOptions),
    );

    // L2 -> L1 flow
    const l2MessageSentEventProcessor = new MessageSentEventProcessor(
      ethereumMessageDBService,
      l2MessageServiceLogClient,
      l2Provider,
      {
        direction: Direction.L2_TO_L1,
        maxBlocksToFetchLogs: config.l2Config.listener.maxBlocksToFetchLogs,
        blockConfirmation: config.l2Config.listener.blockConfirmation,
        isEOAEnabled: config.l2Config.isEOAEnabled,
        isCalldataEnabled: config.l2Config.isCalldataEnabled,
        eventFilters: config.l2Config.listener.eventFilters,
      },
      new PostmanWinstonLogger(`L2${MessageSentEventProcessor.name}`, config.loggerOptions),
    );

    this.l2MessageSentEventPoller = new MessageSentEventPoller(
      l2MessageSentEventProcessor,
      l2Provider,
      ethereumMessageDBService,
      {
        direction: Direction.L2_TO_L1,
        pollingInterval: config.l2Config.listener.pollingInterval,
        initialFromBlock: config.l2Config.listener.initialFromBlock,
        originContractAddress: config.l2Config.messageServiceContractAddress,
      },
      new PostmanWinstonLogger(`L2${MessageSentEventPoller.name}`, config.loggerOptions),
    );

    const l1MessageAnchoringProcessor = new MessageAnchoringProcessor(
      lineaRollupClient,
      l1Provider,
      ethereumMessageDBService,
      {
        maxFetchMessagesFromDb: config.l1Config.listener.maxFetchMessagesFromDb,
        originContractAddress: config.l2Config.messageServiceContractAddress,
      },
      new PostmanWinstonLogger(`L1${MessageAnchoringProcessor.name}`, config.loggerOptions),
    );

    this.l1MessageAnchoringPoller = new MessageAnchoringPoller(
      l1MessageAnchoringProcessor,
      {
        direction: Direction.L2_TO_L1,
        pollingInterval: config.l1Config.listener.pollingInterval,
      },
      new PostmanWinstonLogger(`L1${MessageAnchoringPoller.name}`, config.loggerOptions),
    );

    const l1TransactionValidationService = new EthereumTransactionValidationService(
      lineaRollupClient,
      l1GasProvider,
      {
        profitMargin: config.l1Config.claiming.profitMargin,
        maxClaimGasLimit: BigInt(config.l1Config.claiming.maxClaimGasLimit),
        isPostmanSponsorshipEnabled: config.l1Config.claiming.isPostmanSponsorshipEnabled,
        maxPostmanSponsorGasLimit: config.l1Config.claiming.maxPostmanSponsorGasLimit,
      },
      new PostmanWinstonLogger(`${EthereumTransactionValidationService.name}`, config.loggerOptions),
    );

    const l1MessageClaimingProcessor = new MessageClaimingProcessor(
      lineaRollupClient,
      l1Signer,
      ethereumMessageDBService,
      l1TransactionValidationService,
      {
        direction: Direction.L2_TO_L1,
        originContractAddress: config.l2Config.messageServiceContractAddress,
        maxNonceDiff: config.l1Config.claiming.maxNonceDiff,
        feeRecipientAddress: config.l1Config.claiming.feeRecipientAddress,
        profitMargin: config.l1Config.claiming.profitMargin,
        maxNumberOfRetries: config.l1Config.claiming.maxNumberOfRetries,
        retryDelayInSeconds: config.l1Config.claiming.retryDelayInSeconds,
        maxClaimGasLimit: BigInt(config.l1Config.claiming.maxClaimGasLimit),
        claimViaAddress: config.l1Config.claiming.claimViaAddress,
      },
      new PostmanWinstonLogger(`L1${MessageClaimingProcessor.name}`, config.loggerOptions),
    );

    this.l1MessageClaimingPoller = new MessageClaimingPoller(
      l1MessageClaimingProcessor,
      {
        direction: Direction.L2_TO_L1,
        pollingInterval: config.l1Config.listener.pollingInterval,
      },
      new PostmanWinstonLogger(`L1${MessageClaimingPoller.name}`, config.loggerOptions),
    );

    const l1MessageClaimingPersister = new MessageClaimingPersister(
      ethereumMessageDBService,
      lineaRollupClient,
      this.sponsorshipMetricsUpdater,
      this.transactionMetricsUpdater,
      l1Provider,
      {
        direction: Direction.L2_TO_L1,
        messageSubmissionTimeout: config.l1Config.claiming.messageSubmissionTimeout,
        maxTxRetries: config.l1Config.claiming.maxTxRetries,
      },
      new PostmanWinstonLogger(`L1${MessageClaimingPersister.name}`, config.loggerOptions),
    );

    this.l1MessagePersistingPoller = new MessagePersistingPoller(
      l1MessageClaimingPersister,
      {
        direction: Direction.L2_TO_L1,
        pollingInterval: config.l1Config.listener.receiptPollingInterval,
      },
      new PostmanWinstonLogger(`L1${MessagePersistingPoller.name}`, config.loggerOptions),
    );

    // Database Cleaner
    const databaseCleaner = new DatabaseCleaner(
      ethereumMessageDBService,
      new PostmanWinstonLogger(`${DatabaseCleaner.name}`, config.loggerOptions),
    );

    this.databaseCleaningPoller = new DatabaseCleaningPoller(
      databaseCleaner,
      new PostmanWinstonLogger(`${DatabaseCleaningPoller.name}`, config.loggerOptions),
      {
        enabled: config.databaseCleanerConfig.enabled,
        daysBeforeNowToDelete: config.databaseCleanerConfig.daysBeforeNowToDelete,
        cleaningInterval: config.databaseCleanerConfig.cleaningInterval,
      },
    );
  }

  /**
   * Initializes the database connection.
   */
  public async initializeDatabase(): Promise<void> {
    try {
      await this.db.initialize();
      this.logger.info("Database connection established successfully.");
    } catch (error) {
      this.logger.error("Failed to connect to the database.", error);
      throw error;
    }
  }

  public async initializeMetrics(): Promise<void> {}

  /**
   * Initializes metrics, registers subscribers, and configures the API.
   * This method expects the database to be connected.
   */
  public async initializeMetricsAndApi(): Promise<void> {
    try {
      await this.messageMetricsUpdater.initialize();
      const messageStatusSubscriber = new MessageStatusSubscriber(
        this.messageMetricsUpdater,
        new PostmanWinstonLogger(MessageStatusSubscriber.name),
      );
      this.db.subscribers.push(messageStatusSubscriber);

      // Initialize or reinitialize the API using the metrics service.
      this.api = new ExpressApiApplication(
        this.config.apiConfig.port,
        this.postmanMetricsService,
        new PostmanWinstonLogger(ExpressApiApplication.name),
      );

      this.logger.info("Metrics and API have been initialized successfully.");
    } catch (error) {
      this.logger.error("Failed to initialize metrics or API.", error);
      throw error;
    }
  }

  /**
   * Connects services by first initializing the database and then setting up metrics and the API.
   */
  public async connectServices(): Promise<void> {
    // Database initialization must happen before metrics initialization
    await this.initializeDatabase();
    await this.initializeMetricsAndApi();
  }

  /**
   * Starts all configured services and pollers. This includes message event pollers for both L1 to L2 and L2 to L1 flows, message anchoring, claiming, persisting pollers, and the database cleaning poller.
   */
  public startAllServices(): void {
    if (this.l1L2AutoClaimEnabled) {
      // L1 -> L2 flow
      this.l1MessageSentEventPoller.start();
      this.l2MessageAnchoringPoller.start();
      this.l2MessageClaimingPoller.start();
      this.l2MessagePersistingPoller.start();
      this.l2ClaimMessageTransactionSizePoller.start();
    }

    if (this.l2L1AutoClaimEnabled) {
      // L2 -> L1 flow
      this.l2MessageSentEventPoller.start();
      this.l1MessageAnchoringPoller.start();
      this.l1MessageClaimingPoller.start();
      this.l1MessagePersistingPoller.start();
    }

    // Database Cleaner
    this.databaseCleaningPoller.start();

    this.api.start();

    this.logger.info("All listeners and message deliverers have been started.");
  }

  /**
   * Stops all running services and pollers to gracefully shut down the Postman service.
   */
  public stopAllServices(): void {
    if (this.l1L2AutoClaimEnabled) {
      // L1 -> L2 flow
      this.l1MessageSentEventPoller.stop();
      this.l2MessageAnchoringPoller.stop();
      this.l2MessageClaimingPoller.stop();
      this.l2MessagePersistingPoller.stop();
      this.l2ClaimMessageTransactionSizePoller.stop();
    }

    if (this.l2L1AutoClaimEnabled) {
      // L2 -> L1 flow
      this.l2MessageSentEventPoller.stop();
      this.l1MessageAnchoringPoller.stop();
      this.l1MessageClaimingPoller.stop();
      this.l1MessagePersistingPoller.stop();
    }

    // Database Cleaner
    this.databaseCleaningPoller.stop();

    this.api.stop();

    this.logger.info("All listeners and message deliverers have been stopped.");
  }
}
