(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X.Y.7 Specifics for EXTCODECOPY   ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint copy-instruction---EXTCODECOPY---setting-the-gas-cost---MXPX-case (:guard (copy-instruction---standard-EXTCODECOPY))
               (if-not-zero stack/MXPX
                            (vanishes! GAS_COST)))

(defconstraint copy-instruction---EXTCODECOPY---setting-the-gas-cost---no-MXPX-case (:guard (copy-instruction---standard-EXTCODECOPY))
               (if-zero     stack/MXPX
                            (eq! GAS_COST
                                 (+ stack/STATIC_GAS
                                    (copy-instruction---MXP-memory-expansion-gas)
                                    (* (copy-instruction---foreign-address-warmth)       GAS_CONST_G_WARM_ACCESS)
                                    (* (- 1 (copy-instruction---foreign-address-warmth)) GAS_CONST_G_COLD_ACCOUNT_ACCESS)))))

;; ;; completely redundant constraint:
;; (defconstraint copy-instruction---EXTCODECOPY---the-MXPX-case (:guard (copy-instruction---standard-EXTCODECOPY))
;;                (begin
;;                  (debug
;;                    (if-not-zero stack/MXPX
;;                                 (execution-provides-empty-return-data    ROFF_EXTCODECOPY_MXPX_CONTEXT_ROW)))))

(defconstraint copy-instruction---EXTCODECOPY---the-OOGX-case (:guard (copy-instruction---standard-EXTCODECOPY))
               (if-not-zero stack/OOGX
                            (begin
                              ;; account-row i + 2
                              (account-trim-address                          ROFF_EXTCODECOPY_OOGX_ACCOUNT_ROW (copy-instruction---raw-address-hi) (copy-instruction---raw-address-lo))
                              (vanishes! (shift account/ROMLEX_FLAG          ROFF_EXTCODECOPY_OOGX_ACCOUNT_ROW))
                              (account-same-balance                          ROFF_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                              (account-same-nonce                            ROFF_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                              (account-same-code                             ROFF_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                              (account-same-deployment-number-and-status     ROFF_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                              (account-same-warmth                           ROFF_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                              (account-same-marked-for-deletion              ROFF_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                              (DOM-SUB-stamps---standard                     ROFF_EXTCODECOPY_OOGX_ACCOUNT_ROW 0)
                              ;; context-row i + 3: redundant constraint ahead
                              (debug (execution-provides-empty-return-data   ROFF_EXTCODECOPY_OOGX_CONTEXT_ROW)))))

(defun (copy-instruction---trigger-CFI)
  (* (copy-instruction---is-EXTCODECOPY)
     (copy-instruction---trigger_MMU)
     (copy-instruction---foreign-address-has-code)))

(defconstraint copy-instruction---unexceptional-reverted-EXTCODECOPY---doing-account-row (:guard (copy-instruction---standard-EXTCODECOPY))
               (if-not-zero (* (- 1 XAHOY) CONTEXT_WILL_REVERT)
                            (begin (account-trim-address                        ROFF_EXTCODECOPY_OOGX_ACCOUNT_ROW                   (copy-instruction---raw-address-hi) (copy-instruction---raw-address-lo))
                                   (eq! (shift account/ROMLEX_FLAG              ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW) (copy-instruction---trigger-CFI))
                                   (account-same-balance                        ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-same-nonce                          ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-same-code                           ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-same-deployment-number-and-status   ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-turn-on-warmth                      ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-same-marked-for-deletion            ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (DOM-SUB-stamps---standard                   ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW 0))))

(defconstraint copy-instruction---unexceptional-reverted-EXTCODECOPY---undoing-account-row (:guard (copy-instruction---standard-EXTCODECOPY))
               (if-not-zero (* (- 1 XAHOY) CONTEXT_WILL_REVERT)
                            (begin (account-same-address-as                       ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (debug (vanishes! (shift account/ROMLEX_FLAG   ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW)))
                                   (account-undo-balance-update                   ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-undo-nonce-update                     ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-undo-code-update                      ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-undo-deployment-status-update         ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-undo-warmth-update                    ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-same-marked-for-deletion              ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW)
                                   (DOM-SUB-stamps---revert-with-current          ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW 1))))

(defconstraint copy-instruction---unexceptional-unreverted-EXTCODECOPY-account-row (:guard (copy-instruction---standard-EXTCODECOPY))
               (if-not-zero (* (- 1 XAHOY) (- 1 CONTEXT_WILL_REVERT))
                            (begin (account-trim-address                        ROFF_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW  (copy-instruction---raw-address-hi) (copy-instruction---raw-address-lo))
                                   (eq! (shift   account/ROMLEX_FLAG            ROFF_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW) (copy-instruction---trigger-CFI))
                                   (account-same-balance                        ROFF_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                   (account-same-nonce                          ROFF_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                   (account-same-code                           ROFF_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                   (account-same-deployment-number-and-status   ROFF_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                   (account-turn-on-warmth                      ROFF_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                   (account-same-marked-for-deletion            ROFF_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                   (DOM-SUB-stamps---standard                   ROFF_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW 0))))

