(module hub)

(defun    (storage-instruction---no-stack-exceptions)          (* PEEK_AT_STACK stack/STO_FLAG (- 1 stack/SUX stack/SOX)))
(defun    (storage-instruction---is-SLOAD)                     [ stack/DEC_FLAG 1 ])
(defun    (storage-instruction---is-SSTORE)                    [ stack/DEC_FLAG 2 ])
(defun    (storage-instruction---storage-key-hi)               [ stack/STACK_ITEM_VALUE_HI 1 ])
(defun    (storage-instruction---storage-key-lo)               [ stack/STACK_ITEM_VALUE_LO 1 ])
(defun    (storage-instruction---loaded-value-hi)              [ stack/STACK_ITEM_VALUE_HI 4 ])
(defun    (storage-instruction---loaded-value-lo)              [ stack/STACK_ITEM_VALUE_LO 4 ])
(defun    (storage-instruction---value-to-store-hi)            [ stack/STACK_ITEM_VALUE_HI 4 ])
(defun    (storage-instruction---value-to-store-lo)            [ stack/STACK_ITEM_VALUE_LO 4 ])
(defun    (storage-instruction---OOB-prediction-of-sstorex)    (shift  [ misc/OOB_DATA 7 ]  2)) ;; ""

(defconstraint   storage-instruction---stack-pattern
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (load-store-stack-pattern (storage-instruction---is-SSTORE)))

(defconstraint   storage-instruction---valid-exceptions
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (if-not-zero    (storage-instruction---is-SLOAD)
                                   (eq! XAHOY stack/OOGX))
                   (if-not-zero    (storage-instruction---is-SSTORE)
                                   (eq! XAHOY
                                        (+ stack/STATICX
                                           stack/SSTOREX
                                           stack/OOGX)))))

(defconstraint   storage-instruction---setting-NSR-and-peeking-flags-STATICX
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;; static exception
                 ;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    stack/STATICX
                                 (begin
                                   (eq! NSR 2)
                                   (eq! NSR
                                        (+ (shift PEEK_AT_CONTEXT 1)
                                           (shift PEEK_AT_CONTEXT 2))))))

(defconstraint   storage-instruction---setting-NSR-and-peeking-flags-SSTOREX
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;; sstore gas exception
                 ;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    stack/SSTOREX
                                 (begin
                                   (eq! NSR 3)
                                   (eq! NSR
                                        (+ (shift PEEK_AT_CONTEXT       1)
                                           (shift PEEK_AT_MISCELLANEOUS 2)
                                           (shift PEEK_AT_CONTEXT       3))))))

(defconstraint   storage-instruction---setting-NSR-and-peeking-flags-OOGX
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;; out of gas exception
                 ;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    stack/OOGX
                                 (begin
                                   (eq! NSR 5)
                                   (eq! NSR
                                        (+ (shift PEEK_AT_CONTEXT       1)
                                           (shift PEEK_AT_MISCELLANEOUS 2)
                                           (shift PEEK_AT_STORAGE       3)
                                           (shift PEEK_AT_STORAGE       4)
                                           (shift PEEK_AT_CONTEXT       5))))))

(defconstraint   storage-instruction---setting-NSR-and-peeking-flags-UNEXCEPTIONAL
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;; unexceptional
                 ;;;;;;;;;;;;;;;;
                 (if-zero    XAHOY
                             (begin
                               (eq! NSR (+ 3 CONTEXT_WILL_REVERT))
                               (eq! NSR
                                    (+ (shift PEEK_AT_CONTEXT        1)
                                       (shift PEEK_AT_MISCELLANEOUS  2)
                                       (shift PEEK_AT_STORAGE        3)
                                       (* CONTEXT_WILL_REVERT (shift PEEK_AT_STORAGE 4)))))))

(defconstraint   storage-instruction---first-context-row
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (read-context-data 1                  ;; row offset
                                    CONTEXT_NUMBER ))  ;; context to read

