(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                               ;;;;
;;;;    X.5 Instruction handling   ;;;;
;;;;                               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                            ;;
;;    X.5 Instructions raising the ACC_FLAG   ;;
;;                                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;    X.5.1 Supported instructions and flags   ;;
;;    X.5.2 Shorthands                         ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (account-instruction-requires-trimming)        (+ [ stack/DEC_FLAG 1 ]
                                                         [ stack/DEC_FLAG 2 ]
                                                         [ stack/DEC_FLAG 3 ]))
(defun (account-instruction-no-trimming)              (- stack/ACC_FLAG (account-instruction-requires-trimming)))
(defun (account-instruction-raw-address-hi)           [ stack/STACK_ITEM_VALUE_HI 1 ])
(defun (account-instruction-raw-address-lo)           [ stack/STACK_ITEM_VALUE_LO 1 ])
(defun (account-instruction-account-address-hi)       context/ACCOUNT_ADDRESS_HI)
(defun (account-instruction-account-address-lo)       context/ACCOUNT_ADDRESS_LO)
(defun (account-instruction-byte-code-address-hi)     context/BYTE_CODE_ADDRESS_HI)
(defun (account-instruction-byte-code-address-lo)     context/BYTE_CODE_ADDRESS_LO)
(defun (account-instruction-address-warmth)           account/WARMTH)
(defun (account-instruction-decoded-flags-sum)        (+ [ stack/DEC_FLAG 1 ]
                                                         [ stack/DEC_FLAG 2 ]
                                                         [ stack/DEC_FLAG 3 ]
                                                         [ stack/DEC_FLAG 4 ]))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.5.3 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (account-instruction-standard-hypothesis) (* PEEK_AT_STACK
                                                    stack/ACC_FLAG
                                                    (- 1 stack/SUX stack/SOX)))

(defun (account-instruction-unexceptional) (* (account-instruction-standard-hypothesis)
                                              (- 1 XAHOY)))

(defconstraint   account-instruction-setting-the-stack-pattern                              (:guard (account-instruction-standard-hypothesis))
                 (begin
                   (if-not-zero (account-instruction-requires-trimming) (stack-pattern-1-1))
                   (if-not-zero (account-instruction-no-trimming)       (stack-pattern-0-1))))

(defconstraint   account-instruction-setting-allowable-exceptions                           (:guard (account-instruction-standard-hypothesis))
                 (eq! XAHOY stack/OOGX))

(defconstraint   account-instruction-setting-NSR                                            (:guard (account-instruction-standard-hypothesis))
                 (begin
                   (if-not-zero (account-instruction-requires-trimming)
                                (eq! NSR (+ 1 CONTEXT_WILL_REVERT CMC)))
                   (if-not-zero (account-instruction-no-trimming)
                                (eq! NSR (+ 1 (- 1 CMC))))
                   (debug (eq! XAHOY CMC))
                   (debug (eq! XAHOY stack/OOGX))))

(defconstraint   account-instruction-setting-peeking-flags                                  (:guard (account-instruction-standard-hypothesis))
                 (begin
                   (if-not-zero (account-instruction-requires-trimming)
                                (if-zero CONTEXT_WILL_REVERT
                                         (eq! NSR
                                              (+        (shift PEEK_AT_ACCOUNT 1)
                                                 (* CMC (shift PEEK_AT_CONTEXT 2))))
                                         (eq! NSR
                                              (+        (shift PEEK_AT_ACCOUNT 1)
                                                        (shift PEEK_AT_ACCOUNT 2)
                                                 (* CMC (shift PEEK_AT_CONTEXT 3))))))
                   (if-not-zero (account-instruction-no-trimming)
                                (if-zero XAHOY
                                         (eq! NSR
                                              (+ (shift PEEK_AT_CONTEXT 1)
                                                 (shift PEEK_AT_ACCOUNT 2)))
                                         (eq! NSR
                                              (shift PEEK_AT_CONTEXT 1))))))

(defconstraint   account-instruction-setting-gas-cost                                       (:guard (account-instruction-standard-hypothesis))
                 (begin
                   (if-not-zero (account-instruction-requires-trimming)
                                (eq! GAS_COST
                                     (+ (*      (account-instruction-address-warmth)  GAS_CONST_G_WARM_ACCESS)
                                        (* (- 1 (account-instruction-address-warmth)) GAS_CONST_G_COLD_ACCOUNT_ACCESS))))
                   (if-not-zero (account-instruction-no-trimming)
                                (eq! GAS_COST
                                     stack/STATIC_GAS))))

