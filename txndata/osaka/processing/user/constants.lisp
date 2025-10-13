(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    X. Shorthands    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;


(defconst
  ROFF___USER___HUB_ROW                                                                          0
  ROFF___USER___RLP_ROW                                                                          1
  ROFF___USER___CMPTN_ROW___EIP-2681___MAX_NONCE_UPPER_BOUND_CHECK                               2
  ROFF___USER___CMPTN_ROW___INITIAL_BALANCE_MUST_COVER_VALUE_AND_GAS                             3
  ROFF___USER___CMPTN_ROW___EIP-3860___MAX_INIT_CODE_SIZE_BOUND_CHECK                            4
  ROFF___USER___CMPTN_ROW___EIP-3860___INIT_CODE_WORD_PRICING                                    5
  ROFF___USER___CMPTN_ROW___GAS_LIMIT_MUST_COVER_THE_UPFRONT_GAS_COST                            6
  ROFF___USER___CMPTN_ROW___GAS_LIMIT_CAP                                                        7 
  ROFF___USER___CMPTN_ROW___GAS_LIMIT_MUST_COVER_THE_TRANSACTION_FLOOR_COST                      8
  ROFF___USER___CMPTN_ROW___UPPER_LIMIT_FOR_GAS_REFUNDS                                          9
  ROFF___USER___CMPTN_ROW___EFFECTIVE_GAS_REFUND_COMPUTATION                                     10
  ROFF___USER___CMPTN_ROW___EFFECTIVE_GAS_REFUND_VS_TRANSACTION_CALL_DATA_FLOOR_PRICE_COMPARISON 11
  ROFF___USER___CMPTN_ROW___DETECTING_EMPTY_CALL_DATA                                            12
  ROFF___USER___CMPTN_ROW___THE_MAXIMUM_GAS_PRICE_MUST_MATCH_OR_EXCEED_THE_BASEFEE               13
  ROFF___USER___CMPTN_ROW___CUMULATIVE_GAS_CONSUMPTION_MUST_NOT_EXCEED_BLOCK_GAS_LIMIT           14
  ;; specialized computations for transactions with EIP-1559 gas semantics
  ROFF___USER___CMPTN_ROW___COMPARING_MAX_FEE_AND_MAX_PRIORITY_FEE                               15
  ROFF___USER___CMPTN_ROW___COMPUTING_THE_EFFECTIVE_GAS_PRICE                                    16

  nROWS___TRANSACTION___SANS___EIP_1559_GAS_SEMANTICS                                            15
  nROWS___TRANSACTION___WITH___EIP_1559_GAS_SEMANTICS                                            17
  )

