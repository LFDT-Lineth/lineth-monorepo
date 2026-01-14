(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;    X.Y.Z.3 First set of account rows    ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (call-instruction---summon-both-account-rows-twice-or-more)    (*    PEEK_AT_SCENARIO
                                                                               (scenario-shorthand---CALL---requires-both-accounts-twice)))

(defconstraint    call-instruction---2nd-caller-account-operation                     (:guard (call-instruction---summon-both-account-rows-twice-or-more))
                  (begin
                    (account-same-address-as                         CALL_2nd_caller_account_row___row_offset    CALL_1st_caller_account_row___row_offset)
                    (account-undo-balance-update                     CALL_2nd_caller_account_row___row_offset    CALL_1st_caller_account_row___row_offset)
                    (account-same-nonce                              CALL_2nd_caller_account_row___row_offset)
                    (account-same-code                               CALL_2nd_caller_account_row___row_offset)
                    (account-same-warmth                             CALL_2nd_caller_account_row___row_offset)
                    (account-same-deployment-number-and-status       CALL_2nd_caller_account_row___row_offset)
                    (account-same-marked-for-deletion                CALL_2nd_caller_account_row___row_offset)
                    (vanishes!    (shift    account/ROMLEX_FLAG      CALL_2nd_caller_account_row___row_offset))
                    (vanishes!    (shift    account/TRM_FLAG         CALL_2nd_caller_account_row___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CALL_2nd_caller_account_row___row_offset))
                    ;; DOM / SUB stamps set below
                    ))

(defconstraint    call-instruction---2nd-caller-account-operation---missing-fields    (:guard (call-instruction---summon-both-account-rows-twice-or-more))
                  (begin
                    (if-not-zero    (scenario-shorthand---CALL---balance-update-undone-with-callee-failure)
                                    (DOM-SUB-stamps---revert-with-child    CALL_2nd_caller_account_row___row_offset
                                                                           2
                                                                                 (call-instruction---callee-revert-stamp)))
                    (if-not-zero    (scenario-shorthand---CALL---balance-update-undone-with-caller-revert)
                                    (DOM-SUB-stamps---revert-with-current        CALL_2nd_caller_account_row___row_offset
                                                                                 2))
                    ))

;; second callee account encounter
(defconstraint    call-instruction---2nd-callee-account-operation                     (:guard (call-instruction---summon-both-account-rows-twice-or-more))
                  (begin
                    (account-same-address-as                         CALL_2nd_callee_account_row___row_offset    CALL_1st_callee_account_row___row_offset)
                    (account-undo-balance-update                     CALL_2nd_callee_account_row___row_offset    CALL_1st_callee_account_row___row_offset)
                    (account-same-nonce                              CALL_2nd_callee_account_row___row_offset)
                    (account-same-code                               CALL_2nd_callee_account_row___row_offset)
                    ;; warmth operation done below
                    (account-same-deployment-number-and-status       CALL_2nd_callee_account_row___row_offset)
                    (account-same-marked-for-deletion                CALL_2nd_callee_account_row___row_offset)
                    (vanishes!    (shift    account/ROMLEX_FLAG      CALL_2nd_callee_account_row___row_offset))
                    (vanishes!    (shift    account/TRM_FLAG         CALL_2nd_callee_account_row___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CALL_2nd_callee_account_row___row_offset))
                    ;; DOM / SUB stamps set below
                    ))

(defconstraint    call-instruction---2nd-callee-account-operation---missing-fields    (:guard (call-instruction---summon-both-account-rows-twice-or-more))
                  (begin
                    (if-not-zero    (scenario-shorthand---CALL---balance-update-undone-with-callee-failure)
                                    (begin    (DOM-SUB-stamps---revert-with-child    CALL_2nd_callee_account_row___row_offset    3    (call-instruction---callee-revert-stamp))
                                              (account-same-warmth                   CALL_2nd_callee_account_row___row_offset)))
                    (if-not-zero    (scenario-shorthand---CALL---balance-update-undone-with-caller-revert)
                                    (begin    (DOM-SUB-stamps---revert-with-current        CALL_2nd_callee_account_row___row_offset    3)
                                              (account-undo-warmth-update                  CALL_2nd_callee_account_row___row_offset    CALL_1st_callee_account_row___row_offset)))
                    ))
