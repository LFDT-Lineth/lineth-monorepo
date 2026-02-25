(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                                          ;;;;
;;;;    X.5 Account consistency constraints   ;;;;
;;;;                                          ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    X.5.6 Finalization Constraints   ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    account-consistency---finalization---block-level
                  (:guard   acp_FINAL_IN_BLK)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!    acp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK    acp_DEPLOYMENT_NUMBER_NEW)
                    (eq!    acp_EXISTS_FINAL_IN_BLOCK               acp_EXISTS_NEW           )
                    ))

(defconstraint    account-consistency---finalization---transaction-level
                  (:guard   acp_FINAL_IN_TXN)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (vanishes!    acp_DEPLOYMENT_STATUS_NEW)
                    (if-not-zero  acp_TX_AUTH
                                  (vanishes!   acp_HAD_CODE_INITIALLY))
                    ))
