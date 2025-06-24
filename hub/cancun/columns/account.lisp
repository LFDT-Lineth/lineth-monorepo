(module hub)

(defperspective account

  ;; selector
  PEEK_AT_ACCOUNT

  ;; account-row columns
  ((ADDRESS_HI                  :i32)
   (ADDRESS_LO                  :i128)
   (NONCE                       :i64  :display :dec)
   (NONCE_NEW                   :i64  :display :dec)
   (BALANCE                     :i128 :display :dec)
   (BALANCE_NEW                 :i128 :display :dec)
   (CODE_SIZE                   :i32  :display :dec)
   (CODE_SIZE_NEW               :i32  :display :dec)
   (CODE_HASH_HI                :i128)
   (CODE_HASH_LO                :i128)
   (CODE_HASH_HI_NEW            :i128)
   (CODE_HASH_LO_NEW            :i128)
   (HAS_CODE                    :binary) ;; rmk1: explicitly set on every account row, de facto binary
   (HAS_CODE_NEW                :binary) ;; rmk1
   (HAD_CODE_INITIALLY          :binary)
   (CODE_FRAGMENT_INDEX         :i32 :display :dec)
   (ROMLEX_FLAG                 :binary@prove)
   (EXISTS                      :binary) ;; rmk1
   (EXISTS_NEW                  :binary) ;; rmk1
   (WARMTH                      :binary@prove) ;; rmk2: should be binary even without the explicit constraint; we keep it as a safety;
   (WARMTH_NEW                  :binary@prove) ;; rmk2
   (MARKED_FOR_DELETION         :binary@prove) ;; rmk2
   (MARKED_FOR_DELETION_NEW     :binary@prove) ;; rmk2
   (DEPLOYMENT_NUMBER           :i32 :display :dec)
   (DEPLOYMENT_NUMBER_NEW       :i32 :display :dec)
   (DEPLOYMENT_STATUS           :binary@prove) ;; rmk2
   (DEPLOYMENT_STATUS_NEW       :binary@prove) ;; rmk2

   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
   ;;                               ;;
   ;;   TRM module lookup columns   ;;
   ;;                               ;;
   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

   (TRM_FLAG           :binary@prove)
   (IS_PRECOMPILE      :binary@prove) ;; rmk: this flag is set from the TRM module the very first time an address is encountered in a conflation, and is conflation-constant; the @prove is redundant; we keep it for safety;
   (TRM_RAW_ADDRESS_HI :i128)

   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
   ;;                                   ;;
   ;;   RLPADDR module lookup columns   ;;
   ;;                                   ;;
   ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

   (RLPADDR_FLAG        :binary@prove)
   (RLPADDR_RECIPE      :i8)
   (RLPADDR_DEP_ADDR_HI :i32)
   (RLPADDR_DEP_ADDR_LO :i128)
   (RLPADDR_SALT_HI     :i128)
   (RLPADDR_SALT_LO     :i128)
   (RLPADDR_KEC_HI      :i128)
   (RLPADDR_KEC_LO      :i128)
   ))


