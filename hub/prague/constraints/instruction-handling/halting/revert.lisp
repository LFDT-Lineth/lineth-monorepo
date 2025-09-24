(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.5 REVERT   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;    X.5.1 Introduction   ;;
;;    X.5.2 Shorthands     ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconst
  ROFF_REVERT___MISC_ROW                       1
  ;;
  ROFF_REVERT___XAHOY_CALLER_CONTEXT_ROW       2
  ;;
  ROFF_REVERT___NO_XAHOY_CURRENT_CONTEXT_ROW   2
  ROFF_REVERT___NO_XAHOY_CALLER_CONTEXT_ROW    3)



(defun  (revert-instruction---instruction)                         stack/INSTRUCTION)
(defun  (revert-instruction---offset-hi)                           [ stack/STACK_ITEM_VALUE_HI 1 ])
(defun  (revert-instruction---offset-lo)                           [ stack/STACK_ITEM_VALUE_LO 1 ])
(defun  (revert-instruction---size-hi)                             [ stack/STACK_ITEM_VALUE_HI 2 ])
(defun  (revert-instruction---size-lo)                             [ stack/STACK_ITEM_VALUE_LO 2 ])

(defun  (revert-instruction---current-context)                     CONTEXT_NUMBER)
(defun  (revert-instruction---caller-context)                      CALLER_CONTEXT_NUMBER)
(defun  (revert-instruction---MXP-memory-expansion-gas)            (shift   misc/MXP_GAS_MXP                  ROFF_REVERT___MISC_ROW))
(defun  (revert-instruction---MXP-size-1-is-nonzero-and-no-mxpx)   (shift   misc/MXP_SIZE_1_NONZERO_NO_MXPX   ROFF_REVERT___MISC_ROW))
(defun  (revert-instruction---current-context-is-root)             (shift   context/IS_ROOT                   ROFF_REVERT___NO_XAHOY_CURRENT_CONTEXT_ROW))
(defun  (revert-instruction---r@o)                                 (shift   context/RETURN_AT_OFFSET          ROFF_REVERT___NO_XAHOY_CURRENT_CONTEXT_ROW))
(defun  (revert-instruction---r@c)                                 (shift   context/RETURN_AT_CAPACITY        ROFF_REVERT___NO_XAHOY_CURRENT_CONTEXT_ROW))

(defun  (revert-instruction---type-safe-return-data-offset)        (*       (revert-instruction---offset-lo)  (revert-instruction---MXP-size-1-is-nonzero-and-no-mxpx)))
(defun  (revert-instruction---type-safe-return-data-size)          (revert-instruction---size-lo)) ;; ""

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.5.1 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun  (revert-instruction---standard-precondition)  (*  PEEK_AT_STACK
                                                          stack/HALT_FLAG
                                                          (halting-instruction---is-REVERT)
                                                          (-  1  stack/SUX  stack/SOX )))

(defconstraint  revert-instruction---setting-the-stack-pattern
                (:guard (revert-instruction---standard-precondition))
                (stack-pattern-2-0))

(defconstraint  revert-instruction---allowable-exceptions
                (:guard (revert-instruction---standard-precondition))
                (eq!  XAHOY
                      (+  stack/MXPX
                          stack/OOGX)))

(defconstraint  revert-instruction---setting-NSR
                (:guard (revert-instruction---standard-precondition))
                (eq! NSR
                     (-  3  XAHOY)))

(defconstraint  revert-instruction---setting-the-peeking-flags
                (:guard (revert-instruction---standard-precondition))
                (if-not-zero  XAHOY
                              ;; XAHOY ≡ 1
                              (eq!  NSR
                                    (+  (shift  PEEK_AT_MISCELLANEOUS    ROFF_REVERT___MISC_ROW                )
                                        (shift  PEEK_AT_CONTEXT          ROFF_REVERT___XAHOY_CALLER_CONTEXT_ROW)))
                              ;; XAHOY ≡ 0
                              (eq! NSR
                                   (+  (shift  PEEK_AT_MISCELLANEOUS   ROFF_REVERT___MISC_ROW                    )
                                       (shift  PEEK_AT_CONTEXT         ROFF_REVERT___NO_XAHOY_CURRENT_CONTEXT_ROW)
                                       (shift  PEEK_AT_CONTEXT         ROFF_REVERT___NO_XAHOY_CALLER_CONTEXT_ROW )))))

