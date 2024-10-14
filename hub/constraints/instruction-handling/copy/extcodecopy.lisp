(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X.Y.7 Specifics for EXTCODECOPY   ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint copy-instruction---EXTCODECOPY---setting-the-gas-cost (:guard (copy-instruction---standard-EXTCODECOPY))
               (if-not-zero stack/MXPX
                            ;; MXPX ≡ 1
                            (vanishes! GAS_COST)
                            ;; MXPX ≡ 0
                            (eq! GAS_COST
                                 (+ stack/STATIC_GAS
                                    (copy-instruction---MXP-memory-expansion-gas)
                                    (* (copy-instruction---exo-address-warmth) GAS_CONST_G_WARM_ACCESS)
                                    (* (- 1 (copy-instruction---exo-address-warmth)) GAS_CONST_G_COLD_ACCOUNT_ACCESS)))))

(defconstraint copy-instruction---EXTCODECOPY---the-MXPX-case (:guard (copy-instruction---standard-EXTCODECOPY))
               (if-not-zero stack/MXPX
                            (execution-provides-empty-return-data ROW_OFFSET_EXTCODECOPY_MXPX_CONTEXT_ROW)))

(defconstraint copy-instruction---EXTCODECOPY---the-OOGX-case (:guard (copy-instruction---standard-EXTCODECOPY))
               (if-not-zero stack/OOGX
                            ;; account-row i + 2
                            (begin (account-trim-address                        ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW (copy-instruction---raw-address-hi) (copy-instruction---raw-address-lo))
                                   (vanishes! (shift account/ROMLEX_FLAG        ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW))
                                   (account-same-balance                        ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                                   (account-same-nonce                          ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                                   (account-same-code                           ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                                   (account-same-deployment-number-and-status   ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                                   (account-same-warmth                         ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                                   (account-same-marked-for-selfdestruct        ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                                   (DOM-SUB-stamps---standard                   ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW 0))
                            ;; context-row i + 3
                            (execution-provides-empty-return-data               ROW_OFFSET_EXTCODECOPY_OOGX_CONTEXT_ROW)))

(defun (copy-instruction---trigger-CFI)
  (* (copy-instruction---is-EXTCODECOPY)
     (copy-instruction---trigger_MMU)
     (copy-instruction---exo-address-has-code)))

(defconstraint copy-instruction---unexceptional-reverted-EXTCODECOPY---doing-account-row (:guard (copy-instruction---standard-EXTCODECOPY))
               (if-not-zero (* (- 1 XAHOY) CONTEXT_WILL_REVERT)
                            (begin (account-trim-address                        ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW                   (copy-instruction---raw-address-hi) (copy-instruction---raw-address-lo))
                                   (eq! (shift account/ROMLEX_FLAG              ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW) (copy-instruction---trigger-CFI))
                                   (account-same-balance                        ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-same-nonce                          ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-same-code                           ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-same-deployment-number-and-status   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-turn-on-warmth                      ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-same-marked-for-selfdestruct        ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (DOM-SUB-stamps---standard                   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW 0))))

(defconstraint copy-instruction---unexceptional-reverted-EXTCODECOPY---undoing-account-row (:guard (copy-instruction---standard-EXTCODECOPY))
               (if-not-zero (* (- 1 XAHOY) CONTEXT_WILL_REVERT)
                            (begin (account-same-address-as                       ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (debug (vanishes! (shift account/ROMLEX_FLAG   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW)))
                                   (account-undo-balance-update                   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-undo-nonce-update                     ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-undo-code-update                      ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-undo-deployment-status-update         ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-undo-warmth-update                    ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                   (account-same-marked-for-selfdestruct          ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW)
                                   (DOM-SUB-stamps---revert-with-current          ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW 1))))

(defconstraint copy-instruction---unexceptional-unreverted-EXTCODECOPY-account-row (:guard (copy-instruction---standard-EXTCODECOPY))
               (if-not-zero (* (- 1 XAHOY) (- 1 CONTEXT_WILL_REVERT))
                            (begin (account-trim-address                        ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW  (copy-instruction---raw-address-hi) (copy-instruction---raw-address-lo))
                                   (eq! (shift   account/ROMLEX_FLAG            ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW) (copy-instruction---trigger-CFI))
                                   (account-same-balance                        ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                   (account-same-nonce                          ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                   (account-same-code                           ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                   (account-same-deployment-number-and-status   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                   (account-turn-on-warmth                      ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                   (account-same-marked-for-selfdestruct        ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                   (DOM-SUB-stamps---standard                   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW 0))))

