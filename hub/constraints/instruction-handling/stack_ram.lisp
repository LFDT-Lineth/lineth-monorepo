(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                               ;;;;
;;;;    X.5 Instruction handling   ;;;;
;;;;                               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                    ;;
;;    X.5.Y Instructions raising the STACK_RAM_FLAG   ;;
;;                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;    X.5.1 Supported instructions and flags   ;;
;;    X.5.2 Shorthands                         ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconst
  stack-ram-misc-row-offset    1
  stack-ram-context-row-offset 2)

(defun (stack-ram-inst-instruction)                stack/INSTRUCTION )
(defun (stack-ram-inst-is-CDL)                (+ [ stack/DEC_FLAG 1 ]))
(defun (stack-ram-inst-is-store-instruction)  (+ [ stack/DEC_FLAG 3 ]
                                                        [ stack/DEC_FLAG 4 ]))
(defun (stack-ram-inst-is-MXX)                (+ [ stack/DEC_FLAG 2 ]
                                                        [ stack/DEC_FLAG 3 ]
                                                        [ stack/DEC_FLAG 4 ]))
(defun (stack-ram-inst-offset-hi)                [ stack/STACK_ITEM_VALUE_HI 1 ])
(defun (stack-ram-inst-offset-lo)                [ stack/STACK_ITEM_VALUE_LO 1 ])
(defun (stack-ram-inst-value-hi)                 [ stack/STACK_ITEM_VALUE_HI 4 ])
(defun (stack-ram-inst-value-lo)                 [ stack/STACK_ITEM_VALUE_LO 4 ])
(defun (stack-ram-inst-CDL-is-oob)               (shift   [ misc/OOB_DATA 7 ]          stack-ram-misc-row-offset))
(defun (stack-ram-inst-MXP-gas)                  (shift     misc/MXP_GAS_MXP           stack-ram-misc-row-offset))
(defun (stack-ram-inst-MXPX)                     (shift     misc/MXP_MXPX              stack-ram-misc-row-offset))
(defun (stack-ram-inst-call-data-size)           (shift     context/CALL_DATA_SIZE     stack-ram-context-row-offset))
(defun (stack-ram-inst-call-data-offset)         (shift     context/CALL_DATA_OFFSET   stack-ram-context-row-offset))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.5.3 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (stack-ram-inst-std-hyp) (* PEEK_AT_STACK
                                                    stack/STACKRAM_FLAG
                                                    (- 1 stack/SUX stack/SOX)))

(defconstraint stack-ram-inst-setting-the-stack-pattern                 (:guard (stack-ram-inst-std-hyp))
               (load-store-stack-pattern         (force-bin (stack-ram-inst-is-store-instruction))))

(defconstraint stack-ram-inst-setting-NSR                               (:guard (stack-ram-inst-std-hyp))
               (eq! NSR
                    (+ 1 [ stack/DEC_FLAG 1 ] CMC)))

(defconstraint stack-ram-inst-setting-the-peeking-flags                 (:guard (stack-ram-inst-std-hyp))
               (begin (if-not-zero (stack-ram-inst-is-CDL)
                                   (eq! NSR
                                        (+ (shift PEEK_AT_MISCELLANEOUS   stack-ram-misc-row-offset)
                                           (shift PEEK_AT_CONTEXT         stack-ram-context-row-offset)
                                           (* (shift PEEK_AT_CONTEXT 3) CMC))))
                      (if-not-zero (stack-ram-inst-is-MXX)
                                   (eq! NSR
                                        (+ (shift PEEK_AT_MISCELLANEOUS   stack-ram-misc-row-offset)
                                           (* (shift PEEK_AT_CONTEXT 2) CMC))))
                      (debug (eq! CMC XAHOY))))

(defconstraint stack-ram-inst-setting-the-memory-expansion-exception    (:guard (stack-ram-inst-std-hyp))
               (begin (if-not-zero (stack-ram-inst-is-CDL)
                                   (vanishes! stack/MXPX))
                      (if-not-zero (stack-ram-inst-is-MXX)
                                   (eq! stack/MXPX (stack-ram-inst-MXPX)))))
               
