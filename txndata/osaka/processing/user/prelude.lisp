(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X. USER transaction processing    ;;
;;    X.Y Prelude                       ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (first-row-of-USER-transaction)                                (force-bin    (*   (- TOTL_TXN_NUMBER (prev TOTL_TXN_NUMBER))   USER)))
(defun    (first-row-of-USER-transaction-with-EIP-1559-gas-semantics)    (force-bin    (*   (first-row-of-USER-transaction)
											    (USER-transaction---HUB---has-eip-1559-gas-semantics))))


(defconstraint    USER-transaction-processing---prelude---setting-the-first-few-perspective-flags
		  (:guard    (first-row-of-USER-transaction))
		  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
		  (eq!    (+    (shift    HUB      ROFF___USER___HUB_ROW )
				(shift    RLP      ROFF___USER___RLP_ROW )
				(shift    CMPTN    ROFF___USER___CMPTN_ROW___EIP-2681___MAX_NONCE_UPPER_BOUND_CHECK))
			  3))

(defun   (ct-max-USER-sum)
  (+   (*   (-  nROWS___TRANSACTION___SANS___EIP_1559_GAS_SEMANTICS   1)   (shift   rlp/TYPE_0   ROFF___USER___RLP_ROW))
       (*   (-  nROWS___TRANSACTION___SANS___EIP_1559_GAS_SEMANTICS   1)   (shift   rlp/TYPE_1   ROFF___USER___RLP_ROW))
       (*   (-  nROWS___TRANSACTION___WITH___EIP_1559_GAS_SEMANTICS   1)   (shift   rlp/TYPE_2   ROFF___USER___RLP_ROW))
       (*   (-  nROWS___TRANSACTION___WITH___EIP_1559_GAS_SEMANTICS   1)   (shift   rlp/TYPE_3   ROFF___USER___RLP_ROW))
       (*   (-  nROWS___TRANSACTION___WITH___EIP_1559_GAS_SEMANTICS   1)   (shift   rlp/TYPE_4   ROFF___USER___RLP_ROW))
       ))


(defconstraint    USER-transaction-processing---prelude---setting-CT_MAX
		  (:guard    (first-row-of-USER-transaction))
		  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
		  (eq!   CT_MAX   (ct-max-USER-sum)))
