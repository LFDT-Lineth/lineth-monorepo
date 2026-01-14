(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;    X.Y.6 Specifics for CODECOPY   ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    copy-instruction---CODECOPY---setting-the-gas-cost---MXPX-case
                  (:guard (copy-instruction---standard-CODECOPY))
                  (if-not-zero stack/MXPX (vanishes! GAS_COST)))

(defconstraint    copy-instruction---CODECOPY---setting-the-gas-cost---no-MXPX-case
                  (:guard (copy-instruction---standard-CODECOPY))
                  (if-zero     stack/MXPX (eq!       GAS_COST (+ stack/STATIC_GAS (copy-instruction---MXP-memory-expansion-gas)))))

(defconstraint    copy-instruction---CODECOPY---setting-the-context-row---exceptional-case
                  (:guard (copy-instruction---standard-CODECOPY))
                  (if-not-zero XAHOY
                               (execution-provides-empty-return-data ROFF_CODECOPY_XAHOY_CONTEXT_ROW)))

(defconstraint    copy-instruction---CODECOPY---setting-the-context-row-unexceptional-case
                  (:guard (copy-instruction---standard-CODECOPY))
                  (if-zero XAHOY
                           (read-context-data ROFF_CODECOPY_XAHOY_CONTEXT_ROW CONTEXT_NUMBER)))

(defconstraint    copy-instruction---CODECOPY---setting-the-account-row---unexceptional-case
                  (:guard (copy-instruction---standard-CODECOPY))
                  (if-zero XAHOY
                           (begin (eq! (shift account/ADDRESS_HI             ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW) (shift context/BYTE_CODE_ADDRESS_HI            ROFF_CODECOPY_NO_XAHOY_CONTEXT_ROW))
                                  (eq! (shift account/ADDRESS_LO             ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW) (shift context/BYTE_CODE_ADDRESS_LO            ROFF_CODECOPY_NO_XAHOY_CONTEXT_ROW))
                                  ;; deployment number
                                  ;; deployment status
                                  ;; ROM_LEX flag
                                  (account-same-balance                      ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW)
                                  (account-same-nonce                        ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW)
                                  (account-same-code                         ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW)
                                  (account-same-deployment-number-and-status ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW)
                                  (account-same-warmth                       ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW)
                                  (account-same-marked-for-deletion          ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW)
                                  (DOM-SUB-stamps---standard                 ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW 0))))

(defconstraint    copy-instruction---CODECOPY---consistency-constraints---debug
                  (:guard (copy-instruction---standard-CODECOPY))
                  (if-zero XAHOY
                           (begin
                             (eq!   (shift account/DEPLOYMENT_NUMBER      ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW) (shift context/BYTE_CODE_DEPLOYMENT_NUMBER     ROFF_CODECOPY_NO_XAHOY_CONTEXT_ROW))
                             (eq!   (shift account/DEPLOYMENT_STATUS      ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW) (shift context/BYTE_CODE_DEPLOYMENT_STATUS     ROFF_CODECOPY_NO_XAHOY_CONTEXT_ROW))
                             (eq!   (shift account/CODE_FRAGMENT_INDEX    ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW) (shift context/BYTE_CODE_CODE_FRAGMENT_INDEX   ROFF_CODECOPY_NO_XAHOY_CONTEXT_ROW))
                             (eq!   (shift account/CODE_FRAGMENT_INDEX    ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW) CODE_FRAGMENT_INDEX)
                             (account-retrieve-code-fragment-index        ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW))))
