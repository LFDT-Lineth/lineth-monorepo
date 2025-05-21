(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;   X   TX_INIT phase   ;;
;;   X.Y Introduction    ;;
;;   X.Y Shorthands      ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconst
  tx-init---row-offset---row-preceding-the-init-phase                  -1
  tx-init---row-offset---MISC                                           0
  tx-init---row-offset---TXN                                            1
  tx-init---row-offset---ACC---sender-pay-for-gas                       2
  tx-init---row-offset---ACC---sender-value-transfer                    3
  tx-init---row-offset---ACC---recipient-value-reception                4
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  tx-init---row-offset---CON---context-initialization-row---success     5
  tx-init---row-offset---first-execution-phase-row---success            6
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  tx-init---row-offset---ACC---sender-value-transfer---undoing          5
  tx-init---row-offset---ACC---recipient-value-reception---undoing      6
  tx-init---row-offset---CON---context-initialization-row---failure     7
  tx-init---row-offset---first-execution-phase-row---failure            8
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
