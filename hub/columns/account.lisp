(module hub)

(defperspective account

  ;; selector
  PEEK_AT_ACCOUNT

  ;; account-row columns
  ((ADDRESS_HI :i32)                           ;; 4     bytes
   (ADDRESS_LO :i128)
   (NONCE :i64)
   (NONCE_NEW :i64)
   (BALANCE :i128)
   (BALANCE_NEW :i128)
   (CODE_SIZE :i32)
   (CODE_SIZE_NEW :i32)
   (CODE_HASH_HI :i128)
   (CODE_HASH_LO :i128)
   (CODE_HASH_HI_NEW :i128)
   (CODE_HASH_LO_NEW :i128)
   (HAS_CODE :binary@prove)                    ;; TODO: demote to debug constraint
   (HAS_CODE_NEW :binary@prove)                ;; TODO: demote to debug constraint
   (CODE_FRAGMENT_INDEX :i32)
   (ROMLEX_FLAG :binary@prove)
   (EXISTS :binary@prove)                      ;; TODO: demote to debug constraint, already fully constrained
   (EXISTS_NEW :binary@prove)                  ;; TODO: demote to debug constraint, already fully constrained
   (WARMTH :binary@prove)                      ;; TODO: demote to debug constraint
   (WARMTH_NEW :binary@prove)                  ;; TODO: demote to debug constraint
   (MARKED_FOR_SELFDESTRUCT :binary@prove)     ;; TODO: demote to debug constraint
   (MARKED_FOR_SELFDESTRUCT_NEW :binary@prove) ;; TODO: demote to debug constraint
   (DEPLOYMENT_NUMBER :i32)
   (DEPLOYMENT_NUMBER_NEW :i32)
   (DEPLOYMENT_NUMBER_INFTY :i32)
   (DEPLOYMENT_STATUS :binary@prove)           ;; TODO: demote to debug constraint
   (DEPLOYMENT_STATUS_NEW :binary@prove)       ;; TODO: demote to debug constraint
   (DEPLOYMENT_STATUS_INFTY :binary@prove)     ;; TODO: demote to debug constraint

   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
   ;;                               ;;
   ;;   TRM module lookup columns   ;;
   ;;                               ;;
   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

   (TRM_FLAG :binary@prove)
   (IS_PRECOMPILE :binary@prove)               ;; TODO: demote to debug constraint
   (TRM_RAW_ADDRESS_HI :i128)

   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
   ;;                                   ;;
   ;;   RLPADDR module lookup columns   ;;
   ;;                                   ;;
   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

   (RLPADDR_FLAG :binary@prove)
   (RLPADDR_RECIPE :i8)
   (RLPADDR_DEP_ADDR_HI :i32)
   (RLPADDR_DEP_ADDR_LO :i128)
   (RLPADDR_SALT_HI :i128)
   (RLPADDR_SALT_LO :i128)
   (RLPADDR_KEC_HI :i128)
   (RLPADDR_KEC_LO :i128)

   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
   ;;                                          ;;
   ;;   ACCOUNT consistency temporal columns   ;;
   ;;                                          ;;
   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

  ;; the unpermuted columns whose permuted counter-parts will be used in account-consistency-arguments
  ( FIRST_IN_CNF :binary@prove )     ( FIRST_IN_BLK :binary@prove )     ( FIRST_IN_TXN :binary@prove )
  ( AGAIN_IN_CNF :binary@prove )     ( AGAIN_IN_BLK :binary@prove )     ( AGAIN_IN_TXN :binary@prove )
  ( FINAL_IN_CNF :binary@prove )     ( FINAL_IN_BLK :binary@prove )     ( FINAL_IN_TXN :binary@prove )
  ( DEPLOYMENT_NUMBER_FIRST_IN_BLOCK    :i16)
  ( DEPLOYMENT_NUMBER_FINAL_IN_BLOCK    :i16)
   ))


