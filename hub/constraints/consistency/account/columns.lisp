(module hub)


;; acp_ ⇔ account consistency permutation
(defpermutation
    ;; permuted columns
    ;; replace acp with account_consistency_permutation
    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
    (
      acp_PEEK_AT_ACCOUNT
      acp_ADDRESS_HI
      acp_ADDRESS_LO
      acp_DOM_STAMP
      acp_SUB_STAMP
      ;;
      acp_REL_BLK_NUM
      acp_ABS_TX_NUM
      acp_CODE_FRAGMENT_INDEX
      acp_BALANCE
      acp_BALANCE_NEW
      acp_NONCE
      acp_NONCE_NEW
      acp_CODE_HASH_HI
      acp_CODE_HASH_LO
      acp_CODE_HASH_HI_NEW
      acp_CODE_HASH_LO_NEW
      acp_CODE_SIZE
      acp_CODE_SIZE_NEW
      acp_WARMTH
      acp_WARMTH_NEW
      acp_DEPLOYMENT_NUMBER
      acp_DEPLOYMENT_NUMBER_NEW
      acp_DEPLOYMENT_STATUS
      acp_DEPLOYMENT_STATUS_NEW
      acp_MARKED_FOR_SELFDESTRUCT
      acp_MARKED_FOR_SELFDESTRUCT_NEW
      acp_TRM_FLAG
      acp_IS_PRECOMPILE
      ;; permuted versions
      acp_FIRST_IN_CNF
      acp_FIRST_IN_BLK
      acp_FIRST_IN_TXN
      acp_AGAIN_IN_CNF
      acp_AGAIN_IN_BLK
      acp_AGAIN_IN_TXN
      acp_FINAL_IN_CNF
      acp_FINAL_IN_BLK
      acp_FINAL_IN_TXN
      acp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK
      acp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK
    )
    ;; original columns
    ;;;;;;;;;;;;;;;;;;;
    (
      (↓ PEEK_AT_ACCOUNT )
      (↓ account/ADDRESS_HI )
      (↓ account/ADDRESS_LO )
      (↓ DOM_STAMP )
      (↑ SUB_STAMP )
      ;;
      REL_BLK_NUM
      ABS_TX_NUM
      account/CODE_FRAGMENT_INDEX
      account/BALANCE
      account/BALANCE_NEW
      account/NONCE
      account/NONCE_NEW
      account/CODE_HASH_HI
      account/CODE_HASH_LO
      account/CODE_HASH_HI_NEW
      account/CODE_HASH_LO_NEW
      account/CODE_SIZE
      account/CODE_SIZE_NEW
      account/WARMTH
      account/WARMTH_NEW
      account/DEPLOYMENT_NUMBER
      account/DEPLOYMENT_NUMBER_NEW
      account/DEPLOYMENT_STATUS
      account/DEPLOYMENT_STATUS_NEW
      account/MARKED_FOR_SELFDESTRUCT
      account/MARKED_FOR_SELFDESTRUCT_NEW
      account/TRM_FLAG
      account/IS_PRECOMPILE
      ;; un permuted versions
      account/FIRST_IN_CNF
      account/FIRST_IN_BLK
      account/FIRST_IN_TXN
      account/AGAIN_IN_CNF
      account/AGAIN_IN_BLK
      account/AGAIN_IN_TXN
      account/FINAL_IN_CNF
      account/FINAL_IN_BLK
      account/FINAL_IN_TXN
      account/DEPLOYMENT_NUMBER_FIRST_IN_BLOCK
      account/DEPLOYMENT_NUMBER_FINAL_IN_BLOCK
    )
  )




