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
  ROFF_STACK_RAM___MISC_ROW    1
  ROFF_STACK_RAM___CONTEXT_ROW 2)

(defun    (stack-ram---instruction)                  stack/INSTRUCTION)
(defun    (stack-ram---is-CDL)                     [ stack/DEC_FLAG 1 ])
(defun    (stack-ram---is-MLOAD)                   [ stack/DEC_FLAG 2 ])
(defun    (stack-ram---is-MSTORE)                  [ stack/DEC_FLAG 3 ])
(defun    (stack-ram---is-MSTORE8)                 [ stack/DEC_FLAG 4 ]) ;; ""
(defun    (stack-ram---is-store-instruction)       (+                         (stack-ram---is-MSTORE) (stack-ram---is-MSTORE8)))
(defun    (stack-ram---is-MXX)                     (+ (stack-ram---is-MLOAD)  (stack-ram---is-MSTORE) (stack-ram---is-MSTORE8)))
(defun    (stack-ram---offset-hi)                  [ stack/STACK_ITEM_VALUE_HI 1 ])
(defun    (stack-ram---offset-lo)                  [ stack/STACK_ITEM_VALUE_LO 1 ])
(defun    (stack-ram---value-hi)                   [ stack/STACK_ITEM_VALUE_HI 4 ])
(defun    (stack-ram---value-lo)                   [ stack/STACK_ITEM_VALUE_LO 4 ])
(defun    (stack-ram---CDL-is-oob)                 (shift   [ misc/OOB_DATA 7 ]                  ROFF_STACK_RAM___MISC_ROW)) ;; ""
(defun    (stack-ram---MXP-mxp-gas)                (shift     misc/MXP_GAS_MXP                   ROFF_STACK_RAM___MISC_ROW))
(defun    (stack-ram---MXP-mxpx)                   (shift     misc/MXP_MXPX                      ROFF_STACK_RAM___MISC_ROW))
(defun    (stack-ram---call-data-size)             (shift     context/CALL_DATA_SIZE             ROFF_STACK_RAM___CONTEXT_ROW))
(defun    (stack-ram---call-data-offset)           (shift     context/CALL_DATA_OFFSET           ROFF_STACK_RAM___CONTEXT_ROW))
(defun    (stack-ram---call-data-context-number)   (shift     context/CALL_DATA_CONTEXT_NUMBER   ROFF_STACK_RAM___CONTEXT_ROW))
(defun    (stack-ram---fixed-size)                 (+   (*   WORD_SIZE   (stack-ram---is-MLOAD)   )
                                                        (*   WORD_SIZE   (stack-ram---is-MSTORE)  )
                                                        (*           1   (stack-ram---is-MSTORE8) )))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.5.3 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (stack-ram---std-hyp) (* PEEK_AT_STACK
                                stack/STACKRAM_FLAG
                                (- 1 stack/SUX stack/SOX)))

(defconstraint   stack-ram---setting-the-stack-pattern
                 (:guard (stack-ram---std-hyp))
                 (load-store-stack-pattern         (force-bin (stack-ram---is-store-instruction))))

(defconstraint   stack-ram---allowable-exceptions
                 (:guard (stack-ram---std-hyp))
                 (begin
                   (if-not-zero    (stack-ram---is-MXX) (eq!    XAHOY    (+   stack/MXPX    stack/OOGX)))
                   (if-not-zero    (stack-ram---is-CDL) (eq!    XAHOY                       stack/OOGX))))

(defconstraint   stack-ram---setting-NSR
                 (:guard (stack-ram---std-hyp))
                 (eq! NSR
                      (+ 1 (stack-ram---is-CDL) CMC)))

(defconstraint   stack-ram---setting-the-peeking-flags
                 (:guard (stack-ram---std-hyp))
                 (begin (if-not-zero (stack-ram---is-CDL)
                                     (eq! NSR
                                          (+ (shift PEEK_AT_MISCELLANEOUS   ROFF_STACK_RAM___MISC_ROW)
                                             (shift PEEK_AT_CONTEXT         ROFF_STACK_RAM___CONTEXT_ROW)
                                             (* (shift PEEK_AT_CONTEXT 3) CMC))))
                        (if-not-zero (stack-ram---is-MXX)
                                     (eq! NSR
                                          (+ (shift PEEK_AT_MISCELLANEOUS   ROFF_STACK_RAM___MISC_ROW)
                                             (* (shift PEEK_AT_CONTEXT 2) CMC))))
                        (debug (eq! CMC XAHOY))))

(defconstraint   stack-ram---setting-the-memory-expansion-exception
                 (:guard (stack-ram---std-hyp))
                 (begin (if-not-zero (stack-ram---is-CDL)
                                     (vanishes! stack/MXPX))
                        (if-not-zero (stack-ram---is-MXX)
                                     (eq! stack/MXPX (stack-ram---MXP-mxpx)))))

