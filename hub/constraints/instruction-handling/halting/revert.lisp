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
  ROW_OFFSET_REVERT_MISCELLANEOUS_ROW              1
  ;;
  ROW_OFFSET_REVERT_XAHOY_CALLER_CONTEXT_ROW       2
  ;;
  ROW_OFFSET_REVERT_NO_XAHOY_CURRENT_CONTEXT_ROW   2
  ROW_OFFSET_REVERT_NO_XAHOY_CALLER_CONTEXT_ROW    3)



(defun  (revert-instruction---instruction)                stack/INSTRUCTION)
(defun  (revert-instruction---offset-hi)                  [ stack/STACK_ITEM_VALUE_HI 1 ])
(defun  (revert-instruction---offset-lo)                  [ stack/STACK_ITEM_VALUE_LO 1 ])
(defun  (revert-instruction---size-hi)                    [ stack/STACK_ITEM_VALUE_HI 2 ])
(defun  (revert-instruction---size-lo)                    [ stack/STACK_ITEM_VALUE_LO 2 ])
(defun  (revert-instruction---current-context)            CONTEXT_NUMBER)
(defun  (revert-instruction---caller-context)             CALLER_CONTEXT_NUMBER)
(defun  (revert-instruction---MXP-memory-expansion-gas)   (shift  misc/MXP_GAS_MXP             ROW_OFFSET_REVERT_MISCELLANEOUS_ROW))
(defun  (revert-instruction---current-context-is-root)    (shift  context/IS_ROOT              ROW_OFFSET_REVERT_NO_XAHOY_CURRENT_CONTEXT_ROW))
(defun  (revert-instruction---r@o)                        (shift  context/RETURN_AT_OFFSET     ROW_OFFSET_REVERT_NO_XAHOY_CURRENT_CONTEXT_ROW))
(defun  (revert-instruction---r@c)                        (shift  context/RETURN_AT_CAPACITY   ROW_OFFSET_REVERT_NO_XAHOY_CURRENT_CONTEXT_ROW))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.5.1 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun  (revert-instruction---standard-precondition)  (*  PEEK_AT_STACK
                                                 stack/HALT_FLAG
                                                 [ stack/DEC_FLAG  2 ]
                                                 (-  1  stack/SUX  stack/SOX )))

(defconstraint  revert-instruction---setting-the-stack-pattern                     (:guard (revert-instruction---standard-precondition))
                (stack-pattern-2-0))

(defconstraint  revert-instruction---allowable-exceptions                          (:guard (revert-instruction---standard-precondition))
                (eq!  XAHOY
                      (+  stack/MXPX
                          stack/OOGX)))

(defconstraint  revert-instruction---setting-NSR                                   (:guard (revert-instruction---standard-precondition))
                (eq! NSR
                     (-  3  XAHOY)))

(defconstraint  revert-instruction---setting-the-peeking-flags                     (:guard (revert-instruction---standard-precondition))
                (if-not-zero  XAHOY
                              ;; XAHOY ≡ 1
                              (eq!  NSR
                                    (+  (shift  PEEK_AT_MISCELLANEOUS    ROW_OFFSET_REVERT_MISCELLANEOUS_ROW            )
                                        (shift  PEEK_AT_CONTEXT          ROW_OFFSET_REVERT_XAHOY_CALLER_CONTEXT_ROW     )))
                              ;; XAHOY ≡ 0
                              (eq! NSR
                                   (+  (shift  PEEK_AT_MISCELLANEOUS   ROW_OFFSET_REVERT_MISCELLANEOUS_ROW            )
                                       (shift  PEEK_AT_CONTEXT         ROW_OFFSET_REVERT_NO_XAHOY_CURRENT_CONTEXT_ROW )
                                       (shift  PEEK_AT_CONTEXT         ROW_OFFSET_REVERT_NO_XAHOY_CALLER_CONTEXT_ROW  )))))

