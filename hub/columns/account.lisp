(module hub)

(defperspective account 

	;; selector
	PEEK_AT_ACCOUNT
	
	;; account-row columns
	(
		ADDRESS_HI
		ADDRESS_LO
		NONCE
		NONCE_NEW
		BALANCE
		BALANCE_NEW
		CODE_SIZE
		CODE_SIZE_NEW
		CODE_HASH_HI
		CODE_HASH_LO
		CODE_HASH_HI_NEW
		CODE_HASH_LO_NEW
		( HAS_CODE                      :binary@prove ) ;; TODO: demote to debug constraint 
		( HAS_CODE_NEW                  :binary@prove ) ;; TODO: demote to debug constraint 
		CODE_FRAGMENT_INDEX
		( ROM_LEX_FLAG                  :binary@prove )
		( EXISTS 			:binary@prove ) ;; TODO: demote to debug constraint, both are fully constrained
		( EXISTS_NEW                    :binary@prove ) ;; TODO: demote to debug constraint, both are fully constrained
		( WARMTH                        :binary@prove ) ;; TODO: demote to debug constraint 
		( WARMTH_NEW                    :binary@prove ) ;; TODO: demote to debug constraint 
		( MARKED_FOR_SELFDESTRUCT       :binary@prove ) ;; TODO: demote to debug constraint 
		( MARKED_FOR_SELFDESTRUCT_NEW   :binary@prove ) ;; TODO: demote to debug constraint 
		DEPLOYMENT_NUMBER
		DEPLOYMENT_NUMBER_NEW
		DEPLOYMENT_NUMBER_INFTY
		( DEPLOYMENT_STATUS             :binary@prove ) ;; TODO: demote to debug constraint 
		( DEPLOYMENT_STATUS_NEW         :binary@prove ) ;; TODO: demote to debug constraint 
		( DEPLOYMENT_STATUS_INFTY       :binary@prove ) ;; TODO: demote to debug constraint 

		;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
		;;                               ;;
		;;   TRM module lookup columns   ;;
		;;                               ;;
		;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
		( TRM_FLAG                      :binary@prove )
		( IS_PRECOMPILE                 :binary@prove ) ;; TODO: demote to debug constraint 
		  TRM_RAW_ADDRESS_HI

		;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
		;;                                   ;;
		;;   RLPADDR module lookup columns   ;;
		;;                                   ;;
		;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
		( RLPADDR_FLAG               :binary@prove )
		RLPADDR_RECIPE
		RLPADDR_DEP_ADDR_HI
		RLPADDR_DEP_ADDR_LO
		RLPADDR_SALT_HI
		RLPADDR_SALT_LO
		RLPADDR_KEC_HI
		RLPADDR_KEC_LO
	)
)
