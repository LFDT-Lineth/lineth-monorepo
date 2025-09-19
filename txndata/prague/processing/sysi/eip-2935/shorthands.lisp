(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;    X.Y.Z CT_MAX and CT constraints    ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defun   (EIP-2935---prev-block-number)                (shift    hub/SYST_TXN_DATA_1    ROFF___SYSI___HUB_ROW))
(defun   (EIP-2935---prev-block-number-mod-8191)       (shift    hub/SYST_TXN_DATA_2    ROFF___SYSI___HUB_ROW))
(defun   (EIP-2935---prev-block-hash-hi)               (shift    hub/SYST_TXN_DATA_3    ROFF___SYSI___HUB_ROW))
(defun   (EIP-2935---prev-block-hash-lo)               (shift    hub/SYST_TXN_DATA_4    ROFF___SYSI___HUB_ROW))
(defun   (EIP-2935---current-block-is-genesis-block)   (shift    hub/SYST_TXN_DATA_5    ROFF___SYSI___HUB_ROW))
(defun   (EIP-2935---block-number)                     (shift    hub/btc_BLOCK_NUMBER   ROFF___SYSI___HUB_ROW))

(defun   (first-row-of-EIP-2935-transaction)   (*   (first-row-of-SYSI-transaction)   (shift   hub/EIP_2935   ROFF___SYSI___HUB_ROW)))


(defconst
  ROFF___EIP_2935___DEFINING_THE_PREVIOUS_BLOCK_NUMBER                     1
  ROFF___EIP_2935___COMPUTING_THE_PREVIOUS-BLOCK_NUMBER_MOD_8191           2
  nROWS___EIP_2935                                                         3
  )
