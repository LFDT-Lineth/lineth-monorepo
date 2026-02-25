(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                          ;;
;;   X Transactions which skip evm execution                ;;
;;   X.Y The USER-transaction case                          ;;
;;   X.Y.Z Transaction processing                           ;;
;;   X.Y.Z.T Recipient account-row                          ;;
;;                                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-skip---USER---setting-recipient-account-row
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin (eq!    (shift account/ADDRESS_HI           tx-skip---USER---row-offset---ACC---recipient)   (shift transaction/TO_ADDRESS_HI    tx-skip---USER---row-offset---TXN))
                        (eq!    (shift account/ADDRESS_LO           tx-skip---USER---row-offset---ACC---recipient)   (shift transaction/TO_ADDRESS_LO    tx-skip---USER---row-offset---TXN))
                        (account-increment-balance-by               tx-skip---USER---row-offset---ACC---recipient    (shift transaction/VALUE            tx-skip---USER---row-offset---TXN))
                        ;; (account-increment-nonce                       tx-skip---USER---row-offset---ACC---recipient)
                        ;; (account-same-code                             tx-skip---USER---row-offset---ACC---recipient)
                        ;; (account-same-deployment-number-and-status     tx-skip---USER---row-offset---ACC---recipient)
                        (account-conditionally-check-for-delegation tx-skip---USER---row-offset---ACC---recipient  (check-recipient-for-delegation))
                        (account-same-warmth                        tx-skip---USER---row-offset---ACC---recipient)
                        (account-same-marked-for-deletion           tx-skip---USER---row-offset---ACC---recipient)
                        (account-isnt-precompile                    tx-skip---USER---row-offset---ACC---recipient)
                        (DOM-SUB-stamps---standard                  tx-skip---USER---row-offset---ACC---recipient
                                                                    tx-skip---USER---row-offset---ACC---recipient)))

(defun  (check-recipient-for-delegation)  (*  (tx-skip---USER---is-message-call)
                                              (tx-skip---RCPT---has-nonempty-code)
                                              ))

;;-----------------------;;
;;   Message call case   ;;
;;-----------------------;;



(defconstraint   tx-skip---USER---recipient-account-row---trivial-message-calls---nonce-code-and-deployment-status
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero (tx-skip---USER---is-message-call)
                              ;; message_call ≡ 1 i.e. pure transfers
                              (begin    (account-same-nonce                          tx-skip---USER---row-offset---ACC---recipient)
                                        (account-same-code                           tx-skip---USER---row-offset---ACC---recipient)
                                        (account-same-deployment-number-and-status   tx-skip---USER---row-offset---ACC---recipient))))

;;---------------------;;
;;   Deployment case   ;;
;;---------------------;;



(defconstraint   tx-skip---USER---recipient-account-row---trivial-deployments---nonce
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (tx-skip---USER---is-deployment)
                                 ;; deployment ≡ 1 i.e. trivial deployments
                                 (begin  ;; nonce
                                   (account-increment-nonce           tx-skip---USER---row-offset---ACC---recipient)
                                   (vanishes! (shift account/NONCE    tx-skip---USER---row-offset---ACC---recipient)))))

(defconstraint   tx-skip---USER---recipient-account-row---trivial-deployments---code
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (tx-skip---USER---is-deployment)
                                 ;; deployment ≡ 1 i.e. trivial deployments
                                 (begin  ;; code
                                   ;; current code
                                   (vanishes!         (shift account/HAS_CODE           tx-skip---USER---row-offset---ACC---recipient))
                                   (debug    (eq!     (shift account/CODE_HASH_HI       tx-skip---USER---row-offset---ACC---recipient)    EMPTY_KECCAK_HI))
                                   (debug    (eq!     (shift account/CODE_HASH_LO       tx-skip---USER---row-offset---ACC---recipient)    EMPTY_KECCAK_LO))
                                   (vanishes!         (shift account/CODE_SIZE          tx-skip---USER---row-offset---ACC---recipient))
                                   ;; updated code
                                   (vanishes!         (shift account/HAS_CODE_NEW       tx-skip---USER---row-offset---ACC---recipient))
                                   (debug    (eq!     (shift account/CODE_HASH_HI_NEW   tx-skip---USER---row-offset---ACC---recipient)    EMPTY_KECCAK_HI))
                                   (debug    (eq!     (shift account/CODE_HASH_LO_NEW   tx-skip---USER---row-offset---ACC---recipient)    EMPTY_KECCAK_LO))
                                   (eq!               (shift account/CODE_SIZE_NEW      tx-skip---USER---row-offset---ACC---recipient)
                                                      (shift transaction/INIT_CODE_SIZE tx-skip---USER---row-offset---TXN))
                                   (debug (vanishes!  (shift account/CODE_SIZE_NEW      tx-skip---USER---row-offset---ACC---recipient))))))

(defconstraint   tx-skip---USER---recipient-account-row---trivial-deployments---deployment-status-and-number
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (tx-skip---USER---is-deployment)
                                 ;; deployment ≡ 1 i.e. trivial deployments
                                 (begin
                                   (account-increment-deployment-number                tx-skip---USER---row-offset---ACC---recipient)
                                   (debug (eq! (shift account/DEPLOYMENT_STATUS        tx-skip---USER---row-offset---ACC---recipient) 0))
                                   (eq!        (shift account/DEPLOYMENT_STATUS_NEW    tx-skip---USER---row-offset---ACC---recipient) 0))))

(defconstraint   tx-skip---USER---recipient-is-no-precompile
                 (:guard (tx-skip---precondition---USER))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (tx-skip---USER---is-message-call)
                                 ;; deployment ≡ 0 i.e. pure transfer
                                 (account-trim-address       tx-skip---USER---row-offset---ACC---recipient
                                                             (shift   transaction/TO_ADDRESS_HI   tx-skip---USER---row-offset---TXN)
                                                             (shift   transaction/TO_ADDRESS_LO   tx-skip---USER---row-offset---TXN))))
