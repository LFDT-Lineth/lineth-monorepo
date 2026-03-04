import { Address, Hex } from "viem";
import { RebalanceDirection } from "../entities/RebalanceRequirement.js";
import { OperationMode } from "../enums/OperationModeEnums.js";
import { OperationModeExecutionStatus } from "./LineaNativeYieldAutomationServiceMetrics.js";

export interface INativeYieldAutomationMetricsUpdater {
  recordRebalance(direction: RebalanceDirection, amountGwei: number): void;

  addValidatorPartialUnstakeAmount(validatorPubkey: Hex, amountGwei: number): void;

  incrementValidatorExit(validatorPubkey: Hex, count?: number): void;

  setValidatorStakedAmountGwei(pubkey: Hex, amountGwei: number): void;

  incrementLidoVaultAccountingReport(vaultAddress: Address): void;

  incrementReportYield(vaultAddress: Address): void;

  setLastPeekedNegativeYieldReport(vaultAddress: Address, negativeYield: number): void;

  setLastPeekedPositiveYieldReport(vaultAddress: Address, yieldAmount: number): void;

  setLastSettleableLidoFees(vaultAddress: Address, feesAmount: number): void;

  setLastVaultReportTimestamp(vaultAddress: Address, timestamp: number): void;

  setYieldReportedCumulative(vaultAddress: Address, amountGwei: number): void;

  setLstLiabilityPrincipalGwei(vaultAddress: Address, amountGwei: number): void;

  setLastReportedNegativeYield(vaultAddress: Address, amountGwei: number): void;

  setLidoLstLiabilityGwei(vaultAddress: Address, amountGwei: number): void;

  setLastTotalPendingPartialWithdrawalsGwei(totalPendingPartialWithdrawalsGwei: number): void;

  setLastTotalValidatorBalanceGwei(totalValidatorBalanceGwei: number): void;

  setLastTotalPendingDepositGwei(totalPendingDepositGwei: number): void;

  setPendingPartialWithdrawalQueueAmountGwei(pubkey: Hex, withdrawableEpoch: number, amountGwei: number): void;

  setPendingDepositQueueAmountGwei(pubkey: Hex, slot: number, amountGwei: number): void;

  setPendingExitQueueAmountGwei(pubkey: Hex, exitEpoch: number, amountGwei: number, slashed: boolean): void;

  setLastTotalPendingExitGwei(totalPendingExitGwei: number): void;

  setPendingFullWithdrawalQueueAmountGwei(
    pubkey: Hex,
    withdrawableEpoch: number,
    amountGwei: number,
    slashed: boolean,
  ): void;

  setLastTotalPendingFullWithdrawalGwei(totalPendingFullWithdrawalGwei: number): void;

  addNodeOperatorFeesPaid(vaultAddress: Address, amountGwei: number): void;

  addLiabilitiesPaid(vaultAddress: Address, amountGwei: number): void;

  addLidoFeesPaid(vaultAddress: Address, amountGwei: number): void;

  incrementOperationModeExecution(mode: OperationMode, status?: OperationModeExecutionStatus): void;

  recordOperationModeDuration(mode: OperationMode, durationSeconds: number): void;

  incrementStakingDepositQuotaExceeded(vaultAddress: Address): void;

  setActualRebalanceRequirement(vaultAddress: Address, requirementGwei: number, direction: RebalanceDirection): void;

  setReportedRebalanceRequirement(vaultAddress: Address, requirementGwei: number, direction: RebalanceDirection): void;

  incrementContractEstimateGasError(contractAddress: Address, rawRevertData: string, errorName?: string): void;

  setBeaconChainEpochDrift(drift: number): void;
}