(defconstraint  revert-instruction---setting-the-context-rows                      (:guard (revert-instruction---standard-precondition))
                (if-not-zero  XAHOY
                              ;; XAHOY ≡ 1
                              (execution-provides-empty-return-data      ROW_OFFSET_REVERT_XAHOY_CALLER_CONTEXT_ROW)
                              ;; XAHOY ≡ 0
                              (begin
                                (read-context-data   ROW_OFFSET_REVERT_NO_XAHOY_CURRENT_CONTEXT_ROW
                                                     (revert-instruction---current-context))
                                (if-not-zero   (force-bin (revert-instruction---current-context-is-root))
                                               ;; current context IS root
                                               (read-context-data    ROW_OFFSET_REVERT_NO_XAHOY_CALLER_CONTEXT_ROW
                                                                     (revert-instruction---caller-context)))
                                ;; current context ISN'T root
                                (provide-return-data   ROW_OFFSET_REVERT_NO_XAHOY_CALLER_CONTEXT_ROW      ;; row offset
                                                       (revert-instruction---caller-context)                       ;; receiver context
                                                       (revert-instruction---current-context)                      ;; provider context
                                                       (revert-instruction---offset-lo)                            ;; rdo
                                                       (revert-instruction---size-lo)                              ;; rds
                                                       ))))

(defun  (revert-instruction---trigger_MMU)  (*  (-  1  XAHOY)
                                       (-  1  (revert-instruction---current-context-is-root))
                                       (is-not-zero (*  (revert-instruction---size-lo)
                                                        (revert-instruction---r@c)))))

(defconstraint  revert-instruction---setting-the-miscellaneous-row-module-flags    (:guard (revert-instruction---standard-precondition))
                (eq!  (weighted-MISC-flag-sum  ROW_OFFSET_REVERT_MISCELLANEOUS_ROW)
                      (+  MISC_WEIGHT_MXP
                          (*  MISC_WEIGHT_MMU  (revert-instruction---trigger_MMU)))))

(defconstraint  revert-instruction---setting-the-MXP-data                          (:guard (revert-instruction---standard-precondition))
                (set-MXP-instruction-type-4 ROW_OFFSET_REVERT_MISCELLANEOUS_ROW   ;; row offset kappa
                                            (revert-instruction---instruction)             ;; instruction
                                            0                                     ;; bit modifying the behaviour of RETURN pricing
                                            (revert-instruction---offset-hi)               ;; offset high
                                            (revert-instruction---offset-lo)               ;; offset low
                                            (revert-instruction---size-hi)                 ;; size high
                                            (revert-instruction---size-lo)))               ;; size low

(defconstraint  revert-instruction---setting-the-MXPX                              (:guard (revert-instruction---standard-precondition))
                (eq!  stack/MXPX  (shift  misc/MXP_MXPX  ROW_OFFSET_REVERT_MISCELLANEOUS_ROW)))

(defconstraint  revert-instruction---setting-the-MMU-data                          (:guard (revert-instruction---standard-precondition))
                (if-not-zero  (shift  misc/MMU_FLAG  ROW_OFFSET_REVERT_MISCELLANEOUS_ROW)
                              (set-MMU-instruction---ram-to-ram-sans-padding    ROW_OFFSET_REVERT_MISCELLANEOUS_ROW  ;; row offset
                                                                                (revert-instruction---current-context)        ;; source ID
                                                                                (revert-instruction---caller-context)         ;; target ID
                                                                                ;; aux_id                               ;; auxiliary ID
                                                                                ;; src_offset_hi                        ;; source offset high
                                                                                (revert-instruction---offset-lo)              ;; source offset low
                                                                                ;; tgt_offset_lo                        ;; target offset low
                                                                                (revert-instruction---size-lo)                ;; size
                                                                                (revert-instruction---r@o)                    ;; reference offset
                                                                                (revert-instruction---r@c)                    ;; reference size
                                                                                ;; success_bit                          ;; success bit
                                                                                ;; limb_1                               ;; limb 1
                                                                                ;; limb_2                               ;; limb 2
                                                                                ;; exo_sum                              ;; weighted exogenous module flag sum
                                                                                ;; phase                                ;; phase
                                                                                )))

(defconstraint  revert-instruction---setting-the-gas-cost                          (:guard (revert-instruction---standard-precondition))
                (if-not-zero  stack/MXPX
                              (vanishes!  GAS_COST)
                              (eq!  GAS_COST
                                    (+  stack/STATIC_GAS
                                        (revert-instruction---MXP-memory-expansion-gas)))))
