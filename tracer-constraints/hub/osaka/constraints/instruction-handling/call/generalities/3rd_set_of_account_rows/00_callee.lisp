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
;;    X.Y.Z.T Third set of account-rows    ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;------------------------;;
;;   callee account-row   ;;
;;------------------------;;


(defconstraint    call-instruction---3rd-callee-account-operation
                  (:guard (call-instruction---summon-accounts-thrice))
                  (begin
                    (account-same-address-as                         CALL_3rd_callee_account_row___row_offset    CALL_2nd_callee_account_row___row_offset)  ;; we could use 1st instead of 2nd, too.
                    (account-same-balance                            CALL_3rd_callee_account_row___row_offset)
                    (account-same-nonce                              CALL_3rd_callee_account_row___row_offset)
                    (account-same-code                               CALL_3rd_callee_account_row___row_offset)
                    (account-dont-check-for-delegation               CALL_3rd_callee_account_row___row_offset)
                    (account-undo-warmth-update                      CALL_3rd_callee_account_row___row_offset    CALL_1st_callee_account_row___row_offset)
                    (account-same-deployment-number-and-status       CALL_3rd_callee_account_row___row_offset)
                    (account-same-marked-for-deletion                CALL_3rd_callee_account_row___row_offset)
                    (account-dont-trigger-ROM_LEX                    CALL_3rd_callee_account_row___row_offset)
                    (vanishes!    (shift    account/TRM_FLAG         CALL_3rd_callee_account_row___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CALL_3rd_callee_account_row___row_offset))
                    (DOM-SUB-stamps---revert-with-current            CALL_3rd_callee_account_row___row_offset
                                                                     CALL_3rd_callee_account_row___row_offset)
                    ))
