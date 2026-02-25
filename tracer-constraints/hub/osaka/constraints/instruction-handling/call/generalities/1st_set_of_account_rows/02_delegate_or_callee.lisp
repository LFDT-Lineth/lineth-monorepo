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


;;------------------------------------;;
;;   delegate or callee account-row   ;;
;;------------------------------------;;


(defconstraint    call-instruction---1st-delegate-or-callee-account-operation
                  (:guard (call-instruction---summon-accounts-once-or-more))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!   (shift    account/ADDRESS_HI                    CALL_1st_delegt_account_row___row_offset)    (call-instruction---explicit-delegate-or-callee-address-hi))
                    (eq!   (shift    account/ADDRESS_LO                    CALL_1st_delegt_account_row___row_offset)    (call-instruction---explicit-delegate-or-callee-address-lo))
                    (account-same-balance                                  CALL_1st_delegt_account_row___row_offset)
                    (account-same-nonce                                    CALL_1st_delegt_account_row___row_offset)
                    (account-same-code                                     CALL_1st_delegt_account_row___row_offset)
                    (account-check-for-delegation-if-account-has-code      CALL_1st_delegt_account_row___row_offset)
                    ;; warmth set below
                    (account-same-deployment-number-and-status             CALL_1st_delegt_account_row___row_offset)
                    (account-same-marked-for-deletion                      CALL_1st_delegt_account_row___row_offset)
                    (account-conditionally-trigger-ROM_LEX                 CALL_1st_delegt_account_row___row_offset    (call-instruction---trigger_ROMLEX))
                    (account-trim-address                                  CALL_1st_delegt_account_row___row_offset                     ;; row offset
                                                                           (call-instruction---explicit-delegate-or-callee-address-hi)  ;; hi part of either delegate or callee address
                                                                           (call-instruction---explicit-delegate-or-callee-address-lo)  ;; lo part of either delegate or callee address
                                                                           )
                    (debug (eq!   (shift    account/TRM_FLAG               CALL_1st_delegt_account_row___row_offset)    1)) ;; set implicitly by account-trim-address
                    (vanishes!    (shift    account/RLPADDR_FLAG           CALL_1st_delegt_account_row___row_offset))
                    (DOM-SUB-stamps---standard                             CALL_1st_delegt_account_row___row_offset
                                                                           CALL_1st_delegt_account_row___row_offset)
                    ))


(defun   (call-instruction---explicit-delegate-or-callee-address-hi)  (if-zero   (call-instruction---callee---is-delegated        )
                                                                                 (call-instruction---callee---address-hi          )  ;; callee isn't delegated ≡ <true>
                                                                                 (call-instruction---callee---delegate-address-hi )  ;; callee is    delegated ≡ <true>
                                                                                 ))
(defun   (call-instruction---explicit-delegate-or-callee-address-lo)  (if-zero   (call-instruction---callee---is-delegated        )
                                                                                 (call-instruction---callee---address-lo          )  ;; callee isn't delegated ≡ <true>
                                                                                 (call-instruction---callee---delegate-address-lo )  ;; callee is    delegated ≡ <true>
                                                                                 ))

(defconstraint    call-instruction---1st-delegate-or-callee-account-operation---updating-warmth
                  (:guard (call-instruction---summon-accounts-once-or-more))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not-zero   (scenario-shorthand---CALL---callee-warmth-update-not-required)   (account-same-warmth      CALL_1st_delegt_account_row___row_offset))
                    (if-not-zero   (scenario-shorthand---CALL---callee-warmth-update-required)       (account-turn-on-warmth   CALL_1st_delegt_account_row___row_offset))
                    ))
