import { ILogger, WinstonLogger } from "@consensys/linea-shared-utils";
import { DataSource } from "typeorm";

import { PostmanConfig, L1NetworkConfig, L2NetworkConfig } from "./config/config";
import { L1ToL2App, L1ToL2Deps } from "./L1ToL2App";
import { L2ToL1App, L2ToL1Deps } from "./L2ToL1App";
import { Direction } from "../../../core/enums";
import { ISponsorshipMetricsUpdater, ITransactionMetricsUpdater } from "../../../core/metrics";
import { IPoller } from "../../../core/services/pollers/IPoller";
import { L2ClaimTransactionSizeCalculator } from "../../../infrastructure/blockchain/L2ClaimTransactionSizeCalculator";
import {
  ViemCalldataDecoder,
  ViemEthereumGasProvider,
  ViemL2MessageServiceClient,
  ViemLineaGasProvider,
  ViemLineaProvider,
  ViemLineaRollupClient,
  ViemLineaRollupLogClient,
  ViemL2MessageServiceLogClient,
  ViemProvider,
  ViemTransactionSigner,
  NonceManager,
  createChainContext,
  ViemTransactionRetrier,
  ViemReceiptPoller,
} from "../../../infrastructure/blockchain/viem";
import { ViemErrorParser } from "../../../infrastructure/blockchain/viem";
import { TypeOrmMessageRepository } from "../../../infrastructure/persistence/repositories/TypeOrmMessageRepository";
import { EthereumTransactionValidationService } from "../../../services/EthereumTransactionValidationService";
import { LineaTransactionValidationService } from "../../../services/LineaTransactionValidationService";
import { IntervalPoller } from "../../../services/pollers";
import { DatabaseCleanerProcessor } from "../../../services/processors/DatabaseCleanerProcessor";

export type PostmanServices = {
  l1ToL2App?: L1ToL2App;
  l2ToL1App?: L2ToL1App;
  databaseCleaningPoller?: IPoller;
};

type SharedInfrastructure = {
  l1: Awaited<ReturnType<typeof createChainContext>>;
  l2: Awaited<ReturnType<typeof createChainContext>>;
  messageRepository: TypeOrmMessageRepository;
  calldataDecoder: ViemCalldataDecoder;
  errorParser: ViemErrorParser;
  lineaRollupClient: ViemLineaRollupClient;
  l2MessageServiceClient: ViemL2MessageServiceClient;
  l1Provider: ViemProvider;
  l2Provider: ViemProvider;
  l1GasProvider: ViemEthereumGasProvider;
  l2GasProvider: ViemLineaGasProvider;
};

async function buildSharedInfrastructure(
  config: PostmanConfig,
  db: DataSource,
  logger: ILogger,
): Promise<SharedInfrastructure> {
  const { l1Config, l2Config, loggerOptions } = config;

  const [l1, l2] = await Promise.all([
    createChainContext(l1Config.rpcUrl, l1Config.claiming.signer, logger),
    createChainContext(l2Config.rpcUrl, l2Config.claiming.signer, logger),
  ]);

  const l1GasProvider = new ViemEthereumGasProvider(
    l1.publicClient,
    {
      maxFeePerGasCap: l1Config.claiming.maxFeePerGasCap,
      gasEstimationPercentile: l1Config.claiming.gasEstimationPercentile,
      enforceMaxGasFee: l1Config.claiming.isMaxGasFeeEnforced,
    },
    new WinstonLogger("L1GasProvider", loggerOptions),
  );
  const l2GasProvider = new ViemLineaGasProvider(
    l2.publicClient,
    {
      maxFeePerGasCap: l2Config.claiming.maxFeePerGasCap,
      gasEstimationPercentile: l2Config.claiming.gasEstimationPercentile,
      enforceMaxGasFee: l2Config.claiming.isMaxGasFeeEnforced,
      enableLineaEstimateGas: l2Config.enableLineaEstimateGas,
    },
    new WinstonLogger("L2GasProvider", loggerOptions),
  );

  const lineaRollupClient = new ViemLineaRollupClient(
    l1.publicClient,
    l1.walletClient,
    l1Config.messageServiceContractAddress,
    l2.publicClient,
    l2Config.messageServiceContractAddress,
    l1GasProvider,
  );
  const l2MessageServiceClient = new ViemL2MessageServiceClient(
    l2.publicClient,
    l2.walletClient,
    l2Config.messageServiceContractAddress,
    l2GasProvider,
    l2.account.address,
  );

  return {
    l1,
    l2,
    messageRepository: new TypeOrmMessageRepository(db),
    calldataDecoder: new ViemCalldataDecoder(new WinstonLogger("ViemCalldataDecoder", loggerOptions)),
    errorParser: new ViemErrorParser(),
    lineaRollupClient,
    l2MessageServiceClient,
    l1Provider: new ViemProvider(l1.publicClient, new WinstonLogger("L1Provider", loggerOptions)),
    l2Provider: new ViemProvider(l2.publicClient, new WinstonLogger("L2Provider", loggerOptions)),
    l1GasProvider,
    l2GasProvider,
  };
}

