(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;   X   TX_INIT phase   ;;
;;   X.Y Introduction    ;;
;;   X.Y Shorthands      ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconst
  tx-init---row-offset---row-preceding-the-init-phase                        -1
  tx-init---row-offset---TXN                                                  0
  tx-init---row-offset---MISC                                                 1
  tx-init---row-offset---ACC---coinbase-warming                               2
  tx-init---row-offset---ACC---sender-pay-for-gas                             3
  tx-init---row-offset---ACC---sender-value-transfer                          4
  tx-init---row-offset---ACC---recipient-value-reception                      5
  tx-init---row-offset---ACC---delegate-reading                               6
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  tx-init---row-offset---CON---context-initialization-row---success           7
  tx-init---non-stack-rows---success                                          (+  tx-init---row-offset---CON---context-initialization-row---success  1)
  tx-init---row-offset---first-execution-phase-row---success                  (+  tx-init---row-offset---CON---context-initialization-row---success  1)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  tx-init---row-offset---ACC---sender-value-transfer---undoing                7
  tx-init---row-offset---ACC---recipient-value-reception---undoing            8
  tx-init---row-offset---CON---context-initialization-row---failure           9
  tx-init---non-stack-rows---failure                                          (+  tx-init---row-offset---CON---context-initialization-row---failure  1)
  tx-init---row-offset---first-execution-phase-row---failure                  (+  tx-init---row-offset---CON---context-initialization-row---failure  1)
  )



(defun    (tx-init---standard-precondition)             (*    (shift    (- 1 TX_INIT)    tx-init---row-offset---row-preceding-the-init-phase)    TX_INIT))
(defun    (tx-init---transaction-failure-prediction)    (shift    misc/CCSR_FLAG                 tx-init---row-offset---MISC))
(defun    (tx-init---transaction-success-prediction)    (-   1    (tx-init---transaction-failure-prediction)))
(defun    (tx-init---transaction-end-stamp)             (shift    misc/CCRS_STAMP                tx-init---row-offset---MISC))
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun    (tx-init---sender-address-hi)                 (shift    transaction/FROM_ADDRESS_HI    tx-init---row-offset---TXN))
(defun    (tx-init---sender-address-lo)                 (shift    transaction/FROM_ADDRESS_LO    tx-init---row-offset---TXN))
(defun    (tx-init---recipient-address-hi)              (shift    transaction/TO_ADDRESS_HI      tx-init---row-offset---TXN))
(defun    (tx-init---recipient-address-lo)              (shift    transaction/TO_ADDRESS_LO      tx-init---row-offset---TXN))
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun    (tx-init---is-deployment)                     (shift    transaction/IS_DEPLOYMENT      tx-init---row-offset---TXN))
(defun    (tx-init---is-message-call)                   (-   1    (tx-init---is-deployment)))
(defun    (tx-init---gas-cost)                          (shift    (*    transaction/GAS_LIMIT    transaction/GAS_PRICE)      tx-init---row-offset---TXN))
(defun    (tx-init---value)                             (shift    transaction/VALUE              tx-init---row-offset---TXN))
(defun    (tx-init---call-data-context-number)          (*     HUB_STAMP    (shift   transaction/COPY_TXCD    tx-init---row-offset---TXN)))
(defun    (tx-init---call-data-size)                    (shift    transaction/CALL_DATA_SIZE    tx-init---row-offset---TXN))
(defun    (tx-init---init-code-size)                    (shift    transaction/INIT_CODE_SIZE    tx-init---row-offset---TXN))
(defun    (tx-init---coinbase-address-hi)               (shift    transaction/COINBASE_ADDRESS_HI    tx-init---row-offset---TXN))
(defun    (tx-init---coinbase-address-lo)               (shift    transaction/COINBASE_ADDRESS_LO    tx-init---row-offset---TXN))


(defun    (tx-init---non-skip-message-call)                (+  (tx-init---RCPT---has-code-and-isnt-delegated)
                                                               (tx-init---DLGT---has-code)
                                                               ))
(defun    (tx-init---RCPT---has-code-and-isnt-delegated)   (*  (tx-init---RCPT---has-nonempty-code)
                                                               (tx-init---RCPT---isnt-delegated)
                                                               ))
(defun    (tx-init---DLGT---has-code)                      (*  (tx-init---RCPT---is-delegated)
                                                               (tx-init---DLGT---has-nonempty-code)
                                                               ;; (tx-init---DLGT---isnt-delegated)
                                                               ))

(defun    (tx-init---SNDR---has-nonempty-code)    (shift   account/HAS_CODE       tx-init---row-offset---ACC---sender-pay-for-gas        ) )
(defun    (tx-init---RCPT---has-nonempty-code)    (shift   account/HAS_CODE       tx-init---row-offset---ACC---recipient-value-reception ) )
(defun    (tx-init---DLGT---has-nonempty-code)    (shift   account/HAS_CODE       tx-init---row-offset---ACC---delegate-reading          ) )
;;
(defun    (tx-init---SNDR---is-delegated)         (shift   account/IS_DELEGATED   tx-init---row-offset---ACC---sender-pay-for-gas        ) )
(defun    (tx-init---RCPT---is-delegated)         (shift   account/IS_DELEGATED   tx-init---row-offset---ACC---recipient-value-reception ) )
(defun    (tx-init---DLGT---is-delegated)         (shift   account/IS_DELEGATED   tx-init---row-offset---ACC---delegate-reading          ) )

(defun    (tx-init---SNDR---has-empty-code)    (-  1  (tx-init---SNDR---has-nonempty-code) ) )
(defun    (tx-init---RCPT---has-empty-code)    (-  1  (tx-init---RCPT---has-nonempty-code) ) )
(defun    (tx-init---DLGT---has-empty-code)    (-  1  (tx-init---DLGT---has-nonempty-code) ) )
;;
(defun    (tx-init---SNDR---isnt-delegated)    (-  1  (tx-init---SNDR---is-delegated) ) )
(defun    (tx-init---RCPT---isnt-delegated)    (-  1  (tx-init---RCPT---is-delegated) ) )
(defun    (tx-init---DLGT---isnt-delegated)    (-  1  (tx-init---DLGT---is-delegated) ) )