(defconstraint   stack-ram---setting-the-gas-cost
                 (:guard (stack-ram---std-hyp))
                 (begin (if-not-zero (stack-ram---is-CDL)
                                     (eq! GAS_COST stack/STATIC_GAS))
                        (if-not-zero (stack-ram---is-MXX)
                                     (if-zero (force-bin (stack-ram---MXP-mxpx))
                                              (eq! GAS_COST
                                                   (+ stack/STATIC_GAS
                                                      (stack-ram---MXP-mxp-gas)))
                                              (vanishes! GAS_COST)))))

(defconstraint   stack-ram---setting-MISC-module-flags
                 (:guard (stack-ram---std-hyp))
                 (begin
                   (eq!    (weighted-MISC-flag-sum-sans-MMU       ROFF_STACK_RAM___MISC_ROW)
                           (+   (*   MISC_WEIGHT_MXP   (stack-ram---trigger_MXP))
                                (*   MISC_WEIGHT_OOB   (stack-ram---trigger_OOB))))
                   (eq!    (shift    misc/MMU_FLAG    ROFF_STACK_RAM___MISC_ROW)    (stack-ram---trigger_MMU))
                   ))

(defun    (stack-ram---trigger_MXP)     (stack-ram---is-MXX))
(defun    (stack-ram---trigger_OOB)     (stack-ram---is-CDL))
(defun    (stack-ram---trigger_MMU)     (+   (*   (stack-ram---is-CDL)   (- 1 XAHOY)   (- 1 (stack-ram---CDL-is-oob)))
                                             (*   (stack-ram---is-MXX)   (- 1 XAHOY)                                 )))

;; shorthands specific to the constraints
(defun    (stack-ram---misc-MXP-flag)   (shift   misc/MXP_FLAG   ROFF_STACK_RAM___MISC_ROW))
(defun    (stack-ram---misc-OOB-flag)   (shift   misc/OOB_FLAG   ROFF_STACK_RAM___MISC_ROW))
(defun    (stack-ram---misc-MMU-flag)   (shift   misc/MMU_FLAG   ROFF_STACK_RAM___MISC_ROW))

(defconstraint   stack-ram---setting-OOB-instruction
                 (:guard (stack-ram---std-hyp))
                 (if-not-zero   (stack-ram---misc-OOB-flag)
                                (set-OOB-instruction---cdl     ROFF_STACK_RAM___MISC_ROW     ;; row offset
                                                               (stack-ram---offset-hi)       ;; offset within call data, high part
                                                               (stack-ram---offset-lo)       ;; offset within call data, low  part
                                                               (stack-ram---call-data-size)  ;; call data size
                                                               )))

(defconstraint   stack-ram---setting-value-for-trivial-CALLDATALOAD
                 (:guard (stack-ram---std-hyp))
                 (if-not-zero (stack-ram---is-CDL)
                              (if-not-zero (stack-ram---CDL-is-oob)
                                           (begin
                                             (vanishes! (stack-ram---value-hi))
                                             (vanishes! (stack-ram---value-lo))))))

(defconstraint   stack-ram---setting-MXP-instruction
                 (:guard (stack-ram---std-hyp))
                 (if-not-zero    (stack-ram---misc-MXP-flag)
                                 (set-MXP-instruction---single-mxp-offset-instructions   ROFF_STACK_RAM___MISC_ROW   ;; row offset
                                                                                         (stack-ram---instruction)   ;; instruction
                                                                                         0                           ;; deploys
                                                                                         (stack-ram---offset-hi)     ;; source offset high
                                                                                         (stack-ram---offset-lo)     ;; source offset low
                                                                                         0                           ;; size high
                                                                                         (stack-ram---fixed-size)    ;; size low
                                                                                         )))

(defconstraint   stack-ram---setting-MMU-instruction---CALLDATALOAD-case
                 (:guard (stack-ram---std-hyp))
                 (if-not-zero    (stack-ram---misc-MMU-flag)
                                ;; CALLDATALOAD case
                                ;;;;;;;;;;;;;;;;;;;;
                                (if-not-zero (stack-ram---is-CDL)
                                             (set-MMU-instruction---right-padded-word-extraction    ROFF_STACK_RAM___MISC_ROW              ;; row offset
                                                                                                    (stack-ram---call-data-context-number) ;; source ID
                                                                                                    ;; tgt_id                              ;; target ID
                                                                                                    ;; aux_id                              ;; auxiliary ID
                                                                                                    ;; src_offset_hi                       ;; source offset high
                                                                                                    (stack-ram---offset-lo)                ;; source offset low
                                                                                                    ;; tgt_offset_lo                       ;; target offset low
                                                                                                    ;; size                                ;; size
                                                                                                    (stack-ram---call-data-offset)         ;; reference offset
                                                                                                    (stack-ram---call-data-size)           ;; reference size
                                                                                                    ;; success_bit                         ;; success bit
                                                                                                    (stack-ram---value-hi)                 ;; limb 1
                                                                                                    (stack-ram---value-lo)                 ;; limb 2
                                                                                                    ;; exo_sum                             ;; weighted exogenous module flag sum
                                                                                                    ;; phase                               ;; phase
                                                                                                    ))))

