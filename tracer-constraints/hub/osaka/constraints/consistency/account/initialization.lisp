(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                                          ;;;;
;;;;    X.5 Account consistency constraints   ;;;;
;;;;                                          ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;    X.5.4 Initialization Constraints   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    account-consistency---initialization---conflation-level
                  (:guard   acp_FIRST_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!  acp_TRM_FLAG            1 )
                    (eq!  acp_DEPLOYMENT_NUMBER   0 )
                    (eq!  acp_DELEGATION_NUMBER   0 )
                    ))

(defconstraint    account-consistency---initialization---block-level
                  (:guard   acp_FIRST_IN_BLK)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!    acp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK    acp_DEPLOYMENT_NUMBER )
                    (eq!    acp_EXISTS_FIRST_IN_BLOCK               acp_EXISTS            )
                    ))

(defconstraint    account-consistency---initialization---transaction-level
                  (:guard   acp_FIRST_IN_TXN)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!   acp_WARMTH                acp_IS_PRECOMPILE )
                    (eq!   acp_DEPLOYMENT_STATUS     0                 )
                    (eq!   acp_MARKED_FOR_DELETION   0                 )
                    ;; (eq!   acp_HAD_CODE_INITIALLY    acp_HAS_CODE      )
                    ))

(defconstraint    account-consistency---initialization---transaction-level---HAD_CODE_INITIALLY-for-EIP-7702
                  (:guard   acp_FIRST_IN_TXN)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-zero   acp_TX_AUTH
                             (eq!  acp_HAD_CODE_INITIALLY  acp_HAS_CODE )
                             ))
