(module txndata)

;;TODO: rewrite me completely

(defconst
  CT_MAX_TYPE_0                                         9
  CT_MAX_TYPE_1                                         10
  CT_MAX_TYPE_2                                         10

  ;;
  NB_ROWS_TYPE_0                                         (+ CT_MAX_TYPE_0 1)
  NB_ROWS_TYPE_1                                         (+ CT_MAX_TYPE_1 1)
  NB_ROWS_TYPE_2                                         (+ CT_MAX_TYPE_2 1)

  ;;
  COMMON_RLP_TXN_PHASE_NUMBER_0                          RLP_TXN_PHASE_RLP_PREFIX
  COMMON_RLP_TXN_PHASE_NUMBER_1                          RLP_TXN_PHASE_TO
  COMMON_RLP_TXN_PHASE_NUMBER_2                          RLP_TXN_PHASE_NONCE
  COMMON_RLP_TXN_PHASE_NUMBER_3                          RLP_TXN_PHASE_VALUE
  COMMON_RLP_TXN_PHASE_NUMBER_4                          RLP_TXN_PHASE_DATA
  COMMON_RLP_TXN_PHASE_NUMBER_5                          RLP_TXN_PHASE_GAS_LIMIT
  TYPE_0_RLP_TXN_PHASE_NUMBER_6                          RLP_TXN_PHASE_GAS_PRICE
  TYPE_1_RLP_TXN_PHASE_NUMBER_6                          RLP_TXN_PHASE_GAS_PRICE
  TYPE_1_RLP_TXN_PHASE_NUMBER_7                          RLP_TXN_PHASE_ACCESS_LIST
  TYPE_2_RLP_TXN_PHASE_NUMBER_6                          RLP_TXN_PHASE_MAX_FEE_PER_GAS
  TYPE_2_RLP_TXN_PHASE_NUMBER_7                          RLP_TXN_PHASE_ACCESS_LIST

  ;;
  row-offset---nonce-comparison                         0
  row-offset---initial-balance-comparison               1
  row-offset---init-code-size-limit                     2
  row-offset---init-code-pricing                        3
  row-offset---sufficient-gas-comparison                4
  row-offset---upper-limit-refunds-comparison           5
  row-offset---effective-refund-comparison              6
  row-offset---detecting-empty-call-data-comparison     7
  row-offset---max-fee-and-basefee-comparison           8
  row-offset---max-fee-and-max-priority-fee-comparison  9
  row-offset---computing-effective-gas-price-comparison 10)


