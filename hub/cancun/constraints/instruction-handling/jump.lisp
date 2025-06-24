(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                               ;;;;
;;;;    X.5 Instruction handling   ;;;;
;;;;                               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    X.5.27 Jump instructions   ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (jump-instruction---no-stack-exception)
  (* PEEK_AT_STACK
     stack/JUMP_FLAG
     (- 1 stack/SUX stack/SOX)))

(defun (jump-instruction---no-stack-exception-and-no-oogx)
  (* (jump-instruction---no-stack-exception)
     (- 1 stack/OOGX)))

(defconst
  ;; OOGX case
  ROW_OFFSET_FOR_JUMP_OOGX_CONTEXT_ROW                               1
  ;; no OOGX case
  ROW_OFFSET_FOR_JUMP_NO_OOGX_CURRENT_CONTEXT_ROW                    1
  ROW_OFFSET_FOR_JUMP_NO_OOGX_ADDRESS_ROW                            2
  ROW_OFFSET_FOR_JUMP_NO_OOGX_MISC_ROW                               3
  ROW_OFFSET_FOR_JUMP_NO_OOGX_JUMPX_CALLER_CONTEXT_ROW               4)

(defun (jump-instruction---new-pc-hi)                         [ stack/STACK_ITEM_VALUE_HI 1 ])
(defun (jump-instruction---new-pc-lo)                         [ stack/STACK_ITEM_VALUE_LO 1 ])
(defun (jump-instruction---jump-condition-hi)                 [ stack/STACK_ITEM_VALUE_HI 2 ])
(defun (jump-instruction---jump-condition-lo)                 [ stack/STACK_ITEM_VALUE_LO 2 ])
(defun (jump-instruction---is-JUMP)                           [ stack/DEC_FLAG 1 ])
(defun (jump-instruction---is-JUMPI)                          [ stack/DEC_FLAG 2 ])
;;
(defun (jump-instruction---code-address-hi)                   (shift  context/BYTE_CODE_ADDRESS_HI  ROW_OFFSET_FOR_JUMP_NO_OOGX_CURRENT_CONTEXT_ROW))
(defun (jump-instruction---code-address-lo)                   (shift  context/BYTE_CODE_ADDRESS_LO  ROW_OFFSET_FOR_JUMP_NO_OOGX_CURRENT_CONTEXT_ROW))
;;
(defun (jump-instruction---code-size)                         (shift  account/CODE_SIZE             ROW_OFFSET_FOR_JUMP_NO_OOGX_ADDRESS_ROW))
;;
(defun (jump-instruction---JUMP-guaranteed-exception)         (shift  [ misc/OOB_DATA 7 ]           ROW_OFFSET_FOR_JUMP_NO_OOGX_MISC_ROW))
(defun (jump-instruction---JUMP-must-be-attempted)            (shift  [ misc/OOB_DATA 8 ]           ROW_OFFSET_FOR_JUMP_NO_OOGX_MISC_ROW))
;;
(defun (jump-instruction---JUMPI-jump-not-attempted)          (shift  [ misc/OOB_DATA 6 ]           ROW_OFFSET_FOR_JUMP_NO_OOGX_MISC_ROW))
(defun (jump-instruction---JUMPI-guaranteed-exception)        (shift  [ misc/OOB_DATA 7 ]           ROW_OFFSET_FOR_JUMP_NO_OOGX_MISC_ROW))
(defun (jump-instruction---JUMPI-must-be-attempted)           (shift  [ misc/OOB_DATA 8 ]           ROW_OFFSET_FOR_JUMP_NO_OOGX_MISC_ROW)) ;; ""

(defconstraint jump-instruction---setting-the-stack-pattern                   (:guard (jump-instruction---no-stack-exception))
               (begin
                 (if-not-zero (jump-instruction---is-JUMP)   (stack-pattern-1-0))
                 (if-not-zero (jump-instruction---is-JUMPI)  (stack-pattern-2-0))))

(defconstraint jump-instruction---allowable-exceptions                        (:guard (jump-instruction---no-stack-exception))
               (eq! XAHOY (+ stack/OOGX stack/JUMPX)))


(defconstraint jump-instruction---setting-the-gas-cost                        (:guard (jump-instruction---no-stack-exception))
               (eq! GAS_COST stack/STATIC_GAS))

(defconstraint jump-instruction---setting-NSR                                 (:guard (jump-instruction---no-stack-exception))
               (if-not-zero    (force-bin stack/OOGX)
                               ;; OOGX = 1
                               (eq!        NSR CMC)
                               ;; OOGX = 0
                               (eq! NSR (+ 3 CMC))))

(defconstraint jump-instruction---setting-peeking-flags                       (:guard (jump-instruction---no-stack-exception))
               (if-not-zero (force-bin stack/OOGX)
                            ;; OOGX = 1
                            (eq! NSR (shift PEEK_AT_CONTEXT  ROW_OFFSET_FOR_JUMP_OOGX_CONTEXT_ROW))
                            ;; OOGX = 0
                            (eq! NSR (+ (shift PEEK_AT_CONTEXT             ROW_OFFSET_FOR_JUMP_NO_OOGX_CURRENT_CONTEXT_ROW)
                                        (shift PEEK_AT_ACCOUNT             ROW_OFFSET_FOR_JUMP_NO_OOGX_ADDRESS_ROW)
                                        (shift PEEK_AT_MISCELLANEOUS       ROW_OFFSET_FOR_JUMP_NO_OOGX_MISC_ROW)
                                        (* CMC (shift PEEK_AT_CONTEXT      ROW_OFFSET_FOR_JUMP_NO_OOGX_JUMPX_CALLER_CONTEXT_ROW))))))

