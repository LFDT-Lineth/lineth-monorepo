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
  stack-ram---row-offset---misc-row    1
  stack-ram---row-offset---context-row 2)

(defun (stack-ram---instruction)                stack/INSTRUCTION )
(defun (stack-ram---is-CDL)                (+ [ stack/DEC_FLAG 1 ]))
(defun (stack-ram---is-MLOAD)              (+ [ stack/DEC_FLAG 2 ]))
(defun (stack-ram---is-MSTORE)             (+ [ stack/DEC_FLAG 3 ]))
(defun (stack-ram---is-MSTORE8)            (+ [ stack/DEC_FLAG 4 ]))
(defun (stack-ram---is-store-instruction)  (+ (stack-ram---is-MSTORE)
                                              (stack-ram---is-MSTORE8)))
(defun (stack-ram---is-MXX)                (+ (stack-ram---is-MLOAD)
                                              (stack-ram---is-MSTORE)
                                              (stack-ram---is-MSTORE8)))

(defun (stack-ram---offset-hi)                [ stack/STACK_ITEM_VALUE_HI 1 ])
(defun (stack-ram---offset-lo)                [ stack/STACK_ITEM_VALUE_LO 1 ])
(defun (stack-ram---value-hi)                 [ stack/STACK_ITEM_VALUE_HI 4 ])
(defun (stack-ram---value-lo)                 [ stack/STACK_ITEM_VALUE_LO 4 ])
(defun (stack-ram---CDL-is-oob)               (shift   [ misc/OOB_DATA 7 ]          stack-ram---row-offset---misc-row)) ;; ""
(defun (stack-ram---MXP-gas)                  (shift     misc/MXP_GAS_MXP           stack-ram---row-offset---misc-row))
(defun (stack-ram---MXPX)                     (shift     misc/MXP_MXPX              stack-ram---row-offset---misc-row))
(defun (stack-ram---call-data-size)           (shift     context/CALL_DATA_SIZE     stack-ram---row-offset---context-row))
(defun (stack-ram---call-data-offset)         (shift     context/CALL_DATA_OFFSET   stack-ram---row-offset---context-row))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.5.3 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (stack-ram---std-hyp) (* PEEK_AT_STACK
                                stack/STACKRAM_FLAG
                                (- 1 stack/SUX stack/SOX)))

(defconstraint   stack-ram---setting-the-stack-pattern                 (:guard (stack-ram---std-hyp))
                 (load-store-stack-pattern         (force-bin (stack-ram---is-store-instruction))))

(defconstraint   stack-ram---setting-NSR                               (:guard (stack-ram---std-hyp))
                 (eq! NSR
                      (+ 1 (stack-ram---is-CDL) CMC)))

(defconstraint   stack-ram---setting-the-peeking-flags                 (:guard (stack-ram---std-hyp))
                 (begin (if-not-zero (stack-ram---is-CDL)
                                     (eq! NSR
                                          (+ (shift PEEK_AT_MISCELLANEOUS   stack-ram---row-offset---misc-row)
                                             (shift PEEK_AT_CONTEXT         stack-ram---row-offset---context-row)
                                             (* (shift PEEK_AT_CONTEXT 3) CMC))))
                        (if-not-zero (stack-ram---is-MXX)
                                     (eq! NSR
                                          (+ (shift PEEK_AT_MISCELLANEOUS   stack-ram---row-offset---misc-row)
                                             (* (shift PEEK_AT_CONTEXT 2) CMC))))
                        (debug (eq! CMC XAHOY))))

(defconstraint   stack-ram---setting-the-memory-expansion-exception    (:guard (stack-ram---std-hyp))
                 (begin (if-not-zero (stack-ram---is-CDL)
                                     (vanishes! stack/MXPX))
                        (if-not-zero (stack-ram---is-MXX)
                                     (eq! stack/MXPX (stack-ram---MXPX)))))

(defconstraint   stack-ram---setting-the-gas-cost                      (:guard (stack-ram---std-hyp))
                 (begin (if-not-zero (stack-ram---is-CDL)
                                     (eq! GAS_COST stack/STATIC_GAS))
                        (if-not-zero (stack-ram---is-MXX)
                                     (if-zero (force-bin (stack-ram---MXPX))
                                              (eq! GAS_COST
                                                   (+ stack/STATIC_GAS
                                                      (stack-ram---MXP-gas)))
                                              (vanishes! GAS_COST)))))

