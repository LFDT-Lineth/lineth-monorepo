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
;;   caller account-row   ;;
;;------------------------;;


(defconstraint    call-instruction---2nd-caller-account-operation
                  (:guard (call-instruction---summon-accounts-twice-or-more))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (account-same-address-as                         CALL_2nd_caller_account_row___row_offset    CALL_1st_caller_account_row___row_offset)
                    (account-undo-balance-update                     CALL_2nd_caller_account_row___row_offset    CALL_1st_caller_account_row___row_offset)
                    (account-same-nonce                              CALL_2nd_caller_account_row___row_offset)
                    (account-same-code                               CALL_2nd_caller_account_row___row_offset)
                    (account-dont-check-for-delegation               CALL_2nd_caller_account_row___row_offset)
                    (account-same-warmth                             CALL_2nd_caller_account_row___row_offset)
                    (account-same-deployment-number-and-status       CALL_2nd_caller_account_row___row_offset)
                    (account-same-marked-for-deletion                CALL_2nd_caller_account_row___row_offset)
                    (vanishes!    (shift    account/ROMLEX_FLAG      CALL_2nd_caller_account_row___row_offset))
                    (vanishes!    (shift    account/TRM_FLAG         CALL_2nd_caller_account_row___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CALL_2nd_caller_account_row___row_offset))
                    ;; DOM / SUB stamps set below
                    ))

(defconstraint    call-instruction---2nd-caller-account-operation---missing-fields
                  (:guard (call-instruction---summon-accounts-twice-or-more))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not-zero    (scenario-shorthand---CALL---balance-update-undone-with-callee-failure)
                                    (DOM-SUB-stamps---revert-with-child      CALL_2nd_caller_account_row___row_offset
                                                                             CALL_2nd_caller_account_row___row_offset
                                                                             (call-instruction---callee---revert-stamp)))
                    (if-not-zero    (scenario-shorthand---CALL---balance-update-undone-with-caller-revert)
                                    (DOM-SUB-stamps---revert-with-current    CALL_2nd_caller_account_row___row_offset
                                                                             CALL_2nd_caller_account_row___row_offset))
                    ))
