import { ILogger, IMetricsService, IApplication } from "@consensys/linea-shared-utils";
import { mock, MockProxy } from "jest-mock-extended";
import { DataSource, EntityManager } from "typeorm";

import { ILineaRollupClient } from "../../core/clients/blockchain/ethereum/ILineaRollupClient";
import { ILineaRollupLogClient } from "../../core/clients/blockchain/ethereum/ILineaRollupLogClient";
import { IMerkleTreeService } from "../../core/clients/blockchain/ethereum/IMerkleTreeService";
import { IEthereumGasProvider, ILineaGasProvider } from "../../core/clients/blockchain/IGasProvider";
import { IMessageSentEventLogClient } from "../../core/clients/blockchain/ILogClient";
import { IProvider } from "../../core/clients/blockchain/IProvider";
import { IL2MessageServiceClient } from "../../core/clients/blockchain/linea/IL2MessageServiceClient";
import { IL2MessageServiceLogClient } from "../../core/clients/blockchain/linea/IL2MessageServiceLogClient";
import { ILineaProvider } from "../../core/clients/blockchain/linea/ILineaProvider";
import { IErrorParser } from "../../core/errors/IErrorParser";
import { IMessageMetricsUpdater, ISponsorshipMetricsUpdater, ITransactionMetricsUpdater } from "../../core/metrics";
import { LineaPostmanMetrics } from "../../core/metrics";
import { IMessageRepository } from "../../core/persistence/IMessageRepository";
import { ICalldataDecoder } from "../../core/services/ICalldataDecoder";
import { IDbNonceProvider } from "../../core/services/IDbNonceProvider";
import { INonceManager } from "../../core/services/INonceManager";
import { IReceiptPoller } from "../../core/services/IReceiptPoller";
import { ITransactionRetrier } from "../../core/services/ITransactionRetrier";
import { ITransactionSigner } from "../../core/services/ITransactionSigner";
import { ITransactionValidationService } from "../../core/services/ITransactionValidationService";
import { IPoller } from "../../core/services/pollers/IPoller";
import { IL2ClaimTransactionSizeCalculator } from "../../core/services/processors/IL2ClaimTransactionSizeCalculator";
import { IMessageSentEventProcessor } from "../../core/services/processors/IMessageSentEventProcessor";

export function mockLogger(): MockProxy<ILogger> {
  return mock<ILogger>();
}

export function mockMessageRepository(): MockProxy<IMessageRepository> {
  return mock<IMessageRepository>();
}

export function mockProvider(): MockProxy<IProvider> {
  return mock<IProvider>();
}

export function mockLineaProvider(): MockProxy<ILineaProvider> {
  return mock<ILineaProvider>();
}

export function mockLineaRollupClient(): MockProxy<ILineaRollupClient> {
  return mock<ILineaRollupClient>();
}

export function mockLineaRollupLogClient(): MockProxy<ILineaRollupLogClient> {
  return mock<ILineaRollupLogClient>();
}

export function mockL2MessageServiceClient(): MockProxy<IL2MessageServiceClient> {
  return mock<IL2MessageServiceClient>();
}

export function mockL2MessageServiceLogClient(): MockProxy<IL2MessageServiceLogClient> {
  return mock<IL2MessageServiceLogClient>();
}

export function mockEthereumGasProvider(): MockProxy<IEthereumGasProvider> {
  return mock<IEthereumGasProvider>();
}

export function mockLineaGasProvider(): MockProxy<ILineaGasProvider> {
  return mock<ILineaGasProvider>();
}

export function mockNonceManager(): MockProxy<INonceManager> {
  return mock<INonceManager>();
}

export function mockTransactionRetrier(): MockProxy<ITransactionRetrier> {
  return mock<ITransactionRetrier>();
}

export function mockReceiptPoller(): MockProxy<IReceiptPoller> {
  return mock<IReceiptPoller>();
}

export function mockCalldataDecoder(): MockProxy<ICalldataDecoder> {
  return mock<ICalldataDecoder>();
}

export function mockTransactionSigner(): MockProxy<ITransactionSigner> {
  return mock<ITransactionSigner>();
}

export function mockErrorParser(): MockProxy<IErrorParser> {
  return mock<IErrorParser>();
}

export function mockTransactionValidationService(): MockProxy<ITransactionValidationService> {
  return mock<ITransactionValidationService>();
}

export function mockL2ClaimTransactionSizeCalculator(): MockProxy<IL2ClaimTransactionSizeCalculator> {
  return mock<IL2ClaimTransactionSizeCalculator>();
}

export function mockPoller(): MockProxy<IPoller> {
  return mock<IPoller>();
}

export function mockMessageMetricsUpdater(): MockProxy<IMessageMetricsUpdater> {
  return mock<IMessageMetricsUpdater>();
}

export function mockSponsorshipMetricsUpdater(): MockProxy<ISponsorshipMetricsUpdater> {
  return mock<ISponsorshipMetricsUpdater>();
}

export function mockTransactionMetricsUpdater(): MockProxy<ITransactionMetricsUpdater> {
  return mock<ITransactionMetricsUpdater>();
}

export function mockLogClient(): MockProxy<IMessageSentEventLogClient> {
  return mock<IMessageSentEventLogClient>();
}

export function mockMerkleTreeService(): MockProxy<IMerkleTreeService> {
  return mock<IMerkleTreeService>();
}

export function mockDbNonceProvider(): MockProxy<IDbNonceProvider> {
  return mock<IDbNonceProvider>();
}

export function mockMessageSentEventProcessor(): MockProxy<IMessageSentEventProcessor> {
  return mock<IMessageSentEventProcessor>();
}

export function mockMetricsService(): MockProxy<IMetricsService<LineaPostmanMetrics>> {
  return mock<IMetricsService<LineaPostmanMetrics>>();
}

export function mockApplication(): MockProxy<IApplication> {
  return mock<IApplication>();
}

export function mockDataSource(): MockProxy<DataSource> {
  const ds = mock<DataSource>();
  Object.defineProperty(ds, "subscribers", { value: [], writable: true });
  Object.defineProperty(ds, "manager", { value: mock<EntityManager>(), writable: true });
  return ds;
}
