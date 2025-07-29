(module hub)

;; scp_ ⇔ storage consistency permutation
(defpermutation
  ;; permuted columns
  ;; replace scp with storage_consistency_permutation
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (
    scp_PEEK_AT_STORAGE
    scp_ADDRESS_HI
    scp_ADDRESS_LO
    scp_STORAGE_KEY_HI
    scp_STORAGE_KEY_LO
    scp_DOM_STAMP
    scp_SUB_STAMP
    ;;
    scp_TOTL_TXN_NUMBER
    scp_BLK_NUMBER
    scp_VALUE_ORIG_HI
    scp_VALUE_ORIG_LO
    scp_VALUE_CURR_HI
    scp_VALUE_CURR_LO
    scp_VALUE_NEXT_HI
    scp_VALUE_NEXT_LO
    ;;
    scp_WARMTH
    scp_WARMTH_NEW
    scp_DEPLOYMENT_NUMBER
    ;;
    scp_PREWARMING_OPERATION
    scp_SLOAD_OPERATION
    scp_SSTORE_OPERATION
    scp_EXCEPTIONAL_OPERATION
  )
  ;; original columns
  ;;;;;;;;;;;;;;;;;;;
  (
    (↓ PEEK_AT_STORAGE )
    (↓ storage/ADDRESS_HI )
    (↓ storage/ADDRESS_LO )
    (↓ storage/STORAGE_KEY_HI )
    (↓ storage/STORAGE_KEY_LO )
    (↓ DOM_STAMP )
    (↑ SUB_STAMP )
    ;;
    TOTL_TXN_NUMBER
    BLK_NUMBER
    storage/VALUE_ORIG_HI
    storage/VALUE_ORIG_LO
    storage/VALUE_CURR_HI
    storage/VALUE_CURR_LO
    storage/VALUE_NEXT_HI
    storage/VALUE_NEXT_LO
    ;;
    storage/WARMTH
    storage/WARMTH_NEW
    storage/DEPLOYMENT_NUMBER
    ;;
    TX_WARM
    storage/SLOAD_OPERATION
    storage/SSTORE_OPERATION
    XAHOY
    )
  )
