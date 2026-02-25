(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                    ;;
;;    X.Y.Z.3 Final context row for (unexceptional) aborted CALL's    ;;
;;                                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    call-instruction---unexceptional---aborted---WILL-revert---undoing-callee-warmth-update
                  (:guard (* PEEK_AT_SCENARIO scenario/CALL_ABORT_WILL_REVERT))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (account-same-address-as                         CALL_2nd_callee_account_row___abort_will_revert___row_offset    CALL_1st_callee_account_row___row_offset)  ;; we could use 1st instead of 2nd, too.
                    (account-same-balance                            CALL_2nd_callee_account_row___abort_will_revert___row_offset)
                    (account-same-nonce                              CALL_2nd_callee_account_row___abort_will_revert___row_offset)
                    (account-same-code                               CALL_2nd_callee_account_row___abort_will_revert___row_offset)
                    (account-dont-check-for-delegation               CALL_2nd_callee_account_row___abort_will_revert___row_offset)
                    (account-undo-warmth-update                      CALL_2nd_callee_account_row___abort_will_revert___row_offset    CALL_1st_callee_account_row___row_offset)
                    (account-same-deployment-number-and-status       CALL_2nd_callee_account_row___abort_will_revert___row_offset)
                    (account-same-marked-for-deletion                CALL_2nd_callee_account_row___abort_will_revert___row_offset)
                    (account-dont-trigger-ROM_LEX                    CALL_2nd_callee_account_row___abort_will_revert___row_offset)
                    (vanishes!    (shift    account/TRM_FLAG         CALL_2nd_callee_account_row___abort_will_revert___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CALL_2nd_callee_account_row___abort_will_revert___row_offset))
                    (DOM-SUB-stamps---revert-with-current            CALL_2nd_callee_account_row___abort_will_revert___row_offset
                                                                     CALL_2nd_callee_account_row___abort_will_revert___row_offset)
                    ))

(defconstraint    call-instruction---unexceptional---aborted---WILL-revert---undoing-delegt-warmth-update
                  (:guard (* PEEK_AT_SCENARIO scenario/CALL_ABORT_WILL_REVERT))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (account-same-address-as                         CALL_2nd_delegt_account_row___abort_will_revert___row_offset    CALL_1st_delegt_account_row___row_offset)  ;; we could use 1st instead of 2nd, too.
                    (account-same-balance                            CALL_2nd_delegt_account_row___abort_will_revert___row_offset)
                    (account-same-nonce                              CALL_2nd_delegt_account_row___abort_will_revert___row_offset)
                    (account-same-code                               CALL_2nd_delegt_account_row___abort_will_revert___row_offset)
                    (account-dont-check-for-delegation               CALL_2nd_delegt_account_row___abort_will_revert___row_offset)
                    (account-undo-warmth-update                      CALL_2nd_delegt_account_row___abort_will_revert___row_offset    CALL_1st_delegt_account_row___row_offset)
                    (account-same-deployment-number-and-status       CALL_2nd_delegt_account_row___abort_will_revert___row_offset)
                    (account-same-marked-for-deletion                CALL_2nd_delegt_account_row___abort_will_revert___row_offset)
                    (account-dont-trigger-ROM_LEX                    CALL_2nd_delegt_account_row___abort_will_revert___row_offset)
                    (vanishes!    (shift    account/TRM_FLAG         CALL_2nd_delegt_account_row___abort_will_revert___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CALL_2nd_delegt_account_row___abort_will_revert___row_offset))
                    (DOM-SUB-stamps---revert-with-current            CALL_2nd_delegt_account_row___abort_will_revert___row_offset
                                                                     CALL_2nd_delegt_account_row___abort_will_revert___row_offset)
                    ))

(defconstraint    call-instruction---unexceptional---aborted---WILL-revert---final-context-row
                  (:guard (* PEEK_AT_SCENARIO scenario/CALL_ABORT_WILL_REVERT))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (nonexecution-provides-empty-return-data    CALL_ABORT_WILL_REVERT---current-context-update---row-offset))


(defconstraint    call-instruction---unexceptional---aborted---WONT-revert---final-context-row
                  (:guard (* PEEK_AT_SCENARIO scenario/CALL_ABORT_WONT_REVERT))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (nonexecution-provides-empty-return-data    CALL_ABORT_WONT_REVERT---current-context-update---row-offset))
