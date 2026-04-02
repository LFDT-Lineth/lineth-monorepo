import { WinstonLogger } from "@consensys/linea-shared-utils";
import { type LoggerOptions } from "winston";

import { L1NetworkConfig, L2NetworkConfig } from "./config/config";
import { ILineaRollupClient } from "../../../core/clients/blockchain/ethereum/ILineaRollupClient";
import { IEthereumGasProvider } from "../../../core/clients/blockchain/IGasProvider";
import { IProvider } from "../../../core/clients/blockchain/IProvider";
import { IL2MessageServiceLogClient } from "../../../core/clients/blockchain/linea/IL2MessageServiceLogClient";
import { Direction } from "../../../core/enums";
import { IErrorParser } from "../../../core/errors/IErrorParser";
import { ISponsorshipMetricsUpdater, ITransactionMetricsUpdater } from "../../../core/metrics";
import { IMessageRepository } from "../../../core/persistence/IMessageRepository";
import { ICalldataDecoder } from "../../../core/services/ICalldataDecoder";
import { INonceManager } from "../../../core/services/INonceManager";
import { IReceiptPoller } from "../../../core/services/IReceiptPoller";
import { ITransactionRetrier } from "../../../core/services/ITransactionRetrier";
import { ITransactionValidationService } from "../../../core/services/ITransactionValidationService";
import { IPoller } from "../../../core/services/pollers/IPoller";
import { IntervalPoller, MessageSentEventPoller } from "../../../services/pollers";
import {
  MessageAnchoringProcessor,
  MessageClaimingPersister,
  MessageClaimingProcessor,
  MessageSentEventProcessor,
  ReceiptStatusResolver,
  TransactionLifecycleManager,
} from "../../../services/processors";

export type L2ToL1Deps = {
  l2LogClient: IL2MessageServiceLogClient;
  l2Provider: IProvider;
  lineaRollupClient: ILineaRollupClient;
  l1Provider: IProvider;
  l1NonceManager: INonceManager;
  l1TransactionRetrier: ITransactionRetrier;
  l1ReceiptPoller: IReceiptPoller;
  messageRepository: IMessageRepository;
  l1GasProvider: IEthereumGasProvider;
  transactionValidationService: ITransactionValidationService;
  calldataDecoder: ICalldataDecoder;
  sponsorshipMetricsUpdater: ISponsorshipMetricsUpdater;
  transactionMetricsUpdater: ITransactionMetricsUpdater;
  errorParser: IErrorParser;
  l1Config: L1NetworkConfig;
  l2Config: L2NetworkConfig;
  loggerOptions?: LoggerOptions;
};

export class L2ToL1App {
  private readonly pollers: IPoller[];

