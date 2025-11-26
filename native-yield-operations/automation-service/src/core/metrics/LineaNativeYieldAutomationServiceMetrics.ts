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

  // Counter that increment each time a vault accounting report is submitted
  // Single label `vault_address`
  LidoVaultAccountingReportSubmittedTotal = "linea_native_yield_automation_service_lido_vault_accounting_report_submitted_total",

  // Counter that increment each time YieldManager.reportYield is called
  // Single label `vault_address`
  ReportYieldTotal = "linea_native_yield_automation_service_report_yield_total",

  // Counter that increments by the yield amount reported
  // Single label `vault_address`
  ReportYieldAmountTotal = "linea_native_yield_automation_service_report_yield_amount_total",

  // Gauge representing outstanding negative yield from the last peeked yield report
  // Single label `vault_address`
  LastPeekedNegativeYieldReport = "linea_native_yield_automation_service_last_peeked_negative_yield_report",

  // Gauge representing positive yield amount from the last peeked yield report
  // Single label `vault_address`
  LastPeekedPositiveYieldReport = "linea_native_yield_automation_service_last_peeked_positive_yield_report",

  // Gauge representing unpaid Lido protocol fees from the last peek
  // Single label `vault_address`
  LastPeekUnpaidLidoProtocolFees = "linea_native_yield_automation_service_last_peek_unpaid_lido_protocol_fees",

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

  // Counter that increments each time an operation mode is triggered.
  // Labels:
  // i.) `mode`
  // i.) `operation_trigger` - VaultsReportDataUpdated_event vs timeout
  OperationModeTriggerTotal = "linea_native_yield_automation_service_operation_mode_trigger_total",

  // Counter that increments each time an operation mode completes execution.
  // Single label `mode`
  OperationModeExecutionTotal = "linea_native_yield_automation_service_operation_mode_execution_total",

  // Histogram that tracks time for each operation mode run.
  // Single label `mode`
  OperationModeExecutionDurationSeconds = "linea_native_yield_automation_service_operation_mode_execution_duration_seconds",
}

export enum OperationTrigger {
  VAULTS_REPORT_DATA_UPDATED_EVENT = "VAULTS_REPORT_DATA_UPDATED_EVENT",
  TIMEOUT = "TIMEOUT",
}