(defconstraint   account-instruction-trimming-case-garnishing-non-stack-row-make-account-row   (:guard (account-instruction-standard-hypothesis))
                 (begin
                   (if-not-zero (account-instruction-requires-trimming)
                                (begin
                                  (eq! account/ROMLEX_FLAG 1)
                                  (eq! account/TRM_RAW_ADDRESS_HI [ stack/STACK_ITEM_VALUE_HI 1 ])
                                  (eq! account/ADDRESS_LO         [ stack/STACK_ITEM_VALUE_LO 1 ])
                                  (account-same-balance                         1)
                                  (account-same-nonce                           1)
                                  (account-same-code                            1)
                                  (account-same-deployment-number-and-status    1)
                                  (account-turn-on-warmth                       1)
                                  (account-same-marked-for-selfdestruct         1)
                                  (DOM-SUB-stamps---standard                    1 0)))))


(defconstraint   account-instruction-trimming-case-garnishing-non-stack-row-undo-account-row   (:guard (account-instruction-standard-hypothesis))
                 (begin
                   (if-not-zero (account-instruction-requires-trimming)
                                (if-not-zero CONTEXT_WILL_REVERT
                                             (begin
                                               (account-same-address-as                2 1)
                                               (account-undo-balance-update            2 1)
                                               (account-undo-nonce-update              2 1)
                                               (account-undo-code-update               2 1)
                                               (account-undo-deployment-status-update  2 1)
                                               (account-undo-warmth-update             2 1)
                                               (account-same-marked-for-selfdestruct   2  )
                                               (DOM-SUB-stamps---revert-with-current   2 1))))))

(defconstraint   account-instruction-non-trim-case                                          (:guard (account-instruction-standard-hypothesis))
                 (begin
                   (if-not-zero (account-instruction-no-trimming)
                                (if-zero XAHOY
                                         (begin
                                           (read-context-data                          1 CONTEXT_NUMBER)
                                           (account-same-balance                       2)
                                           (account-same-nonce                         2)
                                           (account-same-code                          2)
                                           (account-same-deployment-number-and-status  2)
                                           (account-turn-on-warmth                     2)
                                           (account-same-marked-for-selfdestruct       2)
                                           (DOM-SUB-stamps---standard                  2 0)
                                           (if-zero [ stack/DEC_FLAG 4 ]
                                                    ;; DEC_FLAG_4 = 0
                                                    (begin
                                                      (eq!  (shift account/ADDRESS_HI  2) (account-instruction-account-address-hi))
                                                      (eq!  (shift account/ADDRESS_LO  2) (account-instruction-account-address-lo)))
                                                    ;; DEC_FLAG_4 = 1
                                                    (begin
                                                      (eq!  (shift account/ADDRESS_HI  2)  (account-instruction-byte-code-address-hi))
                                                      (eq!  (shift account/ADDRESS_LO  2)  (account-instruction-byte-code-address-lo)))))))))

(defconstraint   account-instruction-value-constraints                                      (:guard (account-instruction-unexceptional))
                 (begin
                   (if-not-zero [ stack/DEC_FLAG 1 ]
                                (begin
                                  (eq!  [ stack/STACK_ITEM_VALUE_HI 4 ] 0)
                                  (eq!  [ stack/STACK_ITEM_VALUE_LO 4 ] (shift account/BALANCE 1))))
                   ;;
                   (if-not-zero [ stack/DEC_FLAG 2 ]
                                (begin
                                  (eq!  [ stack/STACK_ITEM_VALUE_HI 4 ] 0)
                                  (eq!  [ stack/STACK_ITEM_VALUE_LO 4 ] (shift account/CODE_SIZE 1))))
                   ;;
                   (if-not-zero [ stack/DEC_FLAG 3 ]
                                (begin
                                  (eq!  [ stack/STACK_ITEM_VALUE_HI 4 ] (shift account/CODE_HASH_HI 1))
                                  (eq!  [ stack/STACK_ITEM_VALUE_LO 4 ] (shift account/CODE_HASH_LO 1))))
                   ;;
                   (if-not-zero [ stack/DEC_FLAG 4 ]
                                (begin
                                  (eq!  [ stack/STACK_ITEM_VALUE_HI 4 ] 0)
                                  (eq!  [ stack/STACK_ITEM_VALUE_LO 4 ] (shift account/CODE_SIZE 2))))
                   ;;
                   (if-not-zero (- 1 (account-instruction-decoded-flags-sum))
                                (begin
                                  (eq!  [ stack/STACK_ITEM_VALUE_HI 4 ] 0)
                                  (eq!  [ stack/STACK_ITEM_VALUE_LO 4 ] (shift account/CODE_SIZE 2))))))
