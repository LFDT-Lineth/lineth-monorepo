(module txndata)

(defconst 
  CT_MAX_TYPE_0                                         7
  CT_MAX_TYPE_1                                         8
  CT_MAX_TYPE_2                                         8
  ;;
  NB_ROWS_TYPE_0                                         8
  NB_ROWS_TYPE_1                                         9
  NB_ROWS_TYPE_2                                         9
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
  comparison---nonce-row-offset                         0
  comparison---initial-balance-row-offset               1
  comparison---sufficient-gas-row-offset                2
  comparison---upper-limit-refunds-row-offset           3
  comparison---effective-refund-row-offset              4
  comparison---detecting-empty-call-data-row-offset     5
  comparison---max-fee-and-basefee-row-offset           6
  comparison---maxfee-and-max-priority-fee-row-offset   7
  comparison---computing-effective-gas-price-row-offset 8)