(defconstraint jump-instruction---setting-the-first-context-row               (:guard (jump-instruction---no-stack-exception))
               (if-not-zero (force-bin stack/OOGX)
                            ;; OOGX = 1
                            (execution-provides-empty-return-data          ROW_OFFSET_FOR_JUMP_NO_OOGX_CURRENT_CONTEXT_ROW)
                            ;; OOGX = 0
                            (read-context-data                             ROW_OFFSET_FOR_JUMP_NO_OOGX_CURRENT_CONTEXT_ROW
                                                                           CONTEXT_NUMBER)))

;; stronger preconditions start here
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint jump-instruction---the-account-row                            (:guard (jump-instruction---no-stack-exception-and-no-oogx))
               (begin
                 (eq! (shift account/ADDRESS_HI  ROW_OFFSET_FOR_JUMP_NO_OOGX_ADDRESS_ROW)  (jump-instruction---code-address-hi))
                 (eq! (shift account/ADDRESS_LO  ROW_OFFSET_FOR_JUMP_NO_OOGX_ADDRESS_ROW)  (jump-instruction---code-address-lo))
                 (account-same-balance                           ROW_OFFSET_FOR_JUMP_NO_OOGX_ADDRESS_ROW)
                 (account-same-nonce                             ROW_OFFSET_FOR_JUMP_NO_OOGX_ADDRESS_ROW)
                 (account-same-code                              ROW_OFFSET_FOR_JUMP_NO_OOGX_ADDRESS_ROW)
                 (account-same-deployment-number-and-status      ROW_OFFSET_FOR_JUMP_NO_OOGX_ADDRESS_ROW)
                 (account-same-warmth                            ROW_OFFSET_FOR_JUMP_NO_OOGX_ADDRESS_ROW)
                 (account-same-marked-for-deletion               ROW_OFFSET_FOR_JUMP_NO_OOGX_ADDRESS_ROW)
                 (DOM-SUB-stamps---standard                      ROW_OFFSET_FOR_JUMP_NO_OOGX_ADDRESS_ROW
                                                                 0)))

(defconstraint jump-instruction---miscellaneous-row---setting-the-module-flags                 (:guard (jump-instruction---no-stack-exception-and-no-oogx))
               (eq! (weighted-MISC-flag-sum   ROW_OFFSET_FOR_JUMP_NO_OOGX_MISC_ROW)    MISC_WEIGHT_OOB))

(defconstraint jump-instruction---miscellaneous-row---setting-the-OOB-instruction---JUMP-case       (:guard (jump-instruction---no-stack-exception-and-no-oogx))
                 (if-not-zero (jump-instruction---is-JUMP)
                              (set-OOB-instruction---jump    ROW_OFFSET_FOR_JUMP_NO_OOGX_MISC_ROW
                                                             (jump-instruction---new-pc-hi)
                                                             (jump-instruction---new-pc-lo)
                                                             (jump-instruction---code-size))))

(defconstraint jump-instruction---miscellaneous-row---setting-the-OOB-instruction---JUMPI-case       (:guard (jump-instruction---no-stack-exception-and-no-oogx))
                 (if-not-zero (jump-instruction---is-JUMPI)
                              (set-OOB-instruction---jumpi   ROW_OFFSET_FOR_JUMP_NO_OOGX_MISC_ROW
                                                             (jump-instruction---new-pc-hi)
                                                             (jump-instruction---new-pc-lo)
                                                             (jump-instruction---jump-condition-hi)
                                                             (jump-instruction---jump-condition-lo)
                                                             (jump-instruction---code-size))))


(defconstraint jump-instruction---setting-PC_NEW-and-JUMP_DESTINATION_VETTING-for-JUMP (:guard (jump-instruction---no-stack-exception-and-no-oogx))
               (if-not-zero (jump-instruction---is-JUMP)
                            (begin
                              (if-not-zero (jump-instruction---JUMP-guaranteed-exception)
                                           (begin (eq! stack/JUMP_DESTINATION_VETTING_REQUIRED 0)
                                                  (eq! stack/JUMPX 1)))
                              (if-not-zero (jump-instruction---JUMP-must-be-attempted)
                                           (begin (eq! stack/JUMP_DESTINATION_VETTING_REQUIRED 1)
                                                  (if-zero XAHOY (eq! PC_NEW (jump-instruction---new-pc-lo))))))))


(defconstraint jump-instruction---setting-PC_NEW-and-JUMP_DESTINATION_VETTING-for-JUMPI (:guard (jump-instruction---no-stack-exception-and-no-oogx))
               (if-not-zero (jump-instruction---is-JUMPI)
                            (begin
                              (if-not-zero (jump-instruction---JUMPI-jump-not-attempted)
                                           (begin (eq! stack/JUMP_DESTINATION_VETTING_REQUIRED 0)
                                                  (eq! stack/JUMPX 0)
                                                  (eq! PC_NEW (+ 1 PC))))
                              (if-not-zero (jump-instruction---JUMPI-guaranteed-exception)
                                           (begin (eq! stack/JUMP_DESTINATION_VETTING_REQUIRED 0)
                                                  (eq! stack/JUMPX 1)))
                              (if-not-zero (jump-instruction---JUMPI-must-be-attempted)
                                           (begin (eq! stack/JUMP_DESTINATION_VETTING_REQUIRED 1)
                                                  (if-zero XAHOY (eq! PC_NEW (jump-instruction---new-pc-lo))))))))
