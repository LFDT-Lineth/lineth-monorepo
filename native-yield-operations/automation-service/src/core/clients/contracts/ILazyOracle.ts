import { Address, Hex } from "viem";
import { IBaseContractClient } from "@consensys/linea-shared-utils";
import { OperationTrigger } from "../../metrics/LineaNativeYieldAutomationServiceMetrics.js";

export interface ILazyOracle<TransactionReceipt> extends IBaseContractClient {
  updateVaultData(params: UpdateVaultDataParams): Promise<TransactionReceipt>;
  latestReportData(): Promise<LazyOracleReportData>;
  waitForVaultsReportDataUpdatedEvent(): Promise<WaitForVaultReportDataEventResult>;
}

export interface UpdateVaultDataParams {
  vault: Address;
  totalValue: bigint;
  cumulativeLidoFees: bigint;
  liabilityShares: bigint;
  maxLiabilityShares: bigint;
  slashingReserve: bigint;
  proof: Hex[];
}

export type WaitForVaultReportDataEventResult = VaultReportResult | TimeoutResult;
export interface VaultReportResult {
  result: OperationTrigger.VAULTS_REPORT_DATA_UPDATED_EVENT;
  report: LazyOracleReportData;
  txHash: Hex;
}

export interface TimeoutResult {
  result: OperationTrigger.TIMEOUT;
}

export interface LazyOracleReportData {
  timestamp: bigint;
  refSlot: bigint;
  treeRoot: Hex;
  reportCid: string;
}