  constructor(deps: L2ToL1Deps) {
    const {
      l2LogClient,
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
    } = deps;

    const log = (name: string) => new WinstonLogger(name, loggerOptions);

    const sentEventProcessor = new MessageSentEventProcessor(
      messageRepository,
      l2LogClient,
      l2Provider,
      calldataDecoder,
      {
        direction: Direction.L2_TO_L1,
        maxBlocksToFetchLogs: l2Config.listener.maxBlocksToFetchLogs,
        blockConfirmation: l2Config.listener.blockConfirmation,
        isEOAEnabled: l2Config.isEOAEnabled,
        isCalldataEnabled: l2Config.isCalldataEnabled,
        eventFilters: l2Config.listener.eventFilters,
      },
      log(`L2${MessageSentEventProcessor.name}`),
    );

    const sentEventPoller = new MessageSentEventPoller(
      sentEventProcessor,
      l2Provider,
      messageRepository,
      {
        direction: Direction.L2_TO_L1,
        pollingInterval: l2Config.listener.pollingInterval,
        initialFromBlock: l2Config.listener.initialFromBlock,
        originContractAddress: l2Config.messageServiceContractAddress,
      },
      log(`L2${MessageSentEventPoller.name}`),
    );

    const anchoringProcessor = new MessageAnchoringProcessor(
      lineaRollupClient,
      messageRepository,
      {
        direction: Direction.L2_TO_L1,
        maxFetchMessagesFromDb: l1Config.listener.maxFetchMessagesFromDb,
        originContractAddress: l2Config.messageServiceContractAddress,
      },
      log(`L1${MessageAnchoringProcessor.name}`),
    );

    const anchoringPoller = new IntervalPoller(
      anchoringProcessor,
      { direction: Direction.L2_TO_L1, pollingInterval: l1Config.listener.pollingInterval },
      log("L1MessageAnchoringPoller"),
    );

    const getNextMessageToClaim = async () => {
      const { maxFeePerGas } = await l1GasProvider.getGasFees();
      return messageRepository.getFirstMessageToClaimOnL1(
        Direction.L2_TO_L1,
        l2Config.messageServiceContractAddress,
        maxFeePerGas,
        l1Config.claiming.profitMargin,
        l1Config.claiming.maxNumberOfRetries,
        l1Config.claiming.retryDelayInSeconds,
      );
    };

    const claimingProcessor = new MessageClaimingProcessor(
      lineaRollupClient,
      l1NonceManager,
      messageRepository,
      getNextMessageToClaim,
      transactionValidationService,
      errorParser,
      {
        direction: Direction.L2_TO_L1,
        originContractAddress: l2Config.messageServiceContractAddress,
        feeRecipientAddress: l1Config.claiming.feeRecipientAddress,
        profitMargin: l1Config.claiming.profitMargin,
        maxNumberOfRetries: l1Config.claiming.maxNumberOfRetries,
        retryDelayInSeconds: l1Config.claiming.retryDelayInSeconds,
        maxClaimGasLimit: l1Config.claiming.maxClaimGasLimit,
        claimViaAddress: l1Config.claiming.claimViaAddress,
      },
      log(`L1${MessageClaimingProcessor.name}`),
    );

    const claimingPoller = new IntervalPoller(
      claimingProcessor,
      { direction: Direction.L2_TO_L1, pollingInterval: l1Config.listener.pollingInterval },
      log("L1MessageClaimingPoller"),
    );

    const transactionLifecycleManager = new TransactionLifecycleManager(
      lineaRollupClient,
      l1Provider,
      l1TransactionRetrier,
      l1ReceiptPoller,
      messageRepository,
      {
        receiptPollingTimeout: l1Config.claiming.messageSubmissionTimeout,
        receiptPollingInterval: l1Config.listener.receiptPollingInterval,
      },
      log(`L1${TransactionLifecycleManager.name}`),
    );

    const receiptStatusResolver = new ReceiptStatusResolver(
      messageRepository,
      lineaRollupClient,
      l1Provider,
      sponsorshipMetricsUpdater,
      transactionMetricsUpdater,
      { direction: Direction.L2_TO_L1 },
      log(`L1${ReceiptStatusResolver.name}`),
    );

    const claimingPersister = new MessageClaimingPersister(
      messageRepository,
      l1Provider,
      transactionLifecycleManager,
      receiptStatusResolver,
      {
        direction: Direction.L2_TO_L1,
        messageSubmissionTimeout: l1Config.claiming.messageSubmissionTimeout,
        maxBumpsPerCycle: l1Config.claiming.maxBumpsPerCycle,
        maxCycles: l1Config.claiming.maxRetryCycles,
      },
      log(`L1${MessageClaimingPersister.name}`),
    );

    const persistingPoller = new IntervalPoller(
      claimingPersister,
      { direction: Direction.L2_TO_L1, pollingInterval: l1Config.listener.receiptPollingInterval },
      log("L1MessagePersistingPoller"),
    );

    this.pollers = [sentEventPoller, anchoringPoller, claimingPoller, persistingPoller];
  }

  public start(): void {
    this.pollers.forEach((p) => p.start());
  }

  public stop(): void {
    this.pollers.forEach((p) => p.stop());
  }
}
