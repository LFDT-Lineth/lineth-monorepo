(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;   X   TX_FINL phase   ;;
;;   X.Y Introduction    ;;
;;   X.Y Shorthands      ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconst
  tx-finl---row-offset---row-preceding-the-finl-phase   -1
  tx-finl---row-offset---TXN                             0
  tx-finl---row-offset---ACC---sender-gas-refund         1
  tx-finl---row-offset---ACC---coinbase-reward           2
  tx-finl---row-offset---CON---final-zero-context        3
  tx-finl---NSR                                          4
  )



(defun    (tx-finl---standard-precondition)   (*       (shift   TX_EXEC               tx-finl---row-offset---row-preceding-the-finl-phase)   TX_FINL))
(defun    (tx-finl---transaction-success)     (-   1   (shift   CONTEXT_WILL_REVERT   tx-finl---row-offset---row-preceding-the-finl-phase)))
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun    (tx-finl---sender-address-hi)       (shift    transaction/FROM_ADDRESS_HI        tx-finl---row-offset---TXN))
(defun    (tx-finl---sender-address-lo)       (shift    transaction/FROM_ADDRESS_LO        tx-finl---row-offset---TXN))
(defun    (tx-finl---coinbase-address-hi)     (shift    transaction/COINBASE_ADDRESS_HI    tx-finl---row-offset---TXN))
(defun    (tx-finl---coinbase-address-lo)     (shift    transaction/COINBASE_ADDRESS_LO    tx-finl---row-offset---TXN))
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun    (tx-finl---effective-gas-price)     (shift    transaction/GAS_PRICE              tx-finl---row-offset---TXN))
(defun    (tx-finl---effective-gas-refund)    (shift    transaction/REFUND_EFFECTIVE       tx-finl---row-offset---TXN))
(defun    (tx-finl---sender-gas-refund)       (*        (tx-finl---effective-gas-price)    (tx-finl---effective-gas-refund)))
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun    (tx-finl---priority-fee-per-gas)    (shift    transaction/PRIORITY_FEE_PER_GAS   tx-finl---row-offset---TXN))
(defun    (tx-finl---gas-limit)               (shift    transaction/GAS_LIMIT              tx-finl---row-offset---TXN))
(defun    (tx-finl---coinbase-reward)         (*        (-   (tx-finl---gas-limit)   (tx-finl---effective-gas-refund))    (tx-finl---priority-fee-per-gas)))