(defconstraint stack-ram-inst-setting-the-gas-cost                      (:guard (stack-ram-inst-std-hyp))
               (begin (if-not-zero (stack-ram-inst-is-CDL)
                                   (eq! GAS_COST stack/STATIC_GAS))
                      (if-not-zero (stack-ram-inst-is-MXX)
                                   (if-zero (force-bin (stack-ram-inst-MXPX))
                                            (eq! GAS_COST
                                                 (+ stack/STATIC_GAS
                                                    (stack-ram-inst-MXP-gas)))
                                            (vanishes! GAS_COST)))))

(defconstraint stack-ram-inst-setting-MISC-module-flags                 (:guard (stack-ram-inst-std-hyp))
               (eq! (weighted-MISC-flag-sum       stack-ram-misc-row-offset)
                    (+ (* MISC_WEIGHT_MMU (stack-ram-inst-trigger_MMU))
                       (* MISC_WEIGHT_MXP (stack-ram-inst-is-MXX))
                       (* MISC_WEIGHT_OOB (stack-ram-inst-is-CDL)))))

;; defining trigger_MMU
(defun (stack-ram-inst-trigger_MMU)              (* (- 1 XAHOY)
                                                           (+ (* (stack-ram-inst-is-CDL) (- 1 (stack-ram-inst-CDL-is-oob)))
                                                              (stack-ram-inst-is-MXX))))

(defconstraint stack-ram-inst-setting-OOB-instruction                   (:guard (stack-ram-inst-std-hyp))
               (if-not-zero (stack-ram-inst-is-CDL)
                            (set-OOB-instruction-cdl     stack-ram-misc-row-offset               ;; row offset
                                                         (stack-ram-inst-offset-hi)              ;; offset within call data, high part
                                                         (stack-ram-inst-offset-lo)              ;; offset within call data, low  part
                                                         (stack-ram-inst-call-data-size))))      ;; call data size

(defconstraint stack-ram-inst-setting-value-for-trivial-CALLDATALOAD    (:guard (stack-ram-inst-std-hyp))
               (if-not-zero (stack-ram-inst-is-CDL)
                            (if-not-zero (stack-ram-inst-CDL-is-oob)
                                         (begin
                                           (vanishes! (stack-ram-inst-value-hi))
                                           (vanishes! (stack-ram-inst-value-lo))))))

(defconstraint stack-ram-inst-setting-context-row-for-CALLDATALOAD      (:guard (stack-ram-inst-std-hyp))
               (if-not-zero (stack-ram-inst-is-CDL)
                            (read-context-data stack-ram-context-row-offset CONTEXT_NUMBER)))

(defconstraint stack-ram-inst-setting-MXP-instruction                   (:guard (stack-ram-inst-std-hyp))
               (if-not-zero (shift misc/MXP_FLAG stack-ram-misc-row-offset)
                            (set-MXP-instruction-type-2 stack-ram-misc-row-offset         ;; row offset
                                                        (stack-ram-inst-instruction)      ;; instruction
                                                        (stack-ram-inst-offset-hi)             ;; source offset high
                                                        (stack-ram-inst-offset-lo))))          ;; source offset low

