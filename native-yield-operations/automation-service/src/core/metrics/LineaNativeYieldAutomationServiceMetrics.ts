// NB - All amounts rounded down in gwei. Due to limitation that PromQL does not support bigint.

export enum LineaNativeYieldAutomationServiceMetrics {
  // Counter that increments each time a rebalance between L1MessageService and YieldProvider is performed
  // Labels:
  // i.) direction = STAKE | UNSTAKE
  // ii.) type = INITIAL | POST_REPORT
  RebalanceAmountTotal = "linea_native_yield_automation_service_rebalance_amount_total",

  // Counter that increments each time a partial beacon chain withdrawal is made
  // Single label - validator_pubkey
  ValidatorPartialUnstakeAmountTotal = "linea_native_yield_automation_service_validator_partial_unstake_amount_total",

  // Counter that increments each time a validator exit is made
  // Single label - validator_pubkey
  ValidatorExitTotal = "linea_native_yield_automation_service_validator_exit_total",

  // Gauge representing the amount staked in a validator in gwei
  // Single label `pubkey`
  ValidatorStakedAmountGwei = "linea_native_yield_automation_service_validator_staked_amount_gwei",

  // Counter that increment each time a vault accounting report is submitted
  // Single label `vault_address`
  LidoVaultAccountingReportSubmittedTotal = "linea_native_yield_automation_service_lido_vault_accounting_report_submitted_total",

  // Counter that increment each time YieldManager.reportYield is called
  // Single label `vault_address`
  ReportYieldTotal = "linea_native_yield_automation_service_report_yield_total",

  // Gauge representing cumulative yield reported from the YieldManager contract
  // Single label `vault_address`
  YieldReportedCumulative = "linea_native_yield_automation_service_yield_reported_cumulative",

  // Gauge representing LST liability principal from the YieldManager contract
  // Single label `vault_address`
  LstLiabilityPrincipalGwei = "linea_native_yield_automation_service_lst_liability_principal_gwei",

  // Gauge representing Lido LST liability in gwei from Lido accounting reports
  // Single label `vault_address`
  LidoLstLiabilityGwei = "linea_native_yield_automation_service_lido_lst_liability_gwei",

  // Gauge representing outstanding negative yield from the last peeked yield report
  // Single label `vault_address`
  LastPeekedNegativeYieldReport = "linea_native_yield_automation_service_last_peeked_negative_yield_report",

  // Gauge representing positive yield amount from the last peeked yield report
  // Single label `vault_address`
  LastPeekedPositiveYieldReport = "linea_native_yield_automation_service_last_peeked_positive_yield_report",

  // Gauge representing last reported negative yield from the YieldManager contract
  // Single label `vault_address`
  LastReportedNegativeYield = "linea_native_yield_automation_service_last_reported_negative_yield",

  // Gauge representing settleable Lido protocol fees from the last query
  // Single label `vault_address`
  LastSettleableLidoFees = "linea_native_yield_automation_service_last_settleable_lido_fees",

  // Gauge representing timestamp from the latest vault report
  // Single label `vault_address`
  LastVaultReportTimestamp = "linea_native_yield_automation_service_last_vault_report_timestamp",

  // Gauge representing total pending partial withdrawals in gwei
  LastTotalPendingPartialWithdrawalsGwei = "linea_native_yield_automation_service_last_total_pending_partial_withdrawals_gwei",

  // Gauge representing total validator balance in gwei
  LastTotalValidatorBalanceGwei = "linea_native_yield_automation_service_last_total_validator_balance_gwei",

  // Gauge representing total pending deposits in gwei
  LastTotalPendingDepositGwei = "linea_native_yield_automation_service_last_total_pending_deposit_gwei",

  // Gauge representing pending partial withdrawal queue amount in gwei
  // Labels:
  // i.) `pubkey` - Validator public key
  // ii.) `withdrawable_epoch` - Epoch when withdrawal becomes available
  PendingPartialWithdrawalQueueAmountGwei = "linea_native_yield_automation_service_pending_partial_withdrawal_queue_amount_gwei",

  // Gauge representing pending deposit queue amount in gwei
  // Labels:
  // i.) `pubkey` - Validator public key
  // ii.) `slot` - Slot number
  PendingDepositQueueAmountGwei = "linea_native_yield_automation_service_pending_deposit_queue_amount_gwei",

