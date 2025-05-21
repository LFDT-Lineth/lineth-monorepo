(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                                  ;;;;
;;;;    X.Y COPY instruction family   ;;;;
;;;;                                  ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    X.Y.1 Introduction     ;;
;;    X.Y.2 Shorthands       ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconst
  ROFF_COPY_INST_MISCELLANEOUS_ROW                     1
  ;;
  ROFF_CALLDATACOPY_CONTEXT_ROW                        2
  ;;
  ROFF_RETURNDATACOPY_CURRENT_CONTEXT_ROW              2
  ROFF_RETURNDATACOPY_CALLER_CONTEXT_ROW               3
  ;;
  ROFF_CODECOPY_XAHOY_CONTEXT_ROW                      2
  ROFF_CODECOPY_NO_XAHOY_CONTEXT_ROW                   2
  ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW                   3
  ;;
  ROFF_EXTCODECOPY_MXPX_CONTEXT_ROW                    2
  ROFF_EXTCODECOPY_OOGX_ACCOUNT_ROW                    2
  ROFF_EXTCODECOPY_OOGX_CONTEXT_ROW                    3
  ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW   2
  ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW 3
  ROFF_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW      2
  ROFF_EXTCODECOPY_NO_XAHOY_ACCOUNT_ROW                2)

(defun (copy-instruction---instruction)           stack/INSTRUCTION)

;;
(defun (copy-instruction---is-CALLDATACOPY)      [stack/DEC_FLAG   1])
(defun (copy-instruction---is-RETURNDATACOPY)    [stack/DEC_FLAG   2])
(defun (copy-instruction---is-CODECOPY)          [stack/DEC_FLAG   3])
(defun (copy-instruction---is-EXTCODECOPY)       [stack/DEC_FLAG   4])

;;
(defun (copy-instruction---target-offset-hi)     [stack/STACK_ITEM_VALUE_HI   1])
(defun (copy-instruction---target-offset-lo)     [stack/STACK_ITEM_VALUE_LO   1])
(defun (copy-instruction---size-hi)              [stack/STACK_ITEM_VALUE_HI   2])
(defun (copy-instruction---size-lo)              [stack/STACK_ITEM_VALUE_LO   2])
(defun (copy-instruction---source-offset-hi)     [stack/STACK_ITEM_VALUE_HI   3])
(defun (copy-instruction---source-offset-lo)     [stack/STACK_ITEM_VALUE_LO   3])
(defun (copy-instruction---raw-address-hi)       [stack/STACK_ITEM_VALUE_HI   4])
(defun (copy-instruction---raw-address-lo)       [stack/STACK_ITEM_VALUE_LO   4])

;;
(defun (copy-instruction---OOB-raises-return-data-exception)         (shift [misc/OOB_DATA 7]    ROFF_COPY_INST_MISCELLANEOUS_ROW)) ;; ""
(defun (copy-instruction---MXP-raises-memory-expansion-exception)    (shift misc/MXP_MXPX        ROFF_COPY_INST_MISCELLANEOUS_ROW))
(defun (copy-instruction---MXP-memory-expansion-gas)                 (shift misc/MXP_GAS_MXP     ROFF_COPY_INST_MISCELLANEOUS_ROW))

;; call data related
(defun (copy-instruction---call-data-context)    (shift context/CALL_DATA_CONTEXT_NUMBER    ROFF_CALLDATACOPY_CONTEXT_ROW))
(defun (copy-instruction---call-data-offset)     (shift context/CALL_DATA_OFFSET            ROFF_CALLDATACOPY_CONTEXT_ROW))
(defun (copy-instruction---call-data-size)       (shift context/CALL_DATA_SIZE              ROFF_CALLDATACOPY_CONTEXT_ROW))

;; return data related
(defun (copy-instruction---return-data-context)    (shift context/RETURN_DATA_CONTEXT_NUMBER    ROFF_RETURNDATACOPY_CURRENT_CONTEXT_ROW))
(defun (copy-instruction---return-data-offset)     (shift context/RETURN_DATA_OFFSET            ROFF_RETURNDATACOPY_CURRENT_CONTEXT_ROW))
(defun (copy-instruction---return-data-size)       (shift context/RETURN_DATA_SIZE              ROFF_RETURNDATACOPY_CURRENT_CONTEXT_ROW))

