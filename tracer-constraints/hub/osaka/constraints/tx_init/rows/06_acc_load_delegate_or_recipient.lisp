(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;   X     TX_INIT phase                   ;;
;;   X.Y   Common constraints              ;;
;;   X.Y.Z Delegate or recipient reading   ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-init---account-row---delegate-or-recipient-account-reading-1
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!     (shift account/ADDRESS_HI                        tx-init---row-offset---ACC---delegate-reading)     (tx-init---delegate-or-recipient-address-hi))
                   (eq!     (shift account/ADDRESS_LO                        tx-init---row-offset---ACC---delegate-reading)     (tx-init---delegate-or-recipient-address-lo))
                   (account-trim-address                                     tx-init---row-offset---ACC---delegate-reading
                                                                             (tx-init---delegate-or-recipient-address-hi)
                                                                             (tx-init---delegate-or-recipient-address-lo))
                   (account-same-balance                                     tx-init---row-offset---ACC---delegate-reading)
                   (account-same-nonce                                       tx-init---row-offset---ACC---delegate-reading)
                   (account-same-code                                        tx-init---row-offset---ACC---delegate-reading)
                   (account-same-deployment-number-and-status                tx-init---row-offset---ACC---delegate-reading)
                   (account-check-for-delegation-if-account-has-code         tx-init---row-offset---ACC---delegate-reading)
                   (account-turn-on-warmth                                   tx-init---row-offset---ACC---delegate-reading)
                   (account-same-marked-for-deletion                         tx-init---row-offset---ACC---delegate-reading)
                   ;; (account-isnt-precompile                                  tx-init---row-offset---ACC---delegate-reading)
                   (DOM-SUB-stamps---standard                                tx-init---row-offset---ACC---delegate-reading
                                                                             tx-init---row-offset---ACC---delegate-reading)
                   (account-retrieve-code-fragment-index                     tx-init---row-offset---ACC---delegate-reading)
                   ))

(defun   (tx-init---delegate-or-recipient-address-hi)   (if-not-zero   (tx-init---is-message-call)
                                                                       ;; message call case
                                                                       (if-not-zero   (tx-init---RCPT---is-delegated)
                                                                                      (tx-init---delegate-address-hi)  ;; recipient IS_DELEGATED ≡ 1
                                                                                      (tx-init---recipient-address-hi) ;; recipient IS_DELEGATED ≡ 0
                                                                                      )
                                                                       ;; deployment case
                                                                       (tx-init---recipient-address-hi)
                                                                       ))

(defun   (tx-init---delegate-or-recipient-address-lo)   (if-not-zero   (tx-init---is-message-call)
                                                                       ;; message call case
                                                                       (if-not-zero   (tx-init---RCPT---is-delegated)
                                                                                      (tx-init---delegate-address-lo)  ;; recipient IS_DELEGATED ≡ 1
                                                                                      (tx-init---recipient-address-lo) ;; recipient IS_DELEGATED ≡ 0
                                                                                      )
                                                                       ;; deployment case
                                                                       (tx-init---recipient-address-lo)
                                                                       ))

