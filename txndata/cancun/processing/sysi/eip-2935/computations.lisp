(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;    X.Y.Z CT_MAX and CT constraints    ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    EIP-2935---defining-the-previous-block-number---WCP-call
		  (:guard (first-row-of-EIP-2935-transaction))
		  (small-call-to-ISZERO    ROFF___EIP_2935___DEFINING_THE_PREVIOUS_BLOCK_NUMBER
					   (EIP-2935---block-number)))

(defconstraint    EIP-2935---defining-the-previous-block-number---committing-to-SYST_TXN_DATA---I
		  (:guard (first-row-of-EIP-2935-transaction))
		  (eq!    (EIP-2935---current-block-is-genesis-block)
			  (shift    computation/WCP_RES    ROFF___EIP_2935___DEFINING_THE_PREVIOUS_BLOCK_NUMBER)))

(defconstraint    EIP-2935---defining-the-previous-block-number---committing-to-SYST_TXN_DATA---II
		  (:guard (first-row-of-EIP-2935-transaction))
		  (if-not-zero    (force-bin   (EIP-2935---current-block-is-genesis-block))
				  (eq!   (EIP-2935---prev-block-number)   0)                                ;; <true>  case
				  (eq!   (EIP-2935---prev-block-number)   (- (EIP-2935---block-number) 1))  ;; <false> case
				  ))

(defconstraint    EIP-2935---computing-the-previous-block-number-modulo-8191---EUC-call
		  (:guard (first-row-of-EIP-2935-transaction))
		  (call-to-EUC    ROFF___EIP_2935___COMPUTING_THE_PREVIOUS-BLOCK_NUMBER_MOD_8191
				  (EIP-2935---prev-block-number)
				  HISTORY_SERVE_WINDOW))

(defconstraint    EIP-2935---computing-the-previous-block-number-modulo-8191---committing-to-SYST_TXN_DATA
		  (:guard (first-row-of-EIP-2935-transaction))
		  (eq!    (EIP-2935---prev-block-number-mod-8191)
			  (shift   computation/EUC_REMAINDER   ROFF___EIP_2935___COMPUTING_THE_PREVIOUS-BLOCK_NUMBER_MOD_8191)))

