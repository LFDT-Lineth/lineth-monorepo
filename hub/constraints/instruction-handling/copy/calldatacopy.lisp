(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;    X.Y.4 Specifics for CALLDATACOPY   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint copy-instruction---CALLDATACOPY---setting-the-gas-cost (:guard (copy-instruction---standard-CALLDATACOPY))
               (begin (if-not-zero stack/MXPX
                                   (vanishes! GAS_COST))
                      (if-not-zero stack/OOGX
                                   (eq! GAS_COST (+ stack/STATIC_GAS (copy-instruction---MXP-memory-expansion-gas))))
                      (if-zero XAHOY
                               (eq! GAS_COST (+ stack/STATIC_GAS (copy-instruction---MXP-memory-expansion-gas))))))

(defconstraint copy-instruction---CALLDATACOPY---setting-context-row---exceptional-case (:guard (copy-instruction---standard-CALLDATACOPY))
               (if-not-zero XAHOY
                            (execution-provides-empty-return-data ROW_OFFSET_CALLDATACOPY_CONTEXT_ROW)
                            (read-context-data ROW_OFFSET_CALLDATACOPY_CONTEXT_ROW CONTEXT_NUMBER)))