  // Gauge representing pending exit queue amount in gwei
  // Labels:
  // i.) `pubkey` - Validator public key
  // ii.) `exit_epoch` - Epoch when exit becomes available
  // iii.) `slashed` - "true" | "false"
  PendingExitQueueAmountGwei = "linea_native_yield_automation_service_pending_exit_queue_amount_gwei",

  // Gauge representing total pending exit amount in gwei
  LastTotalPendingExitGwei = "linea_native_yield_automation_service_last_total_pending_exit_gwei",

  // Gauge representing pending full withdrawal queue amount in gwei
  // Labels:
  // i.) `pubkey` - Validator public key
  // ii.) `withdrawable_epoch` - Epoch when withdrawal becomes available
  // iii.) `slashed` - "true" | "false"
  PendingFullWithdrawalQueueAmountGwei = "linea_native_yield_automation_service_pending_full_withdrawal_queue_amount_gwei",

  // Gauge representing total pending full withdrawal amount in gwei
  LastTotalPendingFullWithdrawalGwei = "linea_native_yield_automation_service_last_total_pending_full_withdrawal_gwei",

  // Counter that increments by the node operator fees paid
  // Single label `vault_address`
  // N.B. Only accounts for payments by the automation service, but external actors can also trigger payment
  NodeOperatorFeesPaidTotal = "linea_native_yield_automation_service_node_operator_fees_paid_total",

  // Counter that increments by the node operator fees paid
  // Single label `vault_address`
  // N.B. Only accounts for payments by the automation service, but external actors can also trigger payment
  LiabilitiesPaidTotal = "linea_native_yield_automation_service_liabilities_paid_total",

  // Counter that increments by Lido fees paid
  // Single label `vault_address`
  // N.B. Only accounts for payments by the automation service, but external actors can also trigger payment
  LidoFeesPaidTotal = "linea_native_yield_automation_service_lido_fees_paid_total",

  // Counter that increments each time an operation mode completes execution.
  // Labels:
  // i.) `mode`
  // ii.) `status` - OperationModeExecutionStatus.Success | OperationModeExecutionStatus.Failure
  OperationModeExecutionTotal = "linea_native_yield_automation_service_operation_mode_execution_total",

  // Histogram that tracks time for each operation mode run.
  // Single label `mode`
  OperationModeExecutionDurationSeconds = "linea_native_yield_automation_service_operation_mode_execution_duration_seconds",

  // Counter that increments each time the staking deposit quota is exceeded
  // Single label `vault_address`
  StakingDepositQuotaExceeded = "linea_native_yield_automation_service_staking_deposit_quota_exceeded",

  // Gauge representing the original rebalance requirement (in gwei) before applying tolerance band, circuit breaker, or rate limit
  // Labels: `vault_address`, `staking_direction` (values: "STAKING", "UNSTAKING", "NONE")
  ActualRebalanceRequirementGwei = "linea_native_yield_automation_service_actual_rebalance_requirement_gwei",

  // Gauge representing the reported rebalance requirement (in gwei) after applying tolerance band, circuit breaker, and rate limit
  // Labels: `vault_address`, `staking_direction` (values: "STAKING", "UNSTAKING", "NONE")
  ReportedRebalanceRequirementGwei = "linea_native_yield_automation_service_reported_rebalance_requirement_gwei",

  // Counter that increments each time a contract estimateGas error occurs
  // Labels:
  // i.) `contract_address` - The contract address where the error occurred
  // ii.) `rawRevertData` - The raw revert data (hex string)
  // iii.) `errorName` - The decoded error name (if available, otherwise "unknown")
  ContractEstimateGasError = "linea_native_yield_automation_service_contract_estimate_gas_error",

  // Gauge representing the absolute epoch difference between primary and reference beacon chain RPCs.
  // Value is -1 when either RPC call fails. Not emitted when reference is not configured.
  BeaconChainEpochDrift = "linea_native_yield_automation_service_beacon_chain_epoch_drift",
}

export enum OperationTrigger {
  VAULTS_REPORT_DATA_UPDATED_EVENT = "VAULTS_REPORT_DATA_UPDATED_EVENT",
  TIMEOUT = "TIMEOUT",
}

export enum OperationModeExecutionStatus {
  Success = "success",
  Failure = "failure",
}
