(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;   ACCOUNT consistency temporal columns   ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

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
      acp_BLK_NUMBER
      acp_TOTL_TXN_NUMBER
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
      acp_EXISTS
      acp_EXISTS_NEW
      acp_HAS_CODE
      acp_HAD_CODE_INITIALLY
      acp_WARMTH
      acp_WARMTH_NEW
      acp_DEPLOYMENT_NUMBER
      acp_DEPLOYMENT_NUMBER_NEW
      acp_DEPLOYMENT_STATUS
      acp_DEPLOYMENT_STATUS_NEW
      acp_MARKED_FOR_DELETION
      acp_MARKED_FOR_DELETION_NEW
      acp_TRM_FLAG
      acp_IS_PRECOMPILE
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
      BLK_NUMBER
      TOTL_TXN_NUMBER
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
      account/EXISTS
      account/EXISTS_NEW
      account/HAS_CODE
      account/HAD_CODE_INITIALLY
      account/WARMTH
      account/WARMTH_NEW
      account/DEPLOYMENT_NUMBER
      account/DEPLOYMENT_NUMBER_NEW
      account/DEPLOYMENT_STATUS
      account/DEPLOYMENT_STATUS_NEW
      account/MARKED_FOR_DELETION
      account/MARKED_FOR_DELETION_NEW
      account/TRM_FLAG
      account/IS_PRECOMPILE
    )
  )