(defconstraint   storage-instruction---justifying-STATICX
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq! stack/STATICX
                      (* (storage-instruction---is-SSTORE)
                         (shift context/IS_STATIC 1))))

(defconstraint   storage-instruction---setting-MISC-row
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (shift    PEEK_AT_MISCELLANEOUS    2)
                                 (begin
                                   (eq! (weighted-MISC-flag-sum 2)
                                        (* (storage-instruction---is-SSTORE) MISC_WEIGHT_OOB))
                                   (if-not-zero    (storage-instruction---is-SSTORE)
                                                   (set-OOB-instruction---sstore    2              ;; offset
                                                                                    GAS_ACTUAL ))  ;; GAS_ACTUAL
                                   )))

(defconstraint   storage-instruction---justifying-SSTOREX
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (shift PEEK_AT_MISCELLANEOUS 2)
                                 (if-not-zero (storage-instruction---is-SSTORE)
                                              (eq! stack/SSTOREX
                                                   (storage-instruction---OOB-prediction-of-sstorex)))))

(defun (no-exception)         (- 1 XAHOY))
(defun (oogx-or-no-exception) (+ stack/OOGX (no-exception)))

(defconstraint   storage-instruction---setting-storage-slot-parameters
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (oogx-or-no-exception)
                                 (begin
                                   (eq! (shift storage/ADDRESS_HI        3) (shift context/ACCOUNT_ADDRESS_HI        1))
                                   (eq! (shift storage/ADDRESS_LO        3) (shift context/ACCOUNT_ADDRESS_LO        1))
                                   (eq! (shift storage/DEPLOYMENT_NUMBER 3) (shift context/ACCOUNT_DEPLOYMENT_NUMBER 1))
                                   (eq! (shift storage/STORAGE_KEY_HI    3) (storage-instruction---storage-key-hi))
                                   (eq! (shift storage/STORAGE_KEY_LO    3) (storage-instruction---storage-key-lo))
                                   (storage-turn-on-warmth               3)
                                   (DOM-SUB-stamps---standard            3        ;; kappa
                                                                         0))))    ;; c

(defconstraint   storage-instruction---setting-storage-slot-values---doing-for-SLOAD
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (storage-instruction---is-SLOAD)
                                 (begin
                                   (if-not-zero    (oogx-or-no-exception)
                                                   (storage-reading 3))
                                   (if-not-zero    (no-exception)
                                                   (begin
                                                     (eq! (storage-instruction---loaded-value-hi)     (shift storage/VALUE_CURR_HI 3))
                                                     (eq! (storage-instruction---loaded-value-lo)     (shift storage/VALUE_CURR_LO 3)))))))

(defconstraint   storage-instruction---setting-storage-slot-values---doing-for-SSTORE
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (storage-instruction---is-SSTORE)
                                 (if-not-zero    (oogx-or-no-exception)
                                                 (begin
                                                   (eq! (storage-instruction---value-to-store-hi)   (shift storage/VALUE_NEXT_HI 3))
                                                   (eq! (storage-instruction---value-to-store-lo)   (shift storage/VALUE_NEXT_LO 3))))))

(defconstraint   storage-instruction---setting-storage-slot-values---undoing-storage-operation
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (oogx-or-no-exception)
                                 (if-not-zero CONTEXT_WILL_REVERT
                                              (begin
                                                (storage-same-slot                        4)
                                                (undo-storage-warmth-and-value-update     4)
                                                (DOM-SUB-stamps---revert-with-current     4       ;; kappa
                                                                                          0)))))  ;; c