(defconstraint   stack-ram---setting-MISC-module-flags                 (:guard (stack-ram---std-hyp))
                 (eq! (weighted-MISC-flag-sum       stack-ram---row-offset---misc-row)
                      (+ (* MISC_WEIGHT_MMU (stack-ram---trigger_MMU))
                         (* MISC_WEIGHT_MXP (stack-ram---is-MXX))
                         (* MISC_WEIGHT_OOB (stack-ram---is-CDL)))))

;; defining trigger_MMU
(defun (stack-ram---trigger_MMU)     (* (- 1 XAHOY)
                                        (+ (* (stack-ram---is-CDL) (- 1 (stack-ram---CDL-is-oob)))
                                           (stack-ram---is-MXX))))

(defconstraint   stack-ram---setting-OOB-instruction                   (:guard (stack-ram---std-hyp))
                 (if-not-zero (stack-ram---is-CDL)
                              (set-OOB-instruction---cdl     stack-ram---row-offset---misc-row               ;; row offset
                                                             (stack-ram---offset-hi)              ;; offset within call data, high part
                                                             (stack-ram---offset-lo)              ;; offset within call data, low  part
                                                             (stack-ram---call-data-size))))      ;; call data size

(defconstraint   stack-ram---setting-value-for-trivial-CALLDATALOAD    (:guard (stack-ram---std-hyp))
                 (if-not-zero (stack-ram---is-CDL)
                              (if-not-zero (stack-ram---CDL-is-oob)
                                           (begin
                                             (vanishes! (stack-ram---value-hi))
                                             (vanishes! (stack-ram---value-lo))))))

(defconstraint   stack-ram---setting-context-row-for-CALLDATALOAD      (:guard (stack-ram---std-hyp))
                 (if-not-zero    (stack-ram---is-CDL)
                                 (read-context-data stack-ram---row-offset---context-row CONTEXT_NUMBER)))

(defconstraint   stack-ram---setting-MXP-instruction---MLOAD-MSTORE-case                   (:guard (stack-ram---std-hyp))
                 (if-not-zero    (shift misc/MXP_FLAG stack-ram---row-offset---misc-row)
                                 (if-not-zero    (+    (stack-ram---is-MLOAD)    (stack-ram---is-MSTORE))
                                                 (set-MXP-instruction-type-2    stack-ram---row-offset---misc-row  ;; row offset
                                                                                (stack-ram---instruction)          ;; instruction
                                                                                (stack-ram---offset-hi)            ;; source offset high
                                                                                (stack-ram---offset-lo)))))        ;; source offset low

(defconstraint   stack-ram---setting-MXP-instruction---MSTORE8-case                   (:guard (stack-ram---std-hyp))
                 (if-not-zero    (shift misc/MXP_FLAG stack-ram---row-offset---misc-row)
                                 (if-not-zero    (stack-ram---is-MSTORE8)
                                                 (set-MXP-instruction-type-3    stack-ram---row-offset---misc-row  ;; row offset
                                                                                (stack-ram---offset-hi)            ;; source offset high
                                                                                (stack-ram---offset-lo)))))        ;; source offset low

(defun    (stack-ram---call-data-context-number)   (shift    context/CALL_DATA_CONTEXT_NUMBER    stack-ram---row-offset---context-row))
(defun    (stack-ram---trigger-MMU)                (shift    misc/MMU_FLAG                       stack-ram---row-offset---misc-row))

(defconstraint   stack-ram---setting-MMU-instruction---CALLDATALOAD-case                   (:guard (stack-ram---std-hyp))
                 (if-not-zero    (stack-ram---trigger-MMU)
                                ;; CALLDATALOAD case
                                ;;;;;;;;;;;;;;;;;;;;
                                (if-not-zero (stack-ram---is-CDL)
                                             (set-MMU-instruction---right-padded-word-extraction    stack-ram---row-offset---misc-row           ;; row offset
                                                                                                    (stack-ram---call-data-context-number)      ;; source ID
                                                                                                    ;; tgt_id                              ;; target ID
                                                                                                    ;; aux_id                              ;; auxiliary ID
                                                                                                    ;; src_offset_hi                       ;; source offset high
                                                                                                    (stack-ram---offset-lo)          ;; source offset low
                                                                                                    ;; tgt_offset_lo                       ;; target offset low
                                                                                                    ;; size                                ;; size
                                                                                                    (stack-ram---call-data-offset)   ;; reference offset
                                                                                                    (stack-ram---call-data-size)     ;; reference size
                                                                                                    ;; success_bit                         ;; success bit
                                                                                                    (stack-ram---value-hi)           ;; limb 1
                                                                                                    (stack-ram---value-lo)           ;; limb 2
                                                                                                    ;; exo_sum                             ;; weighted exogenous module flag sum
                                                                                                    ;; phase                               ;; phase
                                                                                                    ))))

