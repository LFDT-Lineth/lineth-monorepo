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



(defun  (revert-inst-instruction)                stack/INSTRUCTION)
(defun  (revert-inst-offset-hi)                  [ stack/STACK_ITEM_VALUE_HI 1 ])
(defun  (revert-inst-offset-lo)                  [ stack/STACK_ITEM_VALUE_LO 1 ])
(defun  (revert-inst-size-hi)                    [ stack/STACK_ITEM_VALUE_HI 2 ])
(defun  (revert-inst-size-lo)                    [ stack/STACK_ITEM_VALUE_LO 2 ])
(defun  (revert-inst-current-context)            CONTEXT_NUMBER)
(defun  (revert-inst-caller-context)             CALLER_CONTEXT_NUMBER)
(defun  (revert-inst-MXP-memory-expansion-gas)   (shift  misc/MXP_GAS_MXP             ROW_OFFSET_REVERT_MISCELLANEOUS_ROW))
(defun  (revert-inst-current-context-is-root)    (shift  context/IS_ROOT              ROW_OFFSET_REVERT_NO_XAHOY_CURRENT_CONTEXT_ROW))
(defun  (revert-inst-r@o)                        (shift  context/RETURN_AT_OFFSET     ROW_OFFSET_REVERT_NO_XAHOY_CURRENT_CONTEXT_ROW))
(defun  (revert-inst-r@c)                        (shift  context/RETURN_AT_CAPACITY   ROW_OFFSET_REVERT_NO_XAHOY_CURRENT_CONTEXT_ROW))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.5.1 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun  (revert-inst-standard-precondition)  (*  PEEK_AT_STACK
                                                 stack/HALT_FLAG
                                                 [ stack/DEC_FLAG  2 ]
                                                 (-  1  stack/SUX  stack/SOX )))

(defconstraint  revert-inst-setting-the-stack-pattern                     (:guard (revert-inst-standard-precondition))
                (stack-pattern-2-0))

(defconstraint  revert-inst-allowable-exceptions                          (:guard (revert-inst-standard-precondition))
                (eq!  XAHOY
                      (+  stack/MXPX
                          stack/OOGX)))

(defconstraint  revert-inst-setting-NSR                                   (:guard (revert-inst-standard-precondition))
                (eq! NSR
                     (-  3  XAHOY)))

(defconstraint  revert-inst-setting-the-peeking-flags                     (:guard (revert-inst-standard-precondition))
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

(defconstraint  revert-inst-setting-the-context-rows                      (:guard (revert-inst-standard-precondition))
                (if-not-zero  XAHOY
                              ;; XAHOY ≡ 1
                              (execution-provides-empty-return-data      ROW_OFFSET_REVERT_XAHOY_CALLER_CONTEXT_ROW)
                              ;; XAHOY ≡ 0
                              (begin
                                (read-context-data   ROW_OFFSET_REVERT_NO_XAHOY_CURRENT_CONTEXT_ROW
                                                     (revert-inst-current-context))
                                (if-not-zero   (force-bin (revert-inst-current-context-is-root))
                                               ;; current context IS root
                                               (read-context-data    ROW_OFFSET_REVERT_NO_XAHOY_CALLER_CONTEXT_ROW
                                                                     (revert-inst-caller-context)))
                                ;; current context ISN'T root
                                (provide-return-data   ROW_OFFSET_REVERT_NO_XAHOY_CALLER_CONTEXT_ROW      ;; row offset
                                                       (revert-inst-caller-context)                       ;; receiver context
                                                       (revert-inst-current-context)                      ;; provider context
                                                       (revert-inst-offset-lo)                            ;; rdo
                                                       (revert-inst-size-lo)                              ;; rds
                                                       ))))

(defun  (revert-inst-trigger_MMU)  (*  (-  1  XAHOY)
                                       (-  1  (revert-inst-current-context-is-root))
                                       (is-not-zero (*  (revert-inst-size-lo)
                                                        (revert-inst-r@c)))))

(defconstraint  revert-inst-setting-the-miscellaneous-row-module-flags    (:guard (revert-inst-standard-precondition))
                (eq!  (weighted-MISC-flag-sum  ROW_OFFSET_REVERT_MISCELLANEOUS_ROW)
                      (+  MISC_WEIGHT_MXP
                          (*  MISC_WEIGHT_MMU  (revert-inst-trigger_MMU)))))

(defconstraint  revert-inst-setting-the-MXP-data                          (:guard (revert-inst-standard-precondition))
                (set-MXP-instruction-type-4 ROW_OFFSET_REVERT_MISCELLANEOUS_ROW   ;; row offset kappa
                                            (revert-inst-instruction)             ;; instruction
                                            0                                     ;; bit modifying the behaviour of RETURN pricing
                                            (revert-inst-offset-hi)               ;; offset high
                                            (revert-inst-offset-lo)               ;; offset low
                                            (revert-inst-size-hi)                 ;; size high
                                            (revert-inst-size-lo)))               ;; size low

(defconstraint  revert-inst-setting-the-MXPX                              (:guard (revert-inst-standard-precondition))
                (eq!  stack/MXPX  (shift  misc/MXP_MXPX  ROW_OFFSET_REVERT_MISCELLANEOUS_ROW)))

(defconstraint  revert-inst-setting-the-MMU-data                          (:guard (revert-inst-standard-precondition))
                (if-not-zero  (shift  misc/MMU_FLAG  ROW_OFFSET_REVERT_MISCELLANEOUS_ROW)
                              (set-MMU-instruction-ram-to-ram-sans-padding    ROW_OFFSET_REVERT_MISCELLANEOUS_ROW  ;; row offset
                                                                              (revert-inst-current-context)        ;; source ID
                                                                              (revert-inst-caller-context)         ;; target ID
                                                                              ;; aux_id                               ;; auxiliary ID
                                                                              ;; src_offset_hi                        ;; source offset high
                                                                              (revert-inst-offset-lo)              ;; source offset low
                                                                              ;; tgt_offset_lo                        ;; target offset low
                                                                              (revert-inst-size-lo)                ;; size
                                                                              (revert-inst-r@o)                    ;; reference offset
                                                                              (revert-inst-r@c)                    ;; reference size
                                                                              ;; success_bit                          ;; success bit
                                                                              ;; limb_1                               ;; limb 1
                                                                              ;; limb_2                               ;; limb 2
                                                                              ;; exo_sum                              ;; weighted exogenous module flag sum
                                                                              ;; phase                                ;; phase
                                                                              )))

(defconstraint  revert-inst-setting-the-gas-cost                          (:guard (revert-inst-standard-precondition))
                (if-not-zero  stack/MXPX
                              (vanishes!  GAS_COST)
                              (eq!  GAS_COST
                                    (+  stack/STATIC_GAS
                                        (revert-inst-MXP-memory-expansion-gas)))))
