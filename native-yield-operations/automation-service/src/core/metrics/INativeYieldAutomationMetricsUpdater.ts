import { Address, Hex } from "viem";
import { RebalanceDirection } from "../entities/RebalanceRequirement.js";
import { OperationMode } from "../enums/OperationModeEnums.js";
import { OperationTrigger } from "./LineaNativeYieldAutomationServiceMetrics.js";

export interface INativeYieldAutomationMetricsUpdater {
  recordRebalance(direction: RebalanceDirection.STAKE | RebalanceDirection.UNSTAKE, amountGwei: number): void;

  addValidatorPartialUnstakeAmount(validatorPubkey: Hex, amountGwei: number): void;

  incrementValidatorExit(validatorPubkey: Hex, count?: number): void;

  incrementLidoVaultAccountingReport(vaultAddress: Address): void;

  incrementReportYield(vaultAddress: Address): void;

  addReportedYieldAmount(vaultAddress: Address, amountGwei: number): void;

  setLastPeekedNegativeYieldReport(vaultAddress: Address, negativeYield: number): Promise<void>;

  setLastPeekedPositiveYieldReport(vaultAddress: Address, yieldAmount: number): Promise<void>;

  setLastPeekUnpaidLidoProtocolFees(vaultAddress: Address, feesAmount: number): Promise<void>;

  addNodeOperatorFeesPaid(vaultAddress: Address, amountGwei: number): void;

  addLiabilitiesPaid(vaultAddress: Address, amountGwei: number): void;

  addLidoFeesPaid(vaultAddress: Address, amountGwei: number): void;

  incrementOperationModeTrigger(mode: OperationMode, trigger: OperationTrigger): void;

  incrementOperationModeExecution(mode: OperationMode): void;

  recordOperationModeDuration(mode: OperationMode, durationSeconds: number): void;
}
