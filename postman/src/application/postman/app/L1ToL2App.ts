import { WinstonLogger } from "@consensys/linea-shared-utils";
import { type LoggerOptions } from "winston";

import { L1NetworkConfig, L2NetworkConfig } from "./config/config";
import { ILineaRollupLogClient } from "../../../core/clients/blockchain/ethereum/ILineaRollupLogClient";
import { IProvider } from "../../../core/clients/blockchain/IProvider";
import { IL2MessageServiceClient } from "../../../core/clients/blockchain/linea/IL2MessageServiceClient";
import { ILineaProvider } from "../../../core/clients/blockchain/linea/ILineaProvider";
import { Direction, MessageStatus } from "../../../core/enums";
import { IErrorParser } from "../../../core/errors/IErrorParser";
import { ISponsorshipMetricsUpdater, ITransactionMetricsUpdater } from "../../../core/metrics";
import { IMessageRepository } from "../../../core/persistence/IMessageRepository";
import { ICalldataDecoder } from "../../../core/services/ICalldataDecoder";
import { INonceManager } from "../../../core/services/INonceManager";
import { IReceiptPoller } from "../../../core/services/IReceiptPoller";
import { ITransactionRetrier } from "../../../core/services/ITransactionRetrier";
import { ITransactionValidationService } from "../../../core/services/ITransactionValidationService";
import { IPoller } from "../../../core/services/pollers/IPoller";
import { IL2ClaimTransactionSizeCalculator } from "../../../core/services/processors/IL2ClaimTransactionSizeCalculator";
import { IntervalPoller, MessageSentEventPoller } from "../../../services/pollers";
import {
  L2ClaimMessageTransactionSizeProcessor,
  MessageAnchoringProcessor,
  MessageClaimingPersister,
  MessageClaimingProcessor,
  MessageSentEventProcessor,
  ReceiptStatusResolver,
  TransactionLifecycleManager,
} from "../../../services/processors";

export type L1ToL2Deps = {
  l1LogClient: ILineaRollupLogClient;
  l1Provider: IProvider;
  l2MessageServiceClient: IL2MessageServiceClient;
  l2Provider: ILineaProvider;
  l2NonceManager: INonceManager;
  l2TransactionRetrier: ITransactionRetrier;
  l2ReceiptPoller: IReceiptPoller;
  messageRepository: IMessageRepository;
  calldataDecoder: ICalldataDecoder;
  transactionValidationService: ITransactionValidationService;
  transactionSizeCalculator: IL2ClaimTransactionSizeCalculator;
  sponsorshipMetricsUpdater: ISponsorshipMetricsUpdater;
  transactionMetricsUpdater: ITransactionMetricsUpdater;
  errorParser: IErrorParser;
  l1Config: L1NetworkConfig;
  l2Config: L2NetworkConfig;
  loggerOptions?: LoggerOptions;
};

export class L1ToL2App {
  private readonly pollers: IPoller[];

