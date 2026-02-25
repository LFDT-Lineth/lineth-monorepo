(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                            ;;
;;   X     TX_INIT phase                      ;;
;;   X.Y   Common constraints                 ;;
;;   X.Y.Z Recipient accepts value transfer   ;;
;;                                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-init---account-row---recipient-value-reception
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!     (shift account/ADDRESS_HI                        tx-init---row-offset---ACC---recipient-value-reception)     (tx-init---recipient-address-hi))
                   (eq!     (shift account/ADDRESS_LO                        tx-init---row-offset---ACC---recipient-value-reception)     (tx-init---recipient-address-lo))
                   (account-trim-address                                     tx-init---row-offset---ACC---recipient-value-reception
                                                                             (tx-init---recipient-address-hi)
                                                                             (tx-init---recipient-address-lo))
                   (account-increment-balance-by                             tx-init---row-offset---ACC---recipient-value-reception      (tx-init---value))
                   ;; (account-same-nonce                                    tx-init---row-offset---ACC---recipient-value-reception)
                   ;; (account-same-code                                     tx-init---row-offset---ACC---recipient-value-reception)
                   ;; (account-same-deployment-number-and-status             tx-init---row-offset---ACC---recipient-value-reception)
                   (account-check-for-delegation-if-account-has-code         tx-init---row-offset---ACC---recipient-value-reception)
                   (account-turn-on-warmth                                   tx-init---row-offset---ACC---recipient-value-reception)
                   (account-same-marked-for-deletion                         tx-init---row-offset---ACC---recipient-value-reception)
                   ;; (account-retrieve-code-fragment-index                     tx-init---row-offset---ACC---recipient-value-reception)
                   (account-isnt-precompile                                  tx-init---row-offset---ACC---recipient-value-reception)
                   (DOM-SUB-stamps---standard                                tx-init---row-offset---ACC---recipient-value-reception
                                                                             tx-init---row-offset---ACC---recipient-value-reception)
                   ))


(defun  (tx-init---delegate-address-hi)  (shift  account/DELEGATION_ADDRESS_HI  tx-init---row-offset---ACC---recipient-value-reception))
(defun  (tx-init---delegate-address-lo)  (shift  account/DELEGATION_ADDRESS_LO  tx-init---row-offset---ACC---recipient-value-reception))

(defconstraint    tx-init---account-row---recipient-value-reception---message-call
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (tx-init---is-message-call)
                                 (begin
                                   (account-same-nonce                            tx-init---row-offset---ACC---recipient-value-reception)
                                   (account-same-code                             tx-init---row-offset---ACC---recipient-value-reception)
                                   (account-same-deployment-number-and-status     tx-init---row-offset---ACC---recipient-value-reception))))

(defconstraint    tx-init---account-row---recipient-value-reception---deployment---nonce
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (tx-init---is-deployment)
                                 (begin
                                   (account-increment-nonce                tx-init---row-offset---ACC---recipient-value-reception)
                                   (vanishes!   (shift    account/NONCE    tx-init---row-offset---ACC---recipient-value-reception))
                                   )))

(defconstraint    tx-init---account-row---recipient-value-reception---deployment---code
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (tx-init---is-deployment)
                                 (begin
                                   ;; current code
                                   (vanishes!      (shift account/HAS_CODE           tx-init---row-offset---ACC---recipient-value-reception))
                                   (debug     (eq! (shift account/CODE_HASH_HI       tx-init---row-offset---ACC---recipient-value-reception) EMPTY_KECCAK_HI))
                                   (debug     (eq! (shift account/CODE_HASH_LO       tx-init---row-offset---ACC---recipient-value-reception) EMPTY_KECCAK_LO))
                                   (vanishes!      (shift account/CODE_SIZE          tx-init---row-offset---ACC---recipient-value-reception))
                                   ;; updated code
                                   (vanishes!      (shift account/HAS_CODE_NEW       tx-init---row-offset---ACC---recipient-value-reception))
                                   (debug     (eq! (shift account/CODE_HASH_HI_NEW   tx-init---row-offset---ACC---recipient-value-reception) EMPTY_KECCAK_HI))
                                   (debug     (eq! (shift account/CODE_HASH_LO_NEW   tx-init---row-offset---ACC---recipient-value-reception) EMPTY_KECCAK_LO))
                                   (eq!            (shift account/CODE_SIZE_NEW      tx-init---row-offset---ACC---recipient-value-reception)
                                                   (tx-init---init-code-size)))))

(defconstraint    tx-init---account-row---recipient-value-reception---deployment---deployment-number-and-status
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (tx-init---is-deployment)
                                 (begin
                                   ;; deployment
                                   (account-increment-deployment-number               tx-init---row-offset---ACC---recipient-value-reception)
                                   (eq!        (shift account/DEPLOYMENT_STATUS       tx-init---row-offset---ACC---recipient-value-reception) 0)
                                   (eq!        (shift account/DEPLOYMENT_STATUS_NEW   tx-init---row-offset---ACC---recipient-value-reception) 1))))

