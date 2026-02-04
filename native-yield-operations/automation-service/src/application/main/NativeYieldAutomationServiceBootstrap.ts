import {
  ExponentialBackoffRetryService,
  ExpressApiApplication,
  IApplication,
  ILogger,
  IMetricsService,
  IRetryService,
  WinstonLogger,
} from "@consensys/linea-shared-utils";
import { NativeYieldAutomationServiceBootstrapConfig } from "./config/config.js";
import { IOperationLoop } from "../../services/IOperationLoop.js";
import { OperationModeSelector } from "../../services/OperationModeSelector.js";
import {
  IBlockchainClient,
  ViemBlockchainClientAdapter,
  Web3SignerClientAdapter,
  IContractSignerClient,
  IOAuth2TokenClient,
  IBeaconNodeAPIClient,
  BeaconNodeApiClient,
  OAuth2TokenClient,
} from "@consensys/linea-shared-utils";
import { Chain, PublicClient, TransactionReceipt } from "viem";
import { YieldManagerContractClient } from "../../clients/contracts/YieldManagerContractClient.js";
import { IYieldManager } from "../../core/clients/contracts/IYieldManager.js";
import { YieldReportingProcessor } from "../../services/operation-mode-processors/YieldReportingProcessor.js";
import { LazyOracleContractClient } from "../../clients/contracts/LazyOracleContractClient.js";
import { ILazyOracle } from "../../core/clients/contracts/ILazyOracle.js";
import { ApolloClient } from "@apollo/client";
import { ILineaRollupYieldExtension } from "../../core/clients/contracts/ILineaRollupYieldExtension.js";
import { LineaRollupYieldExtensionContractClient } from "../../clients/contracts/LineaRollupYieldExtensionContractClient.js";
import { IOperationModeProcessor } from "../../core/services/operation-mode/IOperationModeProcessor.js";
import { ILidoAccountingReportClient } from "../../core/clients/ILidoAccountingReportClient.js";
import { IBeaconChainStakingClient } from "../../core/clients/IBeaconChainStakingClient.js";
import { IValidatorDataClient } from "../../core/clients/IValidatorDataClient.js";
import { ConsensysStakingApiClient } from "../../clients/ConsensysStakingApiClient.js";
import { LidoAccountingReportClient } from "../../clients/LidoAccountingReportClient.js";
import { BeaconChainStakingClient } from "../../clients/BeaconChainStakingClient.js";
import { OssificationCompleteProcessor } from "../../services/operation-mode-processors/OssificationCompleteProcessor.js";
import { OssificationPendingProcessor } from "../../services/operation-mode-processors/OssificationPendingProcessor.js";
import { mainnet, hoodi } from "viem/chains";
import { createApolloClient } from "../../utils/createApolloClient.js";
import { LineaNativeYieldAutomationServiceMetrics } from "../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";
import { NativeYieldAutomationMetricsService } from "../metrics/NativeYieldAutomationMetricsService.js";
import { NativeYieldAutomationMetricsUpdater } from "../metrics/NativeYieldAutomationMetricsUpdater.js";
import { INativeYieldAutomationMetricsUpdater } from "../../core/metrics/INativeYieldAutomationMetricsUpdater.js";
import { IVaultHub } from "../../core/clients/contracts/IVaultHub.js";
import { VaultHubContractClient } from "../../clients/contracts/VaultHubContractClient.js";
import { IOperationModeMetricsRecorder } from "../../core/metrics/IOperationModeMetricsRecorder.js";
import { OperationModeMetricsRecorder } from "../metrics/OperationModeMetricsRecorder.js";
import { DashboardContractClient } from "../../clients/contracts/DashboardContractClient.js";
import { StakingVaultContractClient } from "../../clients/contracts/StakingVaultContractClient.js";
import { STETHContractClient } from "../../clients/contracts/STETHContractClient.js";
import { ISTETH } from "../../core/clients/contracts/ISTETH.js";
import { GaugeMetricsPoller } from "../../services/GaugeMetricsPoller.js";
import { EstimateGasErrorReporter } from "../../core/services/EstimateGasErrorReporter.js";
import { RebalanceQuotaService } from "../../services/RebalanceQuotaService.js";
import { RebalanceDirection } from "../../core/entities/RebalanceRequirement.js";

