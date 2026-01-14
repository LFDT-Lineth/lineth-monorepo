(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;    X.Y.4 Specifics for CALLDATACOPY   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint copy-instruction---CALLDATACOPY---setting-the-gas-cost---MXPX-case (:guard (copy-instruction---standard-CALLDATACOPY))
               (if-not-zero stack/MXPX
                            (vanishes! GAS_COST)))

(defconstraint copy-instruction---CALLDATACOPY---setting-the-gas-cost---no-MXPX-case (:guard (copy-instruction---standard-CALLDATACOPY))
               (if-zero     stack/MXPX
                            (eq!       GAS_COST (+ stack/STATIC_GAS (copy-instruction---MXP-memory-expansion-gas)))))


(defconstraint copy-instruction---CALLDATACOPY---setting-context-row---exceptional-case (:guard (copy-instruction---standard-CALLDATACOPY))
               (if-not-zero XAHOY
                            (execution-provides-empty-return-data ROFF_CALLDATACOPY_CONTEXT_ROW)
                            (read-context-data ROFF_CALLDATACOPY_CONTEXT_ROW CONTEXT_NUMBER)))