function buildL1ToL2Deps(
  infra: SharedInfrastructure,
  l1Config: L1NetworkConfig,
  l2Config: L2NetworkConfig,
  loggerOptions: PostmanConfig["loggerOptions"],
  sponsorshipMetricsUpdater: ISponsorshipMetricsUpdater,
  transactionMetricsUpdater: ITransactionMetricsUpdater,
): { deps: L1ToL2Deps; initialize: () => Promise<void> } {
  const { l1, l2, messageRepository, calldataDecoder, errorParser, l2MessageServiceClient, l1Provider, l2Provider } =
    infra;

  const l2SignerAddress = l2.account.address;

  const l2NonceManager = new NonceManager(
    l2Provider,
    { getMaxPendingNonce: () => messageRepository.getMaxPendingNonce(Direction.L1_TO_L2) },
    l2SignerAddress,
    l2Config.claiming.maxNonceDiff,
    new WinstonLogger("L2NonceManager", loggerOptions),
  );

  const l2TransactionRetrier = new ViemTransactionRetrier(
    l2.publicClient,
    l2.walletClient,
    l2.account,
    l2Config.claiming.maxFeePerGasCap,
    new WinstonLogger("L2TransactionRetrier", loggerOptions),
  );
  const l2ReceiptPoller = new ViemReceiptPoller(l2Provider, new WinstonLogger("L2ReceiptPoller", loggerOptions));
  const transactionSigner = new ViemTransactionSigner(l2.signer, l2.chainId);

  const transactionValidationService = new LineaTransactionValidationService(
    {
      profitMargin: l2Config.claiming.profitMargin,
      maxClaimGasLimit: l2Config.claiming.maxClaimGasLimit,
      isPostmanSponsorshipEnabled: l2Config.claiming.isPostmanSponsorshipEnabled,
      maxPostmanSponsorGasLimit: l2Config.claiming.maxPostmanSponsorGasLimit,
    },
    new ViemLineaProvider(l2.publicClient, new WinstonLogger("L2LineaProvider", loggerOptions)),
    l2MessageServiceClient,
    new WinstonLogger("LineaTransactionValidationService", loggerOptions),
  );

  const transactionSizeCalculator = new L2ClaimTransactionSizeCalculator(l2MessageServiceClient, transactionSigner);

  const deps: L1ToL2Deps = {
    l1LogClient: new ViemLineaRollupLogClient(l1.publicClient, l1Config.messageServiceContractAddress),
    l1Provider,
    l2MessageServiceClient,
    l2Provider: new ViemLineaProvider(l2.publicClient, new WinstonLogger("L2LineaProvider", loggerOptions)),
    l2NonceManager,
    l2TransactionRetrier,
    l2ReceiptPoller,
    messageRepository,
    calldataDecoder,
    transactionValidationService,
    transactionSizeCalculator,
    sponsorshipMetricsUpdater,
    transactionMetricsUpdater,
    errorParser,
    l1Config,
    l2Config,
    loggerOptions,
  };

  return { deps, initialize: () => l2NonceManager.initialize() };
}