/**
 * Bootstrap class for the Native Yield Automation Service.
 * Initializes and configures all service dependencies including observability (logging and metrics),
 * blockchain clients, contract clients, API clients, and operation mode processors.
 * Manages the lifecycle of all services (start/stop) and provides access to configuration.
 */
export class NativeYieldAutomationServiceBootstrap {
  private readonly config: NativeYieldAutomationServiceBootstrapConfig;
  private readonly logger: ILogger;
  private readonly metricsService: IMetricsService<LineaNativeYieldAutomationServiceMetrics>;
  private readonly metricsUpdater: INativeYieldAutomationMetricsUpdater;
  private readonly api: IApplication;

  private viemBlockchainClientAdapter: IBlockchainClient<PublicClient, TransactionReceipt>;
  private web3SignerClient: IContractSignerClient;
  private yieldManagerContractClient: IYieldManager<TransactionReceipt>;
  private lazyOracleContractClient: ILazyOracle<TransactionReceipt>;
  private vaultHubContractClient: IVaultHub<TransactionReceipt>;
  private lineaRollupYieldExtensionContractClient: ILineaRollupYieldExtension<TransactionReceipt>;
  private stethContractClient: ISTETH;

  private exponentialBackoffRetryService: IRetryService;
  private beaconNodeApiClient: IBeaconNodeAPIClient;
  private oAuth2TokenClient: IOAuth2TokenClient;
  private apolloClient: ApolloClient;
  private beaconChainStakingClient: IBeaconChainStakingClient;
  private lidoAccountingReportClient: ILidoAccountingReportClient;
  private consensysStakingGraphQLClient: IValidatorDataClient;

  private readonly operationModeMetricsRecorder: IOperationModeMetricsRecorder;
  private gaugeMetricsPoller: IOperationLoop;
  private operationModeSelector: IOperationLoop;
  private yieldReportingOperationModeProcessor: IOperationModeProcessor;
  private ossificationPendingOperationModeProcessor: IOperationModeProcessor;
  private ossificationCompleteOperationModeProcessor: IOperationModeProcessor;