(defconstraint   stack-ram---setting-MMU-instruction---MLOAD-case                   (:guard (stack-ram---std-hyp))
                 (if-not-zero    (stack-ram---trigger-MMU)
                                ;; MLOAD case
                                ;;;;;;;;;;;;;
                                (if-not-zero (stack-ram---is-MLOAD)
                                             (set-MMU-instruction---mload    stack-ram---row-offset---misc-row           ;; offset
                                                                             CONTEXT_NUMBER                      ;; source ID
                                                                             ;; tgt_id                              ;; target ID
                                                                             ;; aux_id                              ;; auxiliary ID
                                                                             ;; src_offset_hi                       ;; source offset high
                                                                             (stack-ram---offset-lo)          ;; source offset low
                                                                             ;; tgt_offset_lo                       ;; target offset low
                                                                             ;; size                                ;; size
                                                                             ;; ref_offset                          ;; reference offset
                                                                             ;; ref_size                            ;; reference size
                                                                             ;; success_bit                         ;; success bit
                                                                             (stack-ram---value-hi)           ;; limb 1
                                                                             (stack-ram---value-lo)           ;; limb 2
                                                                             ;; exo_sum                             ;; weighted exogenous module flag sum
                                                                             ;; phase                               ;; phase
                                                                             ))))

(defconstraint   stack-ram---setting-MMU-instruction---MSTORE-case                   (:guard (stack-ram---std-hyp))
                 (if-not-zero    (stack-ram---trigger-MMU)
                                ;; MSTORE case
                                ;;;;;;;;;;;;;;
                                (if-not-zero (stack-ram---is-MSTORE)
                                             (set-MMU-instruction---mstore    stack-ram---row-offset---misc-row           ;; offset
                                                                              ;; src_id                              ;; source ID
                                                                              CONTEXT_NUMBER                      ;; target ID
                                                                              ;; aux_id                              ;; auxiliary ID
                                                                              ;; src_offset_hi                       ;; source offset high
                                                                              ;; src_offset_lo                       ;; source offset low
                                                                              (stack-ram---offset-lo)          ;; target offset low
                                                                              ;; size                                ;; size
                                                                              ;; ref_offset                          ;; reference offset
                                                                              ;; ref_size                            ;; reference size
                                                                              ;; success_bit                         ;; success bit
                                                                              (stack-ram---value-hi)           ;; limb 1
                                                                              (stack-ram---value-lo)           ;; limb 2
                                                                              ;; exo_sum                             ;; weighted exogenous module flag sum
                                                                              ;; phase                               ;; phase
                                                                              ))))

(defconstraint   stack-ram---setting-MMU-instruction---MSTORE8-case                   (:guard (stack-ram---std-hyp))
                 (if-not-zero    (stack-ram---trigger-MMU)
                                ;; MSTORE8 case
                                ;;;;;;;;;;;;;;;
                                (if-not-zero (stack-ram---is-MSTORE8)
                                             (set-MMU-instruction---mstore8    stack-ram---row-offset---misc-row           ;; offset
                                                                               ;; src_id                              ;; source ID
                                                                               CONTEXT_NUMBER                      ;; target ID
                                                                               ;; aux_id                              ;; auxiliary ID
                                                                               ;; src_offset_hi                       ;; source offset high
                                                                               ;; src_offset_lo                       ;; source offset low
                                                                               (stack-ram---offset-lo)          ;; target offset low
                                                                               ;; size                                ;; size
                                                                               ;; ref_offset                          ;; reference offset
                                                                               ;; ref_size                            ;; reference size
                                                                               ;; success_bit                         ;; success bit
                                                                               (stack-ram---value-hi)           ;; limb 1
                                                                               (stack-ram---value-lo)           ;; limb 2
                                                                               ;; exo_sum                             ;; weighted exogenous module flag sum
                                                                               ;; phase                               ;; phase
                                                                               ))))