;; for ext code copy
(defun (copy-instruction---bytecode-address-code-fragment-index)    (shift account/CODE_FRAGMENT_INDEX    ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW))
(defun (copy-instruction---bytecode-address-code-size)              (shift account/CODE_SIZE              ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW))

;;
(defun (copy-instruction---foreign-address-warmth)                 (shift account/WARMTH                 ROFF_EXTCODECOPY_NO_XAHOY_ACCOUNT_ROW))
(defun (copy-instruction---foreign-address-has-code)               (shift account/HAS_CODE               ROFF_EXTCODECOPY_NO_XAHOY_ACCOUNT_ROW))
(defun (copy-instruction---foreign-address-code-size)              (shift account/CODE_SIZE              ROFF_EXTCODECOPY_NO_XAHOY_ACCOUNT_ROW))
(defun (copy-instruction---foreign-address-code-fragment-index)    (shift account/CODE_FRAGMENT_INDEX    ROFF_EXTCODECOPY_NO_XAHOY_ACCOUNT_ROW))
(defun (copy-instruction---standard-precondition)                  (* PEEK_AT_STACK stack/COPY_FLAG (- 1 stack/SUX stack/SOX)))
(defun (copy-instruction---standard-CALLDATACOPY)                  (* (copy-instruction---standard-precondition) (copy-instruction---is-CALLDATACOPY)))
(defun (copy-instruction---standard-RETURNDATACOPY)                (* (copy-instruction---standard-precondition) (copy-instruction---is-RETURNDATACOPY)))
(defun (copy-instruction---standard-CODECOPY)                      (* (copy-instruction---standard-precondition) (copy-instruction---is-CODECOPY)))
(defun (copy-instruction---standard-EXTCODECOPY)                   (* (copy-instruction---standard-precondition) (copy-instruction---is-EXTCODECOPY)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    X.Y.3 General constraints   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    copy-instruction---setting-the-stack-pattern (:guard (copy-instruction---standard-precondition))
                  (copy-stack-pattern (copy-instruction---is-EXTCODECOPY)))

(defconstraint    copy-instruction---allowable-exceptions (:guard (copy-instruction---standard-precondition));; could be debug ...
                  (eq! XAHOY
                       (+ (* (copy-instruction---is-RETURNDATACOPY) stack/RDCX) stack/MXPX stack/OOGX)))

(defconstraint    copy-instruction---setting-NSR-and-peeking-flags---CALLDATACOPY-case (:guard (copy-instruction---standard-CALLDATACOPY))
                  (begin (eq! NSR 2)
                         (eq! NSR
                              (+ (shift PEEK_AT_MISCELLANEOUS    ROFF_COPY_INST_MISCELLANEOUS_ROW)
                                 (shift PEEK_AT_CONTEXT          ROFF_CALLDATACOPY_CONTEXT_ROW)))))

(defconstraint    copy-instruction---setting-NSR-and-peeking-flags---RETURNDATACOPY-case (:guard (copy-instruction---standard-RETURNDATACOPY))
                  (begin
                    (eq!  NSR  (+ 2 XAHOY))
                    (eq!  NSR
                          (+  (shift       PEEK_AT_MISCELLANEOUS   ROFF_COPY_INST_MISCELLANEOUS_ROW         )
                              (shift       PEEK_AT_CONTEXT         ROFF_RETURNDATACOPY_CURRENT_CONTEXT_ROW  )
                              (*   (shift  PEEK_AT_CONTEXT         ROFF_RETURNDATACOPY_CALLER_CONTEXT_ROW   )    XAHOY)))))

(defconstraint    copy-instruction---setting-NSR-and-peeking-flags---CODECOPY-case (:guard (copy-instruction---standard-CODECOPY))
                  (begin (eq! NSR (-   3   XAHOY))
                         (if-not-zero XAHOY
                                      ;; XAHOY ≡ 1
                                      (eq! NSR
                                           (+ (shift PEEK_AT_MISCELLANEOUS    ROFF_COPY_INST_MISCELLANEOUS_ROW)
                                              (shift PEEK_AT_CONTEXT          ROFF_CODECOPY_XAHOY_CONTEXT_ROW)))
                                      ;; XAHOY ≡ 0
                                      (eq! NSR
                                           (+ (shift PEEK_AT_MISCELLANEOUS    ROFF_COPY_INST_MISCELLANEOUS_ROW)
                                              (shift PEEK_AT_CONTEXT          ROFF_CODECOPY_NO_XAHOY_CONTEXT_ROW)
                                              (shift PEEK_AT_ACCOUNT          ROFF_CODECOPY_NO_XAHOY_ACCOUNT_ROW))))))

(defconstraint    copy-instruction---setting-NSR-and-peeking-flags---EXTCODECOPY-case (:guard (copy-instruction---standard-EXTCODECOPY))
                  (begin (eq! NSR
                              (+    2
                                    stack/OOGX
                                    (* (- 1 XAHOY) CONTEXT_WILL_REVERT)))
                         (if-not-zero stack/MXPX
                                      (eq! NSR
                                           (+ (shift PEEK_AT_MISCELLANEOUS    ROFF_COPY_INST_MISCELLANEOUS_ROW)
                                              (shift PEEK_AT_CONTEXT          ROFF_EXTCODECOPY_MXPX_CONTEXT_ROW))))
                         (if-not-zero stack/OOGX
                                      (eq! NSR
                                           (+ (shift PEEK_AT_MISCELLANEOUS    ROFF_COPY_INST_MISCELLANEOUS_ROW)
                                              (shift PEEK_AT_CONTEXT          ROFF_EXTCODECOPY_OOGX_CONTEXT_ROW)
                                              (shift PEEK_AT_ACCOUNT          ROFF_EXTCODECOPY_OOGX_ACCOUNT_ROW))))
                         (if-zero     XAHOY
                                      (if-not-zero CONTEXT_WILL_REVERT
                                                   ;; CN_WILL_REV ≡ 1
                                                   (eq! NSR
                                                        (+ (shift PEEK_AT_MISCELLANEOUS   ROFF_COPY_INST_MISCELLANEOUS_ROW)
                                                           (shift PEEK_AT_ACCOUNT         ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                                           (shift PEEK_AT_ACCOUNT         ROFF_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW)))
                                                   ;; CN_WILL_REV ≡ 0
                                                   (eq! NSR
                                                        (+ (shift PEEK_AT_MISCELLANEOUS    ROFF_COPY_INST_MISCELLANEOUS_ROW)
                                                           (shift PEEK_AT_ACCOUNT          ROFF_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)))))))

(defconstraint    copy-instruction---setting-misc-row-module-flags (:guard (copy-instruction---standard-precondition))
                  (eq! (weighted-MISC-flag-sum ROFF_COPY_INST_MISCELLANEOUS_ROW)
                       (+ (* MISC_WEIGHT_MMU (copy-instruction---trigger_MMU))
                          (* MISC_WEIGHT_MXP (copy-instruction---trigger_MXP))
                          (* MISC_WEIGHT_OOB (copy-instruction---trigger_OOB)))))

(defun (copy-instruction---trigger_OOB)    (copy-instruction---is-RETURNDATACOPY))
(defun (copy-instruction---trigger_MXP)    (- 1 stack/RDCX))
(defun (copy-instruction---trigger_MMU)    (* (- 1 XAHOY) (shift misc/MXP_MTNTOP ROFF_COPY_INST_MISCELLANEOUS_ROW)))

(defconstraint    copy-instruction---misc-row---setting-OOB-instruction (:guard (copy-instruction---standard-precondition))
                  (if-not-zero (shift misc/OOB_FLAG ROFF_COPY_INST_MISCELLANEOUS_ROW)
                               (set-OOB-instruction---rdc ROFF_COPY_INST_MISCELLANEOUS_ROW
                                                          (copy-instruction---source-offset-hi)
                                                          (copy-instruction---source-offset-lo)
                                                          (copy-instruction---size-hi)
                                                          (copy-instruction---size-lo)
                                                          (copy-instruction---return-data-size))))

(defconstraint    copy-instruction---misc-row---setting-RDCX (:guard (copy-instruction---standard-precondition))
                  (if-zero (shift misc/OOB_FLAG ROFF_COPY_INST_MISCELLANEOUS_ROW)
                           ;; OOB_FLAG ≡ 0
                           ;; zero case is redundant ...
                           (vanishes! stack/RDCX)
                           ;; OOB_FLAG ≡ 1
                           (eq! stack/RDCX (copy-instruction---OOB-raises-return-data-exception))))

(defconstraint    copy-instruction---misc-row---setting-MXP-instruction (:guard (copy-instruction---standard-precondition))
                  (if-not-zero (shift misc/MXP_FLAG ROFF_COPY_INST_MISCELLANEOUS_ROW)
                               (set-MXP-instruction-type-4 ROFF_COPY_INST_MISCELLANEOUS_ROW ;; row offset kappa
                                                           stack/INSTRUCTION                      ;; instruction
                                                           0                                      ;; deploys (bit modifying the behaviour of RETURN pricing)
                                                           (copy-instruction---target-offset-hi)  ;; offset high
                                                           (copy-instruction---target-offset-lo)  ;; offset low
                                                           (copy-instruction---size-hi)           ;; size high
                                                           (copy-instruction---size-lo))))        ;; size low

(defconstraint    copy-instruction---misc-row---setting-MXPX (:guard (copy-instruction---standard-precondition))
                  (if-zero    (shift    misc/MXP_FLAG    ROFF_COPY_INST_MISCELLANEOUS_ROW)
                              ;; MXP_FLAG ≡ 0
                              ;; can only happen for RETURNDATACOPY instruction raising the returnDataCopyException; redundant constraint;
                              (eq!      stack/MXPX 0)
                              ;; MXP_FLAG ≡ 1
                              (eq!      stack/MXPX (copy-instruction---MXP-raises-memory-expansion-exception))))

(defconstraint       copy-instruction---misc-row---partially-setting-the-MMU-instruction                        (:guard    (copy-instruction---standard-precondition))
                     (if-not-zero  (shift  misc/MMU_FLAG  ROFF_COPY_INST_MISCELLANEOUS_ROW)
                                   (set-MMU-instruction---any-to-ram-with-padding    ROFF_COPY_INST_MISCELLANEOUS_ROW      ;; offset
                                                                                     (copy-instruction---source-id)              ;; source ID
                                                                                     CONTEXT_NUMBER                              ;; target ID
                                                                                     ;; aux_id                                      ;; auxiliary ID
                                                                                     (copy-instruction---source-offset-hi)       ;; source offset high
                                                                                     (copy-instruction---source-offset-lo)       ;; source offset low
                                                                                     (copy-instruction---target-offset-lo)       ;; target offset low
                                                                                     (copy-instruction---size-lo)                ;; size
                                                                                     (copy-instruction---reference-offset)       ;; reference offset
                                                                                     (copy-instruction---reference-size)         ;; reference size
                                                                                     ;; success_bit                                 ;; success bit
                                                                                     ;; limb_1                                      ;; limb 1
                                                                                     ;; limb_2                                      ;; limb 2
                                                                                     (copy-instruction---exo-sum)                ;; weighted exogenous module flag sum
                                                                                     ;; phase                                       ;; phase
                                                                                     )))

(defun (copy-instruction---source-id)         (+   (* (copy-instruction---is-CALLDATACOPY)      (copy-instruction---call-data-context))
                                                   (* (copy-instruction---is-RETURNDATACOPY)    (copy-instruction---return-data-context))
                                                   (* (copy-instruction---is-CODECOPY)          (copy-instruction---bytecode-address-code-fragment-index))
                                                   (* (copy-instruction---is-EXTCODECOPY)       (copy-instruction---foreign-address-code-fragment-index)  (copy-instruction---foreign-address-has-code))))

(defun (copy-instruction---reference-offset)  (+   (* (copy-instruction---is-CALLDATACOPY)      (copy-instruction---call-data-offset))
                                                   (* (copy-instruction---is-RETURNDATACOPY)    (copy-instruction---return-data-offset))))

(defun (copy-instruction---reference-size)    (+   (* (copy-instruction---is-CALLDATACOPY)      (copy-instruction---call-data-size))
                                                   (* (copy-instruction---is-RETURNDATACOPY)    (copy-instruction---return-data-size))
                                                   (* (copy-instruction---is-CODECOPY)          (copy-instruction---bytecode-address-code-size))
                                                   (* (copy-instruction---is-EXTCODECOPY)       (copy-instruction---foreign-address-code-size) (copy-instruction---foreign-address-has-code))))

(defun (copy-instruction---exo-sum)           (+   (* (copy-instruction---is-CODECOPY)          EXO_SUM_WEIGHT_ROM)
                                                   (* (copy-instruction---is-EXTCODECOPY)       EXO_SUM_WEIGHT_ROM)))
