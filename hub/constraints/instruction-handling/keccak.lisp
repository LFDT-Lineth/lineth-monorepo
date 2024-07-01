(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                               ;;;;
;;;;    X.5 Instruction handling   ;;;;
;;;;                               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    X.5.10 Keccak   ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;  Shorthands  ;;
;;;;;;;;;;;;;;;;;;
(defun (keccak-offset-hi)
  [ stack/STACK_ITEM_VALUE_HI 1 ])

(defun (keccak-offset-lo)
  [ stack/STACK_ITEM_VALUE_LO 1 ])

(defun (keccak-size-hi)
  [ stack/STACK_ITEM_VALUE_HI 2 ])

(defun (keccak-size-lo)
  [ stack/STACK_ITEM_VALUE_LO 2 ])

(defun (keccak-result-hi)
  [ stack/STACK_ITEM_VALUE_HI 4 ])

(defun (keccak-result-lo)
  [ stack/STACK_ITEM_VALUE_LO 4 ])

(defun (keccak-mxpx)
  (next misc/MXP_MXPX))

(defun (keccak-mxp-gas)
  (next misc/MXP_GAS_MXP))

(defun (keccak-mxp-MTNTOP)
  (next misc/MXP_MTNTOP))

(defun (keccak-trigger-MMU)
  (* (- 1 XAHOY) (keccak-mxp-MTNTOP)))

(defun (keccak-no-stack-exceptions)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (* PEEK_AT_STACK stack/KEC_FLAG (- 1 stack/SUX stack/SOX)))

(defconstraint keccak-stack-pattern (:guard (keccak-no-stack-exceptions))
  (stack-pattern-2-1))

(defconstraint keccak-NSR-and-peeking-flags (:guard (keccak-no-stack-exceptions))
  (begin (eq! NON_STACK_ROWS (+ 1 CONTEXT_MAY_CHANGE))
         (eq! NON_STACK_ROWS
              (+ (shift PEEK_AT_MISCELLANEOUS 1)
                 (* (shift PEEK_AT_CONTEXT 2) CONTEXT_MAY_CHANGE)))))

(defconstraint keccak-MISC-flags (:guard (keccak-no-stack-exceptions))
  (eq! (weighted-MISC-flag-sum 1)
       (+ MISC_WEIGHT_MXP
          (* MISC_WEIGHT_MMU (keccak-trigger-MMU)))))

(defconstraint keccak-MXP-call (:guard (keccak-no-stack-exceptions))
  (set-MXP-instruction-type-4 1                  ;; row offset kappa
                              EVM_INST_SHA3      ;; instruction
                              0                  ;; deploys (bit modifying the behaviour of RETURN pricing)
                              (keccak-offset-hi) ;; source offset high
                              (keccak-offset-lo) ;; source offset low
                              (keccak-size-hi)   ;; source size high
                              (keccak-size-lo))) ;; source size low

(defconstraint keccak-MMU-call (:guard (keccak-no-stack-exceptions))
               (if-not-zero misc/MMU_FLAG
                            (set-MMU-instruction-ram-to-exo-with-padding    1                  ;; offset
                                                                            CN                 ;; source ID
                                                                            0                  ;; target ID
                                                                            (+ 1 HUB_STAMP)    ;; auxiliary ID
                                                                            ;; src_offset_hi       ;; source offset high
                                                                            (keccak-offset-lo) ;; source offset low
                                                                            ;; tgt_offset_lo       ;; target offset low
                                                                            (keccak-size-lo)   ;; size
                                                                            ;; ref_offset          ;; reference offset
                                                                            (keccak-size-lo)   ;; reference size
                                                                            0                  ;; success bit
                                                                            ;; limb_1              ;; limb 1
                                                                            ;; limb_2              ;; limb 2
                                                                            EXO_SUM_WEIGHT_KEC ;; weighted exogenous module flag sum
                                                                            0)))               ;; phase

(defconstraint keccak-transferring-MXPX-to-stack (:guard (keccak-no-stack-exceptions))
  (eq! stack/MXPX (keccak-mxpx)))

(defconstraint keccak-setting-gas-cost (:guard (keccak-no-stack-exceptions))
  ;; (if-zero (force-bin (keccak-mxpx))
  (if-zero (keccak-mxpx)
           (eq! GAS_COST (+ stack/STATIC_GAS (keccak-mxp-gas)))
           (vanishes! GAS_COST)))

(defconstraint keccak-setting-HASH_INFO_FLAG (:guard (keccak-no-stack-exceptions))
  (eq! stack/HASH_INFO_FLAG (keccak-trigger-MMU)))

(defconstraint keccak-value-constraints (:guard (keccak-no-stack-exceptions))
  (if-zero XAHOY
           ;; (if-zero (force-bin (keccak-trigger-MMU))
           (if-zero (keccak-trigger-MMU)
                    (begin (eq! (keccak-result-hi) EMPTY_KECCAK_HI)
                           (eq! (keccak-result-lo) EMPTY_KECCAK_LO))
                    (begin (eq! (keccak-result-hi) stack/HASH_INFO_KECCAK_HI)
                           (eq! (keccak-result-lo) stack/HASH_INFO_KECCAK_LO)))))

