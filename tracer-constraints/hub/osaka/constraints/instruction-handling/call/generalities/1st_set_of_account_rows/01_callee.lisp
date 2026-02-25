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
;;   callee account-row   ;;
;;------------------------;;


(defconstraint    call-instruction---1st-callee-account-operation
                  (:guard (call-instruction---summon-accounts-once-or-more))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    ;; account/ADDRESS_HI set implicitly by account-trim-address
                    (debug (eq!   (shift    account/TRM_RAW_ADDRESS_HI     CALL_1st_callee_account_row___row_offset)    (call-instruction---STACK-raw-callee-address-hi))) ;; set implicitly by account-trim-address
                    (debug (eq!   (shift    account/ADDRESS_LO             CALL_1st_callee_account_row___row_offset)    (call-instruction---STACK-raw-callee-address-lo))) ;; set implicitly by account-trim-address
                    ;; balance done below
                    (account-same-nonce                                    CALL_1st_callee_account_row___row_offset)
                    (account-same-code                                     CALL_1st_callee_account_row___row_offset)
                    (account-check-for-delegation-if-account-has-code      CALL_1st_callee_account_row___row_offset)
                    ;; warmth done below
                    (account-same-deployment-number-and-status             CALL_1st_callee_account_row___row_offset)
                    (account-same-marked-for-deletion                      CALL_1st_callee_account_row___row_offset)
                    (account-dont-trigger-ROM_LEX                          CALL_1st_callee_account_row___row_offset)
                    (account-trim-address                                  CALL_1st_callee_account_row___row_offset          ;; row offset
                                                                           (call-instruction---STACK-raw-callee-address-hi)  ;; high part of raw, potentially untrimmed address
                                                                           (call-instruction---STACK-raw-callee-address-lo)  ;; low  part of raw, potentially untrimmed address
                                                                           )
                    (debug (eq!   (shift    account/TRM_FLAG               CALL_1st_callee_account_row___row_offset)    1)) ;; set implicitly by account-trim-address
                    (vanishes!    (shift    account/RLPADDR_FLAG           CALL_1st_callee_account_row___row_offset))
                    (DOM-SUB-stamps---standard                             CALL_1st_callee_account_row___row_offset
                                                                           CALL_1st_callee_account_row___row_offset)
                    ))


(defconstraint    call-instruction---1st-callee-account-operation---balance-update
                  (:guard (call-instruction---summon-accounts-once-or-more))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not-zero    (scenario-shorthand---CALL---balance-update-not-required)
                                    (account-same-balance            CALL_1st_callee_account_row___row_offset))
                    (if-not-zero    (scenario-shorthand---CALL---balance-update-required)
                                    (account-increment-balance-by    CALL_1st_callee_account_row___row_offset
                                                                     (* (call-instruction---is-CALL) (call-instruction---STACK-value-lo))))))

(defconstraint    call-instruction---1st-callee-account-operation---warmth-update
                  (:guard (call-instruction---summon-accounts-once-or-more))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not-zero    (scenario-shorthand---CALL---callee-warmth-update-not-required)
                                    (account-same-warmth             CALL_1st_callee_account_row___row_offset))
                    (if-not-zero    (scenario-shorthand---CALL---callee-warmth-update-required)
                                    (account-turn-on-warmth          CALL_1st_callee_account_row___row_offset))))
