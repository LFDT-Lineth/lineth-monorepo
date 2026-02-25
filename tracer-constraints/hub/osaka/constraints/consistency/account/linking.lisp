(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                                          ;;;;
;;;;    X.5 Account consistency constraints   ;;;;
;;;;                                          ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    X.5.5 Linking Constraints   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;-----------------------------;
;    X.5.5 Conflation level   ;
;-----------------------------;

(defconstraint    account-consistency---linking---conflation-level---nonce
                  (:guard   acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!   acp_NONCE                     (prev acp_NONCE_NEW)               ))

(defconstraint    account-consistency---linking---conflation-level---balance
                  (:guard   acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!   acp_BALANCE                   (prev acp_BALANCE_NEW)             ))

(defconstraint    account-consistency---linking---conflation-level---code
                  (:guard   acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!          acp_CODE_SIZE          (prev acp_CODE_SIZE_NEW)           )
                    (eq!          acp_CODE_HASH_HI       (prev acp_CODE_HASH_HI_NEW)        )
                    (eq!          acp_CODE_HASH_LO       (prev acp_CODE_HASH_LO_NEW)        )
                    (debug (eq!   acp_EXISTS             (prev acp_EXISTS_NEW)              ))))

(defconstraint    account-consistency---linking---conflation-level---precompile-status
                  (:guard   acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!   acp_IS_PRECOMPILE             (prev acp_IS_PRECOMPILE)           ))

(defconstraint    account-consistency---linking---conflation-level---deployment-number-and-status
                  (:guard   acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!   acp_DEPLOYMENT_NUMBER         (prev acp_DEPLOYMENT_NUMBER_NEW))
                    (eq!   acp_DEPLOYMENT_STATUS         (prev acp_DEPLOYMENT_STATUS_NEW))
                    ))

(defconstraint    account-consistency---linking---conflation-level---delegation-data
                  (:guard   acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!   acp_DELEGATION_ADDRESS_HI   (prev   acp_DELEGATION_ADDRESS_HI_NEW ) )
                    (eq!   acp_DELEGATION_ADDRESS_LO   (prev   acp_DELEGATION_ADDRESS_LO_NEW ) )
                    (eq!   acp_DELEGATION_NUMBER       (prev   acp_DELEGATION_NUMBER_NEW     ) )
                    (eq!   acp_IS_DELEGATED            (prev   acp_IS_DELEGATED_NEW          ) )
                    ))


;------------------------;
;    X.5.5 Block level   ;
;------------------------;

(defconstraint    account-consistency---linking---block-level
                  (:guard   acp_AGAIN_IN_BLK)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (remained-constant!    acp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK)
                    (remained-constant!    acp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK)
                    (remained-constant!    acp_EXISTS_FIRST_IN_BLOCK)
                    (remained-constant!    acp_EXISTS_FINAL_IN_BLOCK)
                    ))


;------------------------------;
;    X.5.5 Transaction level   ;
;------------------------------;

(defconstraint    account-consistency---linking---transaction-level
                  (:guard   acp_AGAIN_IN_TXN)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!           acp_HAD_CODE_INITIALLY    (prev   acp_HAD_CODE_INITIALLY           )) ;; transaction-constancy
                    (eq!           acp_WARMTH                (prev   acp_WARMTH_NEW                   ))
                    (eq!           acp_MARKED_FOR_DELETION   (prev   acp_MARKED_FOR_DELETION_NEW      ))
                    (if-not-zero   acp_MARKED_FOR_DELETION   (eq!    acp_MARKED_FOR_DELETION_NEW   1  ))
                    ))

(defconstraint    account-consistency---linking---for-CFI
                  (:guard    acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-eq    acp_DEPLOYMENT_NUMBER_NEW    acp_DEPLOYMENT_NUMBER
                            (if-eq    acp_DEPLOYMENT_STATUS_NEW    acp_DEPLOYMENT_STATUS
                                      (if-eq    acp_DELEGATION_NUMBER_NEW    acp_DELEGATION_NUMBER
                                                (remained-constant!    acp_CODE_FRAGMENT_INDEX)))))

;; acp_TX_AUTH[i - 1] ≡ 1  ∧  acp_TX_AUTH[i] ≡ 0
(defun   (account-consistency---exiting-TX_AUTH-phase)   (*  (prev  acp_TX_AUTH )
                                                             (-  1  acp_TX_AUTH )
                                                             ))

(defconstraint    account-consistency---linking---transaction-level---initializing-HAD_CODE_INITIALLY-for-EIP-7702-provided-there-were-TX_AUTH-rows
                  (:guard   acp_AGAIN_IN_TXN)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq! acp_HAD_CODE_INITIALLY (prev acp_HAD_CODE_INITIALLY ))
                    (if-not-zero   (account-consistency---exiting-TX_AUTH-phase)
                                   (eq!  acp_HAD_CODE_INITIALLY  acp_HAS_CODE )
                                   )))