(defun    (orig-is-zero)           (shift storage/VALUE_ORIG_IS_ZERO 3))
(defun    (curr-is-zero)           (shift storage/VALUE_CURR_IS_ZERO 3))
(defun    (next-is-zero)           (shift storage/VALUE_NEXT_IS_ZERO 3))
(defun    (curr-is-orig)           (shift storage/VALUE_CURR_IS_ORIG 3))
(defun    (next-is-orig)           (shift storage/VALUE_NEXT_IS_ORIG 3))
(defun    (next-is-curr)           (shift storage/VALUE_NEXT_IS_CURR 3))
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun    (orig-not-zero)          (force-bin (- 1 (orig-is-zero))))
(defun    (curr-not-zero)          (force-bin (- 1 (curr-is-zero))))
(defun    (next-not-zero)          (force-bin (- 1 (next-is-zero))))
(defun    (curr-not-orig)          (force-bin (- 1 (curr-is-orig))))
(defun    (next-not-orig)          (force-bin (- 1 (next-is-orig))))
(defun    (next-not-curr)          (force-bin (- 1 (next-is-curr))))
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun    (context-wont-revert)    (force-bin (- 1 CONTEXT_WILL_REVERT)))
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun    (cold-slot)              (force-bin (- 1 (shift storage/WARMTH 3))))


(defconstraint   storage-instruction---setting-gas-costs---SLOAD-case
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (oogx-or-no-exception)
                                 (if-not-zero (storage-instruction---is-SLOAD)
                                              (if-not-zero    (cold-slot)
                                                              (eq!    GAS_COST    GAS_CONST_G_COLD_SLOAD)
                                                              (eq!    GAS_COST    GAS_CONST_G_WARM_ACCESS)))))

(defconstraint   storage-instruction---setting-gas-costs---SSTORE-case
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (oogx-or-no-exception)
                                 (if-not-zero (storage-instruction---is-SSTORE)
                                              (begin
                                                (if-not-zero     (next-is-curr)
                                                                 (eq!   GAS_COST   (+ GAS_CONST_G_WARM_ACCESS    (* (cold-slot)    GAS_CONST_G_COLD_SLOAD))))
                                                (if-not-zero     (curr-not-orig)
                                                                 (eq!   GAS_COST   (+ GAS_CONST_G_WARM_ACCESS    (* (cold-slot)    GAS_CONST_G_COLD_SLOAD))))
                                                (if-not-zero     (next-not-curr)
                                                                 (if-not-zero    (curr-is-orig)
                                                                                 (if-not-zero    (orig-is-zero)
                                                                                                 (eq!   GAS_COST   (+ GAS_CONST_G_SSET     (* (cold-slot)   GAS_CONST_G_COLD_SLOAD)))
                                                                                                 (eq!   GAS_COST   (+ GAS_CONST_G_SRESET   (* (cold-slot)   GAS_CONST_G_COLD_SLOAD))))))))))

(defun    (r-clean-clear)    (*    (next-not-curr)
                                   (curr-is-orig)
                                   (next-is-zero)
                                   REFUND_CONST_R_SCLEAR))
(defun    (r-dirty-clear)    (*    (next-not-curr)
                                   (curr-not-orig)
                                   (orig-not-zero)
                                   (-    (next-is-zero)
                                         (curr-is-zero))
                                   REFUND_CONST_R_SCLEAR))
(defun    (r-dirty-reset)    (*    (next-not-curr)
                                   (curr-not-orig)
                                   (next-is-orig)
                                   (+    (*    (orig-is-zero)     (-   GAS_CONST_G_SSET     GAS_CONST_G_WARM_ACCESS))
                                         (*    (orig-not-zero)    (-   GAS_CONST_G_SRESET   GAS_CONST_G_WARM_ACCESS)))))

(defconstraint   storage-instruction---setting-the-refund
                 (:guard (storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!
                   REFUND_COUNTER_NEW
                   (+    REFUND_COUNTER
                         (*    (context-wont-revert)
                               (storage-instruction---is-SSTORE)
                               (+    (r-clean-clear)
                                     (r-dirty-clear)
                                     (r-dirty-reset))))))
