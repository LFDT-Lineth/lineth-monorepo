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
;;    X.Y.Z.5 Third set of account rows    ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (call-instruction---summon-callee-account-thrice)    (*    PEEK_AT_SCENARIO
                                                                     scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT))

(defconstraint    call-instruction---3rd-callee-account-operation
                  (:guard (call-instruction---summon-callee-account-thrice))
                  (begin
                    (account-same-address-as                         CALL_3rd_callee_account_row___row_offset    CALL_2nd_callee_account_row___row_offset)  ;; we could use 1st instead of 2nd, too.
                    (account-same-balance                            CALL_3rd_callee_account_row___row_offset)
                    (account-same-nonce                              CALL_3rd_callee_account_row___row_offset)
                    (account-same-code                               CALL_3rd_callee_account_row___row_offset)
                    (account-undo-warmth-update                      CALL_3rd_callee_account_row___row_offset    CALL_1st_callee_account_row___row_offset)
                    (account-same-deployment-number-and-status       CALL_3rd_callee_account_row___row_offset)
                    (account-same-marked-for-deletion                CALL_3rd_callee_account_row___row_offset)
                    (vanishes!    (shift    account/ROMLEX_FLAG      CALL_3rd_callee_account_row___row_offset))
                    (vanishes!    (shift    account/TRM_FLAG         CALL_3rd_callee_account_row___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CALL_3rd_callee_account_row___row_offset))
                    (DOM-SUB-stamps---revert-with-current            CALL_3rd_callee_account_row___row_offset
                                                                     4)
                    ))
