(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;    X.Y.Z Computations    ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; HUB row
;;;;;;;;;;

(defconstraint    EIP-4788---setting-the-timestamp
		  (:guard   (first-row-of-EIP-4788-transaction))
		  (eq!   (EIP-4788---timestamp)
			 (shift    hub/btc_TIMESTAMP   ROFF___SYSI___HUB_ROW)))


;; computing TIMESTAMP % 8191
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    EIP-4788---computing-the-timestamp-modulo-8191---EUC-call
		  (:guard   (first-row-of-EIP-4788-transaction))
		  (call-to-EUC   ROFF___EIP_4788___TIMESTAMP_MOD_8191
				 (EIP-4788---timestamp)
				 HISTORY_BUFFER_LENGTH))

(defconstraint    EIP-4788---computing-the-timestamp-modulo-8191---committing-to-SYST_TXN_DATA
		  (:guard   (first-row-of-EIP-4788-transaction))
		  (eq!   (EIP-4788---timestamp-mod-8191)
			 (shift   computation/EUC_REMAINDER   ROFF___EIP_4788___TIMESTAMP_MOD_8191)))

(defconstraint    EIP-4788---detecting-the-genesis-block---WCP-call
		  (:guard   (first-row-of-EIP-4788-transaction))
		  (small-call-to-ISZERO   ROFF___EIP_4788___DETECTING_THE_GENESIS_BLOCK
					  (EIP-4788---block-number)))

;; detecting the genesis block
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    EIP-4788---detecting-the-genesis-block---commiting-to-SYST_TXN_DATA
		  (:guard   (first-row-of-EIP-4788-transaction))
		  (eq!   (EIP-4788---current-block-is-genesis-block)
			 (shift   computation/WCP_RES   ROFF___EIP_4788___DETECTING_THE_GENESIS_BLOCK)))

(defconstraint    EIP-4788---detecting-the-genesis-block---enforcing-trivial-beacon-root-for-the-genesis-block
		  (:guard   (first-row-of-EIP-4788-transaction))
		  (if-not-zero   (EIP-4788---current-block-is-genesis-block)
				 (begin
				   (vanishes!  (EIP-4788---beaconroot-hi))
				   (vanishes!  (EIP-4788---beaconroot-lo)))))