function buildL2ToL1Deps(
  infra: SharedInfrastructure,
  l1Config: L1NetworkConfig,
  l2Config: L2NetworkConfig,
  loggerOptions: PostmanConfig["loggerOptions"],
  sponsorshipMetricsUpdater: ISponsorshipMetricsUpdater,
  transactionMetricsUpdater: ITransactionMetricsUpdater,
): { deps: L2ToL1Deps; initialize: () => Promise<void> } {
  const {
    l1,
    l2,
    messageRepository,
    calldataDecoder,
    errorParser,
    lineaRollupClient,
    l1Provider,
    l2Provider,
    l1GasProvider,
  } = infra;

  const l1SignerAddress = l1.account.address;

  const l1NonceManager = new NonceManager(
    l1Provider,
    { getMaxPendingNonce: () => messageRepository.getMaxPendingNonce(Direction.L2_TO_L1) },
    l1SignerAddress,
    l1Config.claiming.maxNonceDiff,
    new WinstonLogger("L1NonceManager", loggerOptions),
  );

  const l1TransactionRetrier = new ViemTransactionRetrier(
    l1.publicClient,
    l1.walletClient,
    l1.account,
    l1Config.claiming.maxFeePerGasCap,
    new WinstonLogger("L1TransactionRetrier", loggerOptions),
  );
  const l1ReceiptPoller = new ViemReceiptPoller(l1Provider, new WinstonLogger("L1ReceiptPoller", loggerOptions));

  const transactionValidationService = new EthereumTransactionValidationService(
    lineaRollupClient,
    l1GasProvider,
    {
      profitMargin: l1Config.claiming.profitMargin,
      maxClaimGasLimit: l1Config.claiming.maxClaimGasLimit,
      isPostmanSponsorshipEnabled: l1Config.claiming.isPostmanSponsorshipEnabled,
      maxPostmanSponsorGasLimit: l1Config.claiming.maxPostmanSponsorGasLimit,
    },
    new WinstonLogger("EthereumTransactionValidationService", loggerOptions),
  );

  const deps: L2ToL1Deps = {
    l2LogClient: new ViemL2MessageServiceLogClient(l2.publicClient, l2Config.messageServiceContractAddress),
    l2Provider,
    lineaRollupClient,
    l1Provider,
    l1NonceManager,
    l1TransactionRetrier,
    l1ReceiptPoller,
    messageRepository,
    l1GasProvider,
    transactionValidationService,
    calldataDecoder,
    sponsorshipMetricsUpdater,
    transactionMetricsUpdater,
    errorParser,
    l1Config,
    l2Config,
    loggerOptions,
  };

  return { deps, initialize: () => l1NonceManager.initialize() };
}

function buildDatabaseCleaningPoller(
  config: PostmanConfig,
  messageRepository: TypeOrmMessageRepository,
  loggerOptions: PostmanConfig["loggerOptions"],
): IPoller {
  const databaseCleanerProcessor = new DatabaseCleanerProcessor(
    messageRepository,
    { daysBeforeNowToDelete: config.databaseCleanerConfig.daysBeforeNowToDelete },
    new WinstonLogger(DatabaseCleanerProcessor.name, loggerOptions),
  );

  return new IntervalPoller(
    databaseCleanerProcessor,
    { pollingInterval: config.databaseCleanerConfig.cleaningInterval },
    new WinstonLogger("DatabaseCleaningPoller", loggerOptions),
  );
}

export async function buildPostmanServices(
  config: PostmanConfig,
  db: DataSource,
  sponsorshipMetricsUpdater: ISponsorshipMetricsUpdater,
  transactionMetricsUpdater: ITransactionMetricsUpdater,
  logger: ILogger,
): Promise<PostmanServices> {
  const { l1Config, l2Config, loggerOptions } = config;
  const infra = await buildSharedInfrastructure(config, db, logger);
  const services: PostmanServices = {};

  if (config.l1L2AutoClaimEnabled) {
    const { deps, initialize } = buildL1ToL2Deps(
      infra,
      l1Config,
      l2Config,
      loggerOptions,
      sponsorshipMetricsUpdater,
      transactionMetricsUpdater,
    );
    await initialize();
    services.l1ToL2App = new L1ToL2App(deps);
  }

  if (config.l2L1AutoClaimEnabled) {
    const { deps, initialize } = buildL2ToL1Deps(
      infra,
      l1Config,
      l2Config,
      loggerOptions,
      sponsorshipMetricsUpdater,
      transactionMetricsUpdater,
    );
    await initialize();
    services.l2ToL1App = new L2ToL1App(deps);
  }

  if (config.databaseCleanerConfig.enabled) {
    services.databaseCleaningPoller = buildDatabaseCleaningPoller(config, infra.messageRepository, loggerOptions);
  }

  return services;
}
