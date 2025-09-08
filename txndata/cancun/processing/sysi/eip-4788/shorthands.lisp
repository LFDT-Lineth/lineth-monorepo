(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;    X.Y.Z CT_MAX and CT constraints    ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (EIP-4788---timestamp)                        (shift  [ hub/SYST_TXN_DATA 1 ]  ROFF___SYSI___HUB_ROW)) ;; ""
(defun   (EIP-4788---timestamp-mod-8191)               (shift  [ hub/SYST_TXN_DATA 2 ]  ROFF___SYSI___HUB_ROW)) ;; ""
(defun   (EIP-4788---beaconroot-hi)                    (shift  [ hub/SYST_TXN_DATA 3 ]  ROFF___SYSI___HUB_ROW)) ;; ""
(defun   (EIP-4788---beaconroot-lo)                    (shift  [ hub/SYST_TXN_DATA 4 ]  ROFF___SYSI___HUB_ROW)) ;; ""
(defun   (EIP-4788---current-block-is-genesis-block)   (shift  [ hub/SYST_TXN_DATA 5 ]  ROFF___SYSI___HUB_ROW)) ;; ""
(defun   (EIP-4788---block-number)                     (shift    hub/btc_BLOCK_NUMBER   ROFF___SYSI___HUB_ROW)) ;; ""

(defun   (first-row-of-EIP-4788-transaction)   (*   (first-row-of-SYSI-transaction)   (shift  hub/EIP_4788   ROFF___SYSI___HUB_ROW)))

(defconst
  ROFF___EIP_4788___TIMESTAMP_MOD_8191                                     1
  ROFF___EIP_4788___DETECTING_THE_GENESIS_BLOCK                            2
  ROFF___EIP_4788___COMPARING_THE_TIMESTAMP_TO_LINEA_CANCUN_FORK_TIMESTAMP 3
  )
