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
;;    X.Y.Z.T First set of account-rows    ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;------------------------;;
;;   caller account-row   ;;
;;------------------------;;


(defconstraint    call-instruction---1st-caller-account-operation
                  (:guard (call-instruction---summon-accounts-once-or-more))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!    (shift    account/ADDRESS_HI    CALL_1st_caller_account_row___row_offset)    (call-instruction---current-frame---account-address-hi))
                    (eq!    (shift    account/ADDRESS_LO    CALL_1st_caller_account_row___row_offset)    (call-instruction---current-frame---account-address-lo))
                    ;; balance done below
                    (account-same-nonce                              CALL_1st_caller_account_row___row_offset)
                    (account-same-code                               CALL_1st_caller_account_row___row_offset)
                    (account-dont-check-for-delegation               CALL_1st_caller_account_row___row_offset)
                    (account-same-warmth                             CALL_1st_caller_account_row___row_offset)
                    (account-same-deployment-number-and-status       CALL_1st_caller_account_row___row_offset)
                    (account-same-marked-for-deletion                CALL_1st_caller_account_row___row_offset)
                    (account-dont-trigger-ROM_LEX                    CALL_1st_caller_account_row___row_offset)
                    (vanishes!    (shift    account/ROMLEX_FLAG      CALL_1st_caller_account_row___row_offset))
                    (vanishes!    (shift    account/TRM_FLAG         CALL_1st_caller_account_row___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CALL_1st_caller_account_row___row_offset))
                    (DOM-SUB-stamps---standard                       CALL_1st_caller_account_row___row_offset
                                                                     CALL_1st_caller_account_row___row_offset)
                    ))

(defconstraint    call-instruction---1st-caller-account-operation---balance-update
                  (:guard (call-instruction---summon-accounts-once-or-more))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not-zero    (scenario-shorthand---CALL---balance-update-not-required)
                                    (account-same-balance            CALL_1st_caller_account_row___row_offset))
                    (if-not-zero    (scenario-shorthand---CALL---balance-update-required)
                                    (account-decrement-balance-by    CALL_1st_caller_account_row___row_offset
                                                                     (* (call-instruction---is-CALL) (call-instruction---STACK-value-lo))))))