  constructor(deps: L1ToL2Deps) {
    const {
      l1LogClient,
      l1Provider,
      l2MessageServiceClient,
      l2Provider,
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
    } = deps;

    const log = (name: string) => new WinstonLogger(name, loggerOptions);

    const sentEventProcessor = new MessageSentEventProcessor(
      messageRepository,
      l1LogClient,
      l1Provider,
      calldataDecoder,
      {
        direction: Direction.L1_TO_L2,
        maxBlocksToFetchLogs: l1Config.listener.maxBlocksToFetchLogs,
        blockConfirmation: l1Config.listener.blockConfirmation,
        isEOAEnabled: l1Config.isEOAEnabled,
        isCalldataEnabled: l1Config.isCalldataEnabled,
        eventFilters: l1Config.listener.eventFilters,
      },
      log(`L1${MessageSentEventProcessor.name}`),
    );

    const sentEventPoller = new MessageSentEventPoller(
      sentEventProcessor,
      l1Provider,
      messageRepository,
      {
        direction: Direction.L1_TO_L2,
        pollingInterval: l1Config.listener.pollingInterval,
        initialFromBlock: l1Config.listener.initialFromBlock,
        originContractAddress: l1Config.messageServiceContractAddress,
      },
      log(`L1${MessageSentEventPoller.name}`),
    );

    const anchoringProcessor = new MessageAnchoringProcessor(
      l2MessageServiceClient,
      messageRepository,
      {
        direction: Direction.L1_TO_L2,
        maxFetchMessagesFromDb: l1Config.listener.maxFetchMessagesFromDb,
        originContractAddress: l1Config.messageServiceContractAddress,
      },
      log(`L2${MessageAnchoringProcessor.name}`),
    );

    const anchoringPoller = new IntervalPoller(
      anchoringProcessor,
      { direction: Direction.L1_TO_L2, pollingInterval: l2Config.listener.pollingInterval },
      log("L2MessageAnchoringPoller"),
    );

    const getNextMessageToClaim = () =>
      messageRepository.getFirstMessageToClaimOnL2(
        Direction.L1_TO_L2,
        l1Config.messageServiceContractAddress,
        [MessageStatus.TRANSACTION_SIZE_COMPUTED, MessageStatus.FEE_UNDERPRICED],
        l2Config.claiming.maxNumberOfRetries,
        l2Config.claiming.retryDelayInSeconds,
      );

    const claimingProcessor = new MessageClaimingProcessor(
      l2MessageServiceClient,
      l2NonceManager,
      messageRepository,
      getNextMessageToClaim,
      transactionValidationService,
      errorParser,
      {
        direction: Direction.L1_TO_L2,
        originContractAddress: l1Config.messageServiceContractAddress,
        feeRecipientAddress: l2Config.claiming.feeRecipientAddress,
        profitMargin: l2Config.claiming.profitMargin,
        maxNumberOfRetries: l2Config.claiming.maxNumberOfRetries,
        retryDelayInSeconds: l2Config.claiming.retryDelayInSeconds,
        maxClaimGasLimit: l2Config.claiming.maxClaimGasLimit,
        claimViaAddress: l2Config.claiming.claimViaAddress,
      },
      log(`L2${MessageClaimingProcessor.name}`),
    );

    const claimingPoller = new IntervalPoller(
      claimingProcessor,
      { direction: Direction.L1_TO_L2, pollingInterval: l2Config.listener.pollingInterval },
      log("L2MessageClaimingPoller"),
    );

    const transactionLifecycleManager = new TransactionLifecycleManager(
      l2MessageServiceClient,
      l2Provider,
      l2TransactionRetrier,
      l2ReceiptPoller,
      messageRepository,
      {
        receiptPollingTimeout: l2Config.claiming.messageSubmissionTimeout,
        receiptPollingInterval: l2Config.listener.receiptPollingInterval,
      },
      log(`L2${TransactionLifecycleManager.name}`),
    );

    const receiptStatusResolver = new ReceiptStatusResolver(
      messageRepository,
      l2MessageServiceClient,
      l2Provider,
      sponsorshipMetricsUpdater,
      transactionMetricsUpdater,
      { direction: Direction.L1_TO_L2 },
      log(`L2${ReceiptStatusResolver.name}`),
    );

    const claimingPersister = new MessageClaimingPersister(
      messageRepository,
      l2Provider,
      transactionLifecycleManager,
      receiptStatusResolver,
      {
        direction: Direction.L1_TO_L2,
        messageSubmissionTimeout: l2Config.claiming.messageSubmissionTimeout,
        maxBumpsPerCycle: l2Config.claiming.maxBumpsPerCycle,
        maxCycles: l2Config.claiming.maxRetryCycles,
      },
      log(`L2${MessageClaimingPersister.name}`),
    );

    const persistingPoller = new IntervalPoller(
      claimingPersister,
      { direction: Direction.L1_TO_L2, pollingInterval: l2Config.listener.receiptPollingInterval },
      log("L2MessagePersistingPoller"),
    );

    const sizeProcessor = new L2ClaimMessageTransactionSizeProcessor(
      messageRepository,
      l2MessageServiceClient,
      transactionSizeCalculator,
      { direction: Direction.L1_TO_L2, originContractAddress: l1Config.messageServiceContractAddress },
      log(`${L2ClaimMessageTransactionSizeProcessor.name}`),
      errorParser,
    );

    const sizePoller = new IntervalPoller(
      sizeProcessor,
      { pollingInterval: l2Config.listener.pollingInterval, direction: Direction.L1_TO_L2 },
      log("L2ClaimMessageTransactionSizePoller"),
    );

    this.pollers = [sentEventPoller, anchoringPoller, claimingPoller, persistingPoller, sizePoller];
  }

  public start(): void {
    this.pollers.forEach((p) => p.start());
  }

  public stop(): void {
    this.pollers.forEach((p) => p.stop());
  }
}