(defconstraint stack-ram-inst-setting-MMU-instruction                   (:guard (stack-ram-inst-std-hyp))
               (if-not-zero (shift misc/MMU_FLAG stack-ram-misc-row-offset)
                            (begin (if-not-zero [ stack/DEC_FLAG 1]
                                                ;; CALLDATALOAD case
                                                (set-MMU-instruction-right-padded-word-extraction    stack-ram-misc-row-offset           ;; row offsetet
                                                                                                     CALLER_CONTEXT_NUMBER               ;; source ID
                                                                                                     ;; tgt_id                              ;; target ID
                                                                                                     ;; aux_id                              ;; auxiliary ID
                                                                                                     ;; src_offset_hi                       ;; source offset high
                                                                                                     (stack-ram-inst-offset-lo)          ;; source offset low
                                                                                                     ;; tgt_offset_lo                       ;; target offset low
                                                                                                     ;; size                                ;; size
                                                                                                     (stack-ram-inst-call-data-offset)   ;; reference offset
                                                                                                     (stack-ram-inst-call-data-size)     ;; reference size
                                                                                                     ;; success_bit                         ;; success bit
                                                                                                     (stack-ram-inst-value-hi)           ;; limb 1
                                                                                                     (stack-ram-inst-value-lo)           ;; limb 2
                                                                                                     ;; exo_sum                             ;; weighted exogenous module flag sum
                                                                                                     ;; phase                               ;; phase
                                                                                                     ))
                                   (if-not-zero [ stack/DEC_FLAG 2]
                                                ;; MLOAD case
                                                (set-MMU-instruction-mload    stack-ram-misc-row-offset           ;; offset
                                                                              CONTEXT_NUMBER                      ;; source ID
                                                                              ;; tgt_id                              ;; target ID
                                                                              ;; aux_id                              ;; auxiliary ID
                                                                              ;; src_offset_hi                       ;; source offset high
                                                                              (stack-ram-inst-offset-lo)          ;; source offset low
                                                                              ;; tgt_offset_lo                       ;; target offset low
                                                                              ;; size                                ;; size
                                                                              ;; ref_offset                          ;; reference offset
                                                                              ;; ref_size                            ;; reference size
                                                                              ;; success_bit                         ;; success bit
                                                                              (stack-ram-inst-value-hi)           ;; limb 1
                                                                              (stack-ram-inst-value-lo)           ;; limb 2
                                                                              ;; exo_sum                             ;; weighted exogenous module flag sum
                                                                              ;; phase                               ;; phase
                                                                              ))
                                   (if-not-zero [ stack/DEC_FLAG 3]
                                                ;; MSTORE case
                                                (set-MMU-instruction-mstore    stack-ram-misc-row-offset           ;; offset
                                                                               ;; src_id                              ;; source ID
                                                                               CONTEXT_NUMBER                      ;; target ID
                                                                               ;; aux_id                              ;; auxiliary ID
                                                                               ;; src_offset_hi                       ;; source offset high
                                                                               ;; src_offset_lo                       ;; source offset low
                                                                               (stack-ram-inst-offset-lo)          ;; target offset low
                                                                               ;; size                                ;; size
                                                                               ;; ref_offset                          ;; reference offset
                                                                               ;; ref_size                            ;; reference size
                                                                               ;; success_bit                         ;; success bit
                                                                               (stack-ram-inst-value-hi)           ;; limb 1
                                                                               (stack-ram-inst-value-lo)           ;; limb 2
                                                                               ;; exo_sum                             ;; weighted exogenous module flag sum
                                                                               ;; phase                               ;; phase
                                                                               ))
                                   (if-not-zero [ stack/DEC_FLAG 4]
                                                ;; MSTORE8 case
                                                (set-MMU-instruction-mstore8    stack-ram-misc-row-offset           ;; offset
                                                                                ;; src_id                              ;; source ID
                                                                                CONTEXT_NUMBER                      ;; target ID
                                                                                ;; aux_id                              ;; auxiliary ID
                                                                                ;; src_offset_hi                       ;; source offset high
                                                                                ;; src_offset_lo                       ;; source offset low
                                                                                (stack-ram-inst-offset-lo)          ;; target offset low
                                                                                ;; size                                ;; size
                                                                                ;; ref_offset                          ;; reference offset
                                                                                ;; ref_size                            ;; reference size
                                                                                ;; success_bit                         ;; success bit
                                                                                (stack-ram-inst-value-hi)           ;; limb 1
                                                                                (stack-ram-inst-value-lo)           ;; limb 2
                                                                                ;; exo_sum                             ;; weighted exogenous module flag sum
                                                                                ;; phase                               ;; phase
                                                                                )))))