(defconstraint  revert-instruction---setting-the-context-rows---exceptional
                (:guard (revert-instruction---standard-precondition))
                (if-not-zero  XAHOY
                              (execution-provides-empty-return-data      ROFF_REVERT___XAHOY_CALLER_CONTEXT_ROW)))

(defconstraint  revert-instruction---setting-the-context-rows---unexceptional
                (:guard (revert-instruction---standard-precondition))
                (if-zero      XAHOY
                              (provide-return-data   ROFF_REVERT___NO_XAHOY_CALLER_CONTEXT_ROW             ;; row offset
                                                     (revert-instruction---caller-context)                 ;; receiver context
                                                     (revert-instruction---current-context)                ;; provider context
                                                     (revert-instruction---type-safe-return-data-offset)   ;; type safe rdo
                                                     (revert-instruction---type-safe-return-data-size)     ;; type safe rds
                                                     )))

(defconstraint  revert-instruction---setting-the-miscellaneous-row-module-flags
                (:guard (revert-instruction---standard-precondition))
  (let ((FLAG (weighted-MISC-flag-sum  ROFF_REVERT___MISC_ROW)))
    ;;
    (if (or!
         (eq! XAHOY 1)
         (eq! (revert-instruction---current-context-is-root) 1)
         (eq! (revert-instruction---size-lo) 0)
         (eq! (revert-instruction---r@c) 0))
        ;; trigger_MMU == 0
        (eq! FLAG MISC_WEIGHT_MXP)
        ;; trigger_MMU == 1
        (eq! FLAG (+ MISC_WEIGHT_MXP MISC_WEIGHT_MMU)))))

(defconstraint  revert-instruction---setting-the-MXP-instruction
                (:guard (revert-instruction---standard-precondition))
                (set-MXP-instruction---single-mxp-offset-instructions   ROFF_REVERT___MISC_ROW                 ;; row offset kappa
                                                                        (revert-instruction---instruction)     ;; instruction
                                                                        0                                      ;; bit modifying the behaviour of RETURN pricing
                                                                        (revert-instruction---offset-hi)       ;; offset high
                                                                        (revert-instruction---offset-lo)       ;; offset low
                                                                        (revert-instruction---size-hi)         ;; size high
                                                                        (revert-instruction---size-lo)))       ;; size low

(defconstraint  revert-instruction---setting-the-MXPX
                (:guard (revert-instruction---standard-precondition))
                (eq!  stack/MXPX  (shift  misc/MXP_MXPX  ROFF_REVERT___MISC_ROW)))

(defconstraint  revert-instruction---setting-the-MMU-data
                (:guard (revert-instruction---standard-precondition))
                (if-not-zero  (shift  misc/MMU_FLAG  ROFF_REVERT___MISC_ROW)
                              (set-MMU-instruction---ram-to-ram-sans-padding    ROFF_REVERT___MISC_ROW  ;; row offset
                                                                                (revert-instruction---current-context)        ;; source ID
                                                                                (revert-instruction---caller-context)         ;; target ID
                                                                                ;; aux_id                                        ;; auxiliary ID
                                                                                ;; src_offset_hi                                 ;; source offset high
                                                                                (revert-instruction---offset-lo)              ;; source offset low
                                                                                ;; tgt_offset_lo                                 ;; target offset low
                                                                                (revert-instruction---size-lo)                ;; size
                                                                                (revert-instruction---r@o)                    ;; reference offset
                                                                                (revert-instruction---r@c)                    ;; reference size
                                                                                ;; success_bit                                   ;; success bit
                                                                                ;; limb_1                                        ;; limb 1
                                                                                ;; limb_2                                        ;; limb 2
                                                                                ;; exo_sum                                       ;; weighted exogenous module flag sum
                                                                                ;; phase                                         ;; phase
                                                                                )))

(defconstraint  revert-instruction---setting-the-gas-cost
                (:guard (revert-instruction---standard-precondition))
                (if-not-zero  stack/MXPX
                              (vanishes!  GAS_COST)
                              (eq!  GAS_COST
                                    (+  stack/STATIC_GAS
                                        (revert-instruction---MXP-memory-expansion-gas)))))