(defconstraint   stack-ram---setting-MMU-instruction---MLOAD-case
                 (:guard (stack-ram---std-hyp))
                 (if-not-zero    (stack-ram---misc-MMU-flag)
                                ;; MLOAD case
                                ;;;;;;;;;;;;;
                                (if-not-zero (stack-ram---is-MLOAD)
                                             (set-MMU-instruction---mload    ROFF_STACK_RAM___MISC_ROW   ;; offset
                                                                             CONTEXT_NUMBER              ;; source ID
                                                                             ;; tgt_id                   ;; target ID
                                                                             ;; aux_id                   ;; auxiliary ID
                                                                             ;; src_offset_hi            ;; source offset high
                                                                             (stack-ram---offset-lo)     ;; source offset low
                                                                             ;; tgt_offset_lo            ;; target offset low
                                                                             ;; size                     ;; size
                                                                             ;; ref_offset               ;; reference offset
                                                                             ;; ref_size                 ;; reference size
                                                                             ;; success_bit              ;; success bit
                                                                             (stack-ram---value-hi)      ;; limb 1
                                                                             (stack-ram---value-lo)      ;; limb 2
                                                                             ;; exo_sum                  ;; weighted exogenous module flag sum
                                                                             ;; phase                    ;; phase
                                                                             ))))

(defconstraint   stack-ram---setting-MMU-instruction---MSTORE-case
                 (:guard (stack-ram---std-hyp))
                 (if-not-zero    (stack-ram---misc-MMU-flag)
                                ;; MSTORE case
                                ;;;;;;;;;;;;;;
                                (if-not-zero (stack-ram---is-MSTORE)
                                             (set-MMU-instruction---mstore    ROFF_STACK_RAM___MISC_ROW   ;; offset
                                                                              ;; src_id                   ;; source ID
                                                                              CONTEXT_NUMBER              ;; target ID
                                                                              ;; aux_id                   ;; auxiliary ID
                                                                              ;; src_offset_hi            ;; source offset high
                                                                              ;; src_offset_lo            ;; source offset low
                                                                              (stack-ram---offset-lo)     ;; target offset low
                                                                              ;; size                     ;; size
                                                                              ;; ref_offset               ;; reference offset
                                                                              ;; ref_size                 ;; reference size
                                                                              ;; success_bit              ;; success bit
                                                                              (stack-ram---value-hi)      ;; limb 1
                                                                              (stack-ram---value-lo)      ;; limb 2
                                                                              ;; exo_sum                  ;; weighted exogenous module flag sum
                                                                              ;; phase                    ;; phase
                                                                              ))))

(defconstraint   stack-ram---setting-MMU-instruction---MSTORE8-case
                 (:guard (stack-ram---std-hyp))
                 (if-not-zero    (stack-ram---misc-MMU-flag)
                                ;; MSTORE8 case
                                ;;;;;;;;;;;;;;;
                                (if-not-zero (stack-ram---is-MSTORE8)
                                             (set-MMU-instruction---mstore8    ROFF_STACK_RAM___MISC_ROW   ;; offset
                                                                               ;; src_id                   ;; source ID
                                                                               CONTEXT_NUMBER              ;; target ID
                                                                               ;; aux_id                   ;; auxiliary ID
                                                                               ;; src_offset_hi            ;; source offset high
                                                                               ;; src_offset_lo            ;; source offset low
                                                                               (stack-ram---offset-lo)     ;; target offset low
                                                                               ;; size                     ;; size
                                                                               ;; ref_offset               ;; reference offset
                                                                               ;; ref_size                 ;; reference size
                                                                               ;; success_bit              ;; success bit
                                                                               (stack-ram---value-hi)      ;; limb 1
                                                                               (stack-ram---value-lo)      ;; limb 2
                                                                               ;; exo_sum                  ;; weighted exogenous module flag sum
                                                                               ;; phase                    ;; phase
                                                                               ))))

(defconstraint   stack-ram---setting-context-row-for-CALLDATALOAD
                 (:guard (stack-ram---std-hyp))
                 (if-not-zero    (stack-ram---is-CDL)
                                 (read-context-data ROFF_STACK_RAM___CONTEXT_ROW CONTEXT_NUMBER)))
