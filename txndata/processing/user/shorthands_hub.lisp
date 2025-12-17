(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X. USER transaction processing    ;;
;;    X.Y HUB shorthands                ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun   (USER-transaction---HUB---basefee)                      (shift   hub/btc_BASEFEE                  ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---block-gas-limit)              (shift   hub/btc_BLOCK_GAS_LIMIT          ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---nonce)                        (shift   hub/NONCE                        ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---initial-balance)              (shift   hub/INIT_BALANCE                 ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---gas-limit)                    (shift   hub/GAS_LIMIT                    ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---gas-price)                    (shift   hub/GAS_PRICE                    ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---initial-gas)                  (shift   hub/GAS_INITIALLY_AVAILABLE      ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---value)                        (shift   hub/VALUE                        ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---is-deployment)                (shift   hub/IS_DEPLOYMENT                ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---call-data-size)               (shift   hub/CALL_DATA_SIZE               ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---init-code-size)               (shift   hub/INIT_CODE_SIZE               ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---has-eip-1559-gas-semantics)   (shift   hub/HAS_EIP_1559_GAS_SEMANTICS   ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---requires-EVM-execution)       (shift   hub/REQUIRES_EVM_EXECUTION       ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---copy-txcd)                    (shift   hub/COPY_TXCD                    ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---status-code)                  (shift   hub/STATUS_CODE                  ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---gas-leftover)                 (shift   hub/GAS_LEFTOVER                 ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---refund-counter-final)         (shift   hub/REFUND_COUNTER_FINAL         ROFF___USER___HUB_ROW))
(defun   (USER-transaction---HUB---refund-effective)             (shift   hub/REFUND_EFFECTIVE             ROFF___USER___HUB_ROW))