  /**
   * Creates a new NativeYieldAutomationServiceBootstrap instance.
   * Initializes all service dependencies in the following order:
   * 1. Observability - logging and metrics
   * 2. Clients - blockchain, contract, and API clients
   * 3. Processor Services - operation mode processors and selector
   *
   * @param {NativeYieldAutomationServiceBootstrapConfig} config - Configuration object containing all service settings.
   */
  constructor(config: NativeYieldAutomationServiceBootstrapConfig) {
    this.config = config;

    // Observability - logging and metrics
    this.logger = new WinstonLogger(NativeYieldAutomationServiceBootstrap.name, config.loggerOptions);
    this.metricsService = new NativeYieldAutomationMetricsService();
    this.metricsUpdater = new NativeYieldAutomationMetricsUpdater(this.metricsService);
    this.api = new ExpressApiApplication(
      this.config.apiPort,
      this.metricsService,
      new WinstonLogger(ExpressApiApplication.name),
    );

    // Clients
    this.web3SignerClient = new Web3SignerClientAdapter(
      new WinstonLogger(Web3SignerClientAdapter.name, config.loggerOptions),
      config.web3signer.url,
      config.web3signer.publicKey,
      config.web3signer.keystore.path,
      config.web3signer.keystore.passphrase,
      config.web3signer.truststore.path,
      config.web3signer.truststore.passphrase,
    );

    const getChain = (chainId: number): Chain => {
      switch (chainId) {
        case mainnet.id:
          return mainnet;
        case hoodi.id:
          return hoodi;
        default:
          throw new Error(`Unsupported chain ID: ${chainId}`);
      }
    };
    const estimateGasErrorReporter = new EstimateGasErrorReporter(this.metricsUpdater);
    this.viemBlockchainClientAdapter = new ViemBlockchainClientAdapter(
      new WinstonLogger(ViemBlockchainClientAdapter.name, config.loggerOptions),
      config.dataSources.l1RpcUrl,
      getChain(config.dataSources.chainId),
      this.web3SignerClient,
      estimateGasErrorReporter,
    );
    DashboardContractClient.initialize(
      this.viemBlockchainClientAdapter,
      new WinstonLogger(DashboardContractClient.name, config.loggerOptions),
    );
    StakingVaultContractClient.initialize(
      this.viemBlockchainClientAdapter,
      new WinstonLogger(StakingVaultContractClient.name, config.loggerOptions),
    );
    const rebalanceQuotaService = new RebalanceQuotaService(
      new WinstonLogger(RebalanceQuotaService.name, config.loggerOptions),
      this.metricsUpdater,
      RebalanceDirection.STAKE,
      config.rebalance.stakingRebalanceQuotaWindowSizeInCycles,
      config.rebalance.stakingRebalanceQuotaBps,
      config.rebalance.toleranceAmountWei,
    );
    this.yieldManagerContractClient = new YieldManagerContractClient(
      new WinstonLogger(YieldManagerContractClient.name, config.loggerOptions),
      this.viemBlockchainClientAdapter,
      config.contractAddresses.yieldManagerAddress,
      config.rebalance.toleranceAmountWei,
      config.rebalance.minWithdrawalThresholdEth,
      rebalanceQuotaService,
      this.metricsUpdater,
    );
    this.lazyOracleContractClient = new LazyOracleContractClient(
      new WinstonLogger(LazyOracleContractClient.name, config.loggerOptions),
      this.viemBlockchainClientAdapter,
      config.contractAddresses.lazyOracleAddress,
      config.timing.trigger.pollIntervalMs,
      config.timing.trigger.maxInactionMs,
    );
    this.vaultHubContractClient = new VaultHubContractClient(
      this.viemBlockchainClientAdapter,
      config.contractAddresses.vaultHubAddress,
      new WinstonLogger(VaultHubContractClient.name, config.loggerOptions),
    );
    this.lineaRollupYieldExtensionContractClient = new LineaRollupYieldExtensionContractClient(
      new WinstonLogger(LineaRollupYieldExtensionContractClient.name, config.loggerOptions),
      this.viemBlockchainClientAdapter,
      config.contractAddresses.lineaRollupContractAddress,
    );
    this.stethContractClient = new STETHContractClient(
      this.viemBlockchainClientAdapter,
      config.contractAddresses.stethAddress,
      new WinstonLogger(STETHContractClient.name, config.loggerOptions),
    );

    this.exponentialBackoffRetryService = new ExponentialBackoffRetryService(
      new WinstonLogger(ExponentialBackoffRetryService.name, config.loggerOptions),
    );
    this.beaconNodeApiClient = new BeaconNodeApiClient(
      new WinstonLogger(BeaconNodeApiClient.name, config.loggerOptions),
      this.exponentialBackoffRetryService,
      config.dataSources.beaconChainRpcUrl,
    );
    this.oAuth2TokenClient = new OAuth2TokenClient(
      new WinstonLogger(OAuth2TokenClient.name, config.loggerOptions),
      this.exponentialBackoffRetryService,
      config.consensysStakingOAuth2.tokenEndpoint,
      config.consensysStakingOAuth2.clientId,
      config.consensysStakingOAuth2.clientSecret,
      config.consensysStakingOAuth2.audience,
    );
    this.apolloClient = createApolloClient(this.oAuth2TokenClient, config.dataSources.stakingGraphQLUrl);
    this.consensysStakingGraphQLClient = new ConsensysStakingApiClient(
      new WinstonLogger(ConsensysStakingApiClient.name, config.loggerOptions),
      this.exponentialBackoffRetryService,
      this.apolloClient,
      this.beaconNodeApiClient,
    );
    this.lidoAccountingReportClient = new LidoAccountingReportClient(
      new WinstonLogger(LidoAccountingReportClient.name, config.loggerOptions),
      this.exponentialBackoffRetryService,
      this.lazyOracleContractClient,
      config.dataSources.ipfsBaseUrl,
    );
    this.beaconChainStakingClient = new BeaconChainStakingClient(
      new WinstonLogger(BeaconChainStakingClient.name, config.loggerOptions),
      this.metricsUpdater,
      this.consensysStakingGraphQLClient,
      config.rebalance.maxValidatorWithdrawalRequestsPerTransaction,
      this.yieldManagerContractClient,
      this.config.contractAddresses.lidoYieldProviderAddress,
      config.rebalance.minWithdrawalThresholdEth,
    );

    // Processor Services
    this.operationModeMetricsRecorder = new OperationModeMetricsRecorder(
      new WinstonLogger(OperationModeMetricsRecorder.name, config.loggerOptions),
      this.metricsUpdater,
      this.yieldManagerContractClient,
      this.vaultHubContractClient,
    );

    this.yieldReportingOperationModeProcessor = new YieldReportingProcessor(
      new WinstonLogger(YieldReportingProcessor.name, config.loggerOptions),
      this.metricsUpdater,
      this.operationModeMetricsRecorder,
      this.yieldManagerContractClient,
      this.lazyOracleContractClient,
      this.lineaRollupYieldExtensionContractClient,
      this.lidoAccountingReportClient,
      this.beaconChainStakingClient,
      this.vaultHubContractClient,
      config.contractAddresses.lidoYieldProviderAddress,
      config.contractAddresses.l2YieldRecipientAddress,
      config.reporting.shouldSubmitVaultReport,
      config.reporting.shouldReportYield,
      config.reporting.isUnpauseStakingEnabled,
      config.reporting.minNegativeYieldDiffToReportYieldWei,
      config.rebalance.minWithdrawalThresholdEth,
      config.reporting.cyclesPerYieldReport,
    );

    this.ossificationPendingOperationModeProcessor = new OssificationPendingProcessor(
      new WinstonLogger(OssificationPendingProcessor.name, config.loggerOptions),
      this.metricsUpdater,
      this.operationModeMetricsRecorder,
      this.yieldManagerContractClient,
      this.lazyOracleContractClient,
      this.lidoAccountingReportClient,
      this.beaconChainStakingClient,
      this.vaultHubContractClient,
      config.contractAddresses.lidoYieldProviderAddress,
      config.reporting.shouldSubmitVaultReport,
    );

    this.ossificationCompleteOperationModeProcessor = new OssificationCompleteProcessor(
      new WinstonLogger(OssificationCompleteProcessor.name, config.loggerOptions),
      this.metricsUpdater,
      this.operationModeMetricsRecorder,
      this.yieldManagerContractClient,
      this.beaconChainStakingClient,
      config.timing.trigger.maxInactionMs,
      config.contractAddresses.lidoYieldProviderAddress,
    );

    this.gaugeMetricsPoller = new GaugeMetricsPoller(
      new WinstonLogger(GaugeMetricsPoller.name, config.loggerOptions),
      this.consensysStakingGraphQLClient,
      this.metricsUpdater,
      this.yieldManagerContractClient,
      this.vaultHubContractClient,
      config.contractAddresses.lidoYieldProviderAddress,
      this.beaconNodeApiClient,
      config.timing.gaugeMetricsPollIntervalMs,
      this.stethContractClient,
    );

    this.operationModeSelector = new OperationModeSelector(
      new WinstonLogger(OperationModeSelector.name, config.loggerOptions),
      this.metricsUpdater,
      this.yieldManagerContractClient,
      this.yieldReportingOperationModeProcessor,
      this.ossificationPendingOperationModeProcessor,
      this.ossificationCompleteOperationModeProcessor,
      config.contractAddresses.lidoYieldProviderAddress,
      config.timing.contractReadRetryTimeMs,
    );
  }

  /**
   * Starts all services.
   * Purposely refrains from awaiting .start() methods so they don't become blocking calls.
   * Starts the metrics API server, gauge metrics poller, and the operation mode selector.
   */
  public startAllServices(): void {
    this.api.start();
    this.logger.info("Metrics API server started");
    this.gaugeMetricsPoller.start();
    this.logger.info("Gauge metrics poller started");
    this.operationModeSelector.start();
    this.logger.info("Native yield automation service started");
  }

  /**
   * Stops all services gracefully.
   * Stops the metrics API server, gauge metrics poller, and the operation mode selector.
   */
  public stopAllServices(): void {
    this.api.stop();
    this.logger.info("Metrics API server stopped");
    this.gaugeMetricsPoller.stop();
    this.logger.info("Gauge metrics poller stopped");
    this.operationModeSelector.stop();
    this.logger.info("Native yield automation service stopped");
  }

  /**
   * Gets the bootstrap configuration.
   *
   * @returns {NativeYieldAutomationServiceBootstrapConfig} The configuration object used to initialize this bootstrap instance.
   */
  public getConfig(): NativeYieldAutomationServiceBootstrapConfig {
    return this.config;
  }
}
