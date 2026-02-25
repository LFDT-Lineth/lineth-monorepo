(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;    X.Y.Z.T Second set of account-rows    ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;------------------------;;
;;   callee account-row   ;;
;;------------------------;;


(defconstraint    call-instruction---2nd-callee-account-operation
                  (:guard (call-instruction---summon-accounts-twice-or-more))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (account-same-address-as                         CALL_2nd_callee_account_row___row_offset    CALL_1st_callee_account_row___row_offset)
                    (account-undo-balance-update                     CALL_2nd_callee_account_row___row_offset    CALL_1st_callee_account_row___row_offset)
                    (account-same-nonce                              CALL_2nd_callee_account_row___row_offset)
                    (account-same-code                               CALL_2nd_callee_account_row___row_offset)
                    (account-dont-check-for-delegation               CALL_2nd_callee_account_row___row_offset)
                    ;; warmth operation done below
                    (account-same-deployment-number-and-status       CALL_2nd_callee_account_row___row_offset)
                    (account-same-marked-for-deletion                CALL_2nd_callee_account_row___row_offset)
                    (account-dont-trigger-ROM_LEX                    CALL_2nd_callee_account_row___row_offset)
                    (vanishes!    (shift    account/TRM_FLAG         CALL_2nd_callee_account_row___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CALL_2nd_callee_account_row___row_offset))
                    ;; DOM / SUB stamps set below
                    ))

(defconstraint    call-instruction---2nd-callee-account-operation---undoing-warmth-update-and-setting-dom-sub-stamps
                  (:guard (call-instruction---summon-accounts-twice-or-more))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not-zero    (scenario-shorthand---CALL---balance-update-undone-with-callee-failure)
                                    (begin    (DOM-SUB-stamps---revert-with-child    CALL_2nd_callee_account_row___row_offset
                                                                                     CALL_2nd_callee_account_row___row_offset
                                                                                     (call-instruction---callee---revert-stamp))
                                              (account-same-warmth                   CALL_2nd_callee_account_row___row_offset)))
                    (if-not-zero    (scenario-shorthand---CALL---balance-update-undone-with-caller-revert)
                                    (begin    (DOM-SUB-stamps---revert-with-current        CALL_2nd_callee_account_row___row_offset
                                                                                           CALL_2nd_callee_account_row___row_offset)
                                              (account-undo-warmth-update                  CALL_2nd_callee_account_row___row_offset
                                                                                           CALL_1st_callee_account_row___row_offset)))
                    ))
