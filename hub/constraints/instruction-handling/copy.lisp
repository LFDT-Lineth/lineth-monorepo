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
  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW                     1
  ;;
  ROW_OFFSET_CALLDATACOPY_CONTEXT_ROW                        2
  ;;
  ROW_OFFSET_RETURNDATACOPY_CURRENT_CONTEXT_ROW              2
  ROW_OFFSET_RETURNDATACOPY_CALLER_CONTEXT_ROW               2
  ROW_OFFSET_RETURNDATACOPY_CALLER_CONTEXT_ROW_RDCX          3
  ;;
  ROW_OFFSET_CODECOPY_XAHOY_CONTEXT_ROW                      2
  ROW_OFFSET_CODECOPY_NO_XAHOY_CONTEXT_ROW                   2
  ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW                   3
  ;;
  ROW_OFFSET_EXTCODECOPY_MXPX_CONTEXT_ROW                    2
  ROW_OFFSET_EXTCODECOPY_OOGX_CONTEXT_ROW                    2
  ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW                    3
  ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW   2
  ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW 3
  ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW      2
  ROW_OFFSET_EXTCODECOPY_NO_XAHOY_ACCOUNT_ROW                2
  )

(defun (copy-inst-instruction)                                  stack/INSTRUCTION    )
;;
(defun (copy-inst-is-CALLDATACOPY)                              [ stack/DEC_FLAG 1 ] )
(defun (copy-inst-is-RETURNDATACOPY)                            [ stack/DEC_FLAG 2 ] )
(defun (copy-inst-is-CODECOPY)                                  [ stack/DEC_FLAG 3 ] )
(defun (copy-inst-is-EXTCODECOPY)                               [ stack/DEC_FLAG 4 ] )
;;
(defun (copy-inst-target-offset-hi)                             [ stack/STACK_ITEM_VALUE_HI 1 ] )
(defun (copy-inst-target-offset-lo)                             [ stack/STACK_ITEM_VALUE_LO 1 ] )
(defun (copy-inst-source-offset-hi)                             [ stack/STACK_ITEM_VALUE_HI 2 ] )
(defun (copy-inst-source-offset-lo)                             [ stack/STACK_ITEM_VALUE_LO 2 ] )
(defun (copy-inst-size-hi)                                      [ stack/STACK_ITEM_VALUE_HI 3 ] )
(defun (copy-inst-size-lo)                                      [ stack/STACK_ITEM_VALUE_LO 3 ] )
(defun (copy-inst-raw-address-hi)                               [ stack/STACK_ITEM_VALUE_HI 4 ] )
(defun (copy-inst-raw-address-lo)                               [ stack/STACK_ITEM_VALUE_LO 4 ] )
;;
(defun (copy-inst-OOB-raises-return-data-exception)             ( shift    [ misc/OOB_DATA 7 ]    ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW ))
(defun (copy-inst-MXP-raises-memory-expansion-exception)        ( shift    misc/MXP_MXPX          ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW ))
(defun (copy-inst-MXP-memory-expansion-gas)                     ( shift    misc/MXP_GAS_MXP       ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW ))
;;
(defun (copy-inst-call-data-context)                            ( shift    context/CALL_DATA_CONTEXT_NUMBER      ROW_OFFSET_CALLDATACOPY_CONTEXT_ROW ))
(defun (copy-inst-call-data-offset)                             ( shift    context/CALL_DATA_OFFSET              ROW_OFFSET_CALLDATACOPY_CONTEXT_ROW ))
(defun (copy-inst-call-data-size)                               ( shift    context/CALL_DATA_SIZE                ROW_OFFSET_CALLDATACOPY_CONTEXT_ROW ))
;;
(defun (copy-inst-return-data-context)                          ( shift    context/RETURN_DATA_CONTEXT_NUMBER    ROW_OFFSET_RETURNDATACOPY_CURRENT_CONTEXT_ROW ))
(defun (copy-inst-return-data-offset)                           ( shift    context/RETURN_DATA_OFFSET            ROW_OFFSET_RETURNDATACOPY_CURRENT_CONTEXT_ROW ))
(defun (copy-inst-return-data-size)                             ( shift    context/RETURN_DATA_SIZE              ROW_OFFSET_RETURNDATACOPY_CURRENT_CONTEXT_ROW ))
;;
(defun (copy-inst-current-address-code-fragment-index)          ( shift    account/CODE_FRAGMENT_INDEX           ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW ))
(defun (copy-inst-current-address-code-size)                    ( shift    account/CODE_SIZE                     ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW ))
;;
(defun (copy-inst-exo-address-warmth)                               ( shift    account/WARMTH                 ROW_OFFSET_EXTCODECOPY_NO_XAHOY_ACCOUNT_ROW ))
(defun (copy-inst-exo-address-has-code)                             ( shift    account/HAS_CODE               ROW_OFFSET_EXTCODECOPY_NO_XAHOY_ACCOUNT_ROW ))
(defun (copy-inst-exo-address-code-size)                            ( shift    account/CODE_SIZE              ROW_OFFSET_EXTCODECOPY_NO_XAHOY_ACCOUNT_ROW ))
(defun (copy-inst-exo-address-code-fragment-index)                  ( shift    account/CODE_FRAGMENT_INDEX    ROW_OFFSET_EXTCODECOPY_NO_XAHOY_ACCOUNT_ROW ))


(defun (copy-inst-standard-precondition)  (*  PEEK_AT_STACK
                                              stack/COPY_FLAG
                                              (- 1   stack/SUX   stack/SOX)))

(defun (copy-inst-standard-CALLDATACOPY)     (*   (copy-inst-standard-precondition)    (copy-inst-is-CALLDATACOPY)))
(defun (copy-inst-standard-RETURNDATACOPY)   (*   (copy-inst-standard-precondition)    (copy-inst-is-RETURNDATACOPY)))
(defun (copy-inst-standard-CODECOPY)         (*   (copy-inst-standard-precondition)    (copy-inst-is-CODECOPY)))
(defun (copy-inst-standard-EXTCODECOPY)      (*   (copy-inst-standard-precondition)    (copy-inst-is-EXTCODECOPY)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    X.Y.3 General constraints   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    copy-setting-the-stack-pattern                                             (:guard    (copy-inst-standard-precondition))
                  (copy-stack-pattern    (copy-inst-is-EXTCODECOPY)))

(defconstraint    copy-allowable-exceptions                                                  (:guard    (copy-inst-standard-precondition))   ;; TODO: make debug
                  (eq!    XAHOY    (+  (*  (copy-inst-is-RETURNDATACOPY) stack/RDCX)
                                       stack/MXPX
                                       stack/OOGX)))

(defconstraint    copy-setting-NSR-and-peeking-flags-CALLDATACOPY-case                       (:guard    (copy-inst-standard-CALLDATACOPY))
                  (begin
                    (eq!  NSR  2)
                    (eq!  NSR
                          (+  (shift  PEEK_AT_MISCELLANEOUS  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                              (shift  PEEK_AT_CONTEXT        ROW_OFFSET_CALLDATACOPY_CONTEXT_ROW)))))

(defconstraint    copy-setting-NSR-and-peeking-flags-RETURNDATACOPY-case                     (:guard    (copy-inst-standard-RETURNDATACOPY))
                  (begin
                    (eq!  NSR  (+  2  stack/RDCX))
                    (if-not-zero   stack/RDCX
                                   ;; RDCX ≡ 1
                                   (eq!  NSR
                                         (+  (shift  PEEK_AT_MISCELLANEOUS  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                                             (shift  PEEK_AT_CONTEXT        ROW_OFFSET_RETURNDATACOPY_CURRENT_CONTEXT_ROW)
                                             (shift  PEEK_AT_CONTEXT        ROW_OFFSET_RETURNDATACOPY_CALLER_CONTEXT_ROW_RDCX)))
                                   ;; RDCX ≡ 0
                                   (eq!  NSR
                                         (+  (shift  PEEK_AT_MISCELLANEOUS  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                                             (shift  PEEK_AT_CONTEXT        ROW_OFFSET_RETURNDATACOPY_CURRENT_CONTEXT_ROW))))))

(defconstraint    copy-setting-NSR-and-peeking-flags-CODECOPY-case                           (:guard    (copy-inst-standard-CODECOPY))
                  (begin
                    (eq!  NSR  (-  3  stack/RDCX))
                    (if-not-zero   XAHOY
                                   ;; XAHOY ≡ 1
                                   (eq!  NSR
                                         (+  (shift  PEEK_AT_MISCELLANEOUS  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                                             (shift  PEEK_AT_CONTEXT        ROW_OFFSET_CODECOPY_XAHOY_CONTEXT_ROW)))
                                   ;; XAHOY ≡ 0
                                   (eq!  NSR
                                         (+  (shift  PEEK_AT_MISCELLANEOUS  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                                             (shift  PEEK_AT_CONTEXT        ROW_OFFSET_CODECOPY_NO_XAHOY_CONTEXT_ROW)
                                             (shift  PEEK_AT_ACCOUNT        ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW))))))

(defconstraint    copy-setting-NSR-and-peeking-flags-EXTCODECOPY-case                        (:guard    (copy-inst-standard-EXTCODECOPY))
                  (begin
                    (eq!  NSR  (+  2
                                   stack/OOGX
                                   (*  (-  1  XAHOY) CONTEXT_WILL_REVERT)))
                    (if-not-zero  stack/MXPX
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_MISCELLANEOUS  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                                            (shift  PEEK_AT_CONTEXT        ROW_OFFSET_EXTCODECOPY_MXPX_CONTEXT_ROW))))
                    (if-not-zero  stack/OOGX
                                  (eq!  NSR
                                        (+  (shift  PEEK_AT_MISCELLANEOUS  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                                            (shift  PEEK_AT_CONTEXT        ROW_OFFSET_EXTCODECOPY_OOGX_CONTEXT_ROW)
                                            (shift  PEEK_AT_ACCOUNT        ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW))))
                    (if-zero  XAHOY
                              (if-not-zero  CONTEXT_WILL_REVERT
                                            ;; CN_WILL_REV ≡ 1
                                            (eq!  NSR
                                                  (+  (shift  PEEK_AT_MISCELLANEOUS  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                                                      (shift  PEEK_AT_ACCOUNT        ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                                      (shift  PEEK_AT_ACCOUNT        ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW)))
                                            ;; CN_WILL_REV ≡ 0
                                            (eq!  NSR
                                                  (+  (shift  PEEK_AT_MISCELLANEOUS  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                                                      (shift  PEEK_AT_ACCOUNT        ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)))))))


(defconstraint    copy-setting-misc-row-module-flags                                         (:guard    (copy-inst-standard-precondition))
                  (eq!  (weighted-MISC-flag-sum   ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                        (+  (*  MISC_WEIGHT_MMU   (copy-inst-trigger_MMU))
                            (*  MISC_WEIGHT_MXP   (copy-inst-trigger_MXP))
                            (*  MISC_WEIGHT_OOB   (copy-inst-trigger_OOB)))))

(defun  (copy-inst-trigger_OOB)  (copy-inst-is-RETURNDATACOPY))
(defun  (copy-inst-trigger_MXP)  (-  1  stack/RDCX))
(defun  (copy-inst-trigger_MMU)  (*  (-  1  XAHOY) (shift  misc/MXP_MTNTOP  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)))

(defconstraint    copy-misc-row-setting-OOB-instruction                                      (:guard    (copy-inst-standard-precondition))
                  (if-not-zero  (shift  misc/OOB_FLAG  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                                (set-OOB-instruction-rdc  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW
                                                          (copy-inst-source-offset-hi)
                                                          (copy-inst-source-offset-lo)
                                                          (copy-inst-size-hi)
                                                          (copy-inst-size-lo)
                                                          (copy-inst-return-data-size))))

(defconstraint    copy-misc-row-setting-RDCX                                                 (:guard    (copy-inst-standard-precondition))
                  (if-zero  (shift  misc/OOB_FLAG  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                            ;; OOB_FLAG ≡ 0
                            ;; TODO: zero case is redundant; careful when modifying the constraint (if-zero vs. if-not-zero)
                            (vanishes!  stack/RDCX)
                            ;; OOB_FLAG ≡ 1
                            (eq!  stack/RDCX (copy-inst-OOB-raises-return-data-exception))))

(defconstraint    copy-misc-row-setting-MXP-instruction                                      (:guard    (copy-inst-standard-precondition))
                  (if-not-zero  (shift  misc/MXP_FLAG  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                                (set-MXP-instruction-type-4   ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW    ;; row offset kappa
                                                              stack/INSTRUCTION                         ;; instruction
                                                              0                                         ;; deploys (bit modifying the behaviour of RETURN pricing)
                                                              (copy-inst-target-offset-hi)              ;; offset high
                                                              (copy-inst-target-offset-lo)              ;; offset low
                                                              (copy-inst-size-hi)                       ;; size high
                                                              (copy-inst-size-lo))))                    ;; size low

(defconstraint    copy-misc-row-setting-MXPX                                                 (:guard    (copy-inst-standard-precondition))
                  (if-zero  (shift  misc/MXP_FLAG  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                            ;; MXP_FLAG ≡ 0
                            ;; can only happen for RETURNDATACOPY instruction raising the returnDataCopyException; redundant constraint;
                            (eq!  stack/RDCX  0)
                            ;; MXP_FLAG ≡ 1
                            (eq!  stack/RDCX  (copy-inst-MXP-raises-memory-expansion-exception))))


(defconstraint    copy-misc-row-partially-setting-the-MMU-instruction                        (:guard    (copy-inst-standard-precondition))
                  (if-not-zero  (shift  misc/MMU_FLAG  ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW)
                                (set-MMU-instruction-any-to-ram-with-padding    ROW_OFFSET_COPY_INST_MISCELLANEOUS_ROW      ;; offset
                                                                                (copy-inst-source-id)                       ;; source ID
                                                                                CONTEXT_NUMBER                              ;; target ID
                                                                                ;; aux_id                                      ;; auxiliary ID
                                                                                (copy-inst-source-offset-hi)                ;; source offset high
                                                                                (copy-inst-source-offset-lo)                ;; source offset low
                                                                                (copy-inst-target-offset-lo)                ;; target offset low
                                                                                (copy-inst-size-lo)                         ;; size
                                                                                (copy-inst-reference-offset)                ;; reference offset
                                                                                (copy-inst-reference-size)                  ;; reference size
                                                                                ;; success_bit                                 ;; success bit
                                                                                ;; limb_1                                      ;; limb 1
                                                                                ;; limb_2                                      ;; limb 2
                                                                                (copy-inst-exo-sum)                         ;; weighted exogenous module flag sum
                                                                                ;; phase                                       ;; phase
                                                                                )))

(defun  (copy-inst-source-id)  (+  (*  (copy-inst-is-CALLDATACOPY)     (copy-inst-call-data-context))
                                   (*  (copy-inst-is-RETURNDATACOPY)   (copy-inst-return-data-context))
                                   (*  (copy-inst-is-CODECOPY)         (copy-inst-current-address-code-fragment-index))
                                   (*  (copy-inst-is-EXTCODECOPY)      (copy-inst-exo-address-code-fragment-index))))

(defun  (copy-inst-reference-offset)  (+  (*  (copy-inst-is-CALLDATACOPY)     (copy-inst-call-data-offset))
                                          (*  (copy-inst-is-RETURNDATACOPY)   (copy-inst-return-data-offset))))

(defun  (copy-inst-reference-size)  (+  (*  (copy-inst-is-CALLDATACOPY)     (copy-inst-call-data-size))
                                        (*  (copy-inst-is-RETURNDATACOPY)   (copy-inst-return-data-size))
                                        (*  (copy-inst-is-CODECOPY)         (copy-inst-current-address-code-size))
                                        (*  (copy-inst-is-EXTCODECOPY)      (*  (copy-inst-exo-address-code-fragment-index)
                                                                                (copy-inst-exo-address-has-code)))))

(defun  (copy-inst-exo-sum)  (*  (+  (copy-inst-is-CODECOPY) (copy-inst-is-EXTCODECOPY))
                                 EXO_SUM_WEIGHT_ROM))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;    X.Y.4 Specifics for CALLDATACOPY   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   copy-setting-the-gas-cost-for-CALLDATACOPY                      (:guard   (copy-inst-standard-CALLDATACOPY))
                 (begin
                   (if-not-zero  stack/MXPX  (vanishes!  GAS_COST))
                   (if-not-zero  stack/OOGX  (eq!  GAS_COST (+  stack/STATIC_GAS  (copy-inst-MXP-memory-expansion-gas))))
                   (if-zero      XAHOY       (eq!  GAS_COST (+  stack/STATIC_GAS  (copy-inst-MXP-memory-expansion-gas))))))

(defconstraint   copy-setting-context-row-for-exceptional-CALLDATACOPY           (:guard   (copy-inst-standard-CALLDATACOPY))
                 (if-not-zero  XAHOY
                               (execution-provides-empty-return-data  ROW_OFFSET_CALLDATACOPY_CONTEXT_ROW)
                               (read-context-data                     ROW_OFFSET_CALLDATACOPY_CONTEXT_ROW
                                                                      CONTEXT_NUMBER)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;    X.Y.5 Specifics for RETURNDATACOPY   ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   copy-setting-the-gas-cost-for-RETURNDATACOPY                   (:guard   (copy-inst-standard-RETURNDATACOPY))
                 (begin
                   (if-not-zero  stack/RDCX  (vanishes!  GAS_COST))
                   (if-not-zero  stack/MXPX  (vanishes!  GAS_COST))
                   (if-not-zero  stack/OOGX  (eq!        GAS_COST (+  stack/STATIC_GAS  (copy-inst-MXP-memory-expansion-gas))))
                   (if-zero      XAHOY       (eq!        GAS_COST (+  stack/STATIC_GAS  (copy-inst-MXP-memory-expansion-gas))))))

(defconstraint   copy-setting-the-context-row-for-RETURNDATACOPY                (:guard   (copy-inst-standard-RETURNDATACOPY))
                 (begin
                   (if-not-zero  stack/RDCX  (begin  (read-context-data                     ROW_OFFSET_RETURNDATACOPY_CURRENT_CONTEXT_ROW  CONTEXT_NUMBER  )
                                                     (execution-provides-empty-return-data  ROW_OFFSET_RETURNDATACOPY_CALLER_CONTEXT_ROW_RDCX              )))
                   (if-not-zero  stack/MXPX  (execution-provides-empty-return-data          ROW_OFFSET_RETURNDATACOPY_CALLER_CONTEXT_ROW                   ))
                   (if-not-zero  stack/OOGX  (execution-provides-empty-return-data          ROW_OFFSET_RETURNDATACOPY_CALLER_CONTEXT_ROW                   ))
                   (if-zero      XAHOY       (read-context-data                             ROW_OFFSET_RETURNDATACOPY_CURRENT_CONTEXT_ROW  CONTEXT_NUMBER  ))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;    X.Y.6 Specifics for CODECOPY   ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   copy-setting-the-gas-cost-for-CODECOPY                         (:guard  (copy-inst-standard-CODECOPY))
                 (begin
                   (if-not-zero  stack/MXPX  (vanishes!  GAS_COST))
                   (if-not-zero  stack/OOGX  (eq!        GAS_COST (+  stack/STATIC_GAS  (copy-inst-MXP-memory-expansion-gas))))
                   (if-zero      XAHOY       (eq!        GAS_COST (+  stack/STATIC_GAS  (copy-inst-MXP-memory-expansion-gas))))))

(defconstraint   copy-setting-the-context-row-for-exceptional-CODECOPY          (:guard  (copy-inst-standard-CODECOPY))
                 (if-not-zero  XAHOY  (execution-provides-empty-return-data  ROW_OFFSET_CODECOPY_XAHOY_CONTEXT_ROW)))

(defconstraint   copy-setting-the-context-row-for-unexceptional-CODECOPY        (:guard  (copy-inst-standard-CODECOPY))
                 (if-zero  XAHOY
                           (read-context-data  ROW_OFFSET_CODECOPY_XAHOY_CONTEXT_ROW  CONTEXT_NUMBER)))

(defconstraint   copy-setting-the-account-row-for-unexceptional-CODECOPY        (:guard  (copy-inst-standard-CODECOPY))
                 (if-zero  XAHOY
                           (begin
                             (eq!  (shift  account/ADDRESS_HI             ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW)  (shift  context/BYTE_CODE_ADDRESS_HI           ROW_OFFSET_CODECOPY_NO_XAHOY_CONTEXT_ROW))
                             (eq!  (shift  account/ADDRESS_LO             ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW)  (shift  context/BYTE_CODE_ADDRESS_LO           ROW_OFFSET_CODECOPY_NO_XAHOY_CONTEXT_ROW))
                             (eq!  (shift  account/DEPLOYMENT_NUMBER      ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW)  (shift  context/BYTE_CODE_CODE_FRAGMENT_INDEX  ROW_OFFSET_CODECOPY_NO_XAHOY_CONTEXT_ROW))
                             (eq!  (shift  account/CODE_FRAGMENT_INDEX    ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW)  CODE_FRAGMENT_INDEX)
                             (eq!  (shift  account/ROMLEX_FLAG            ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW)  1)
                             (account-same-balance                        ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW)
                             (account-same-nonce                          ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW)
                             (account-same-code                           ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW)
                             (account-same-deployment-number-and-status   ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW)
                             (account-same-warmth                         ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW)
                             (account-same-marked-for-selfdestruct        ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW)
                             (standard-dom-sub-stamps                     ROW_OFFSET_CODECOPY_NO_XAHOY_ACCOUNT_ROW
                                                                          0))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X.Y.7 Specifics for EXTCODECOPY   ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   copy-setting-the-gas-cost-for-EXTCODECOPY                                        (:guard   (copy-inst-standard-EXTCODECOPY))
                 (if-not-zero  stack/MXPX
                               ;; MXPX ≡ 1
                               (vanishes!  GAS_COST)
                               ;; MXPX ≡ 0
                               (eq!        GAS_COST
                                           (+  stack/STATIC_GAS
                                               (copy-inst-MXP-memory-expansion-gas)
                                               (*  (copy-inst-exo-address-warmth)          GAS_CONST_G_WARM_ACCESS)
                                               (*  (-  1  (copy-inst-exo-address-warmth))  GAS_CONST_G_COLD_ACCOUNT_ACCESS)))))

(defconstraint   copy-the-MXPX-case-for-EXTCODECOPY                                               (:guard   (copy-inst-standard-EXTCODECOPY))
                 (if-not-zero  stack/MXPX  (execution-provides-empty-return-data  ROW_OFFSET_EXTCODECOPY_MXPX_CONTEXT_ROW)))

(defconstraint   copy-the-OOGX-case-for-EXTCODECOPY                                               (:guard   (copy-inst-standard-EXTCODECOPY))
                 (if-not-zero  stack/OOGX
                               ;; account-row i + 2
                               (begin
                                 (account-trim-address  ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW
                                                        (copy-inst-raw-address-hi)
                                                        (copy-inst-raw-address-lo))
                                 (vanishes!             (shift  account/ROMLEX_FLAG    ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW))
                                 (account-same-balance                                 ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                                 (account-same-nonce                                   ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                                 (account-same-code                                    ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                                 (account-same-deployment-number-and-status            ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                                 (account-same-warmth                                  ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                                 (account-same-marked-for-selfdestruct                 ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW)
                                 (standard-dom-sub-stamps                              ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW
                                                                                       0))
                               ;; context-row i + 3
                               (execution-provides-empty-return-data  ROW_OFFSET_EXTCODECOPY_OOGX_CONTEXT_ROW)))

(defun  (copy-inst-trigger-CFI)  (*  (copy-inst-is-EXTCODECOPY)  (copy-inst-trigger_MMU)  (copy-inst-exo-address-has-code)))

(defconstraint   copy-inst-unexceptional-reverted-EXTCODECOPY-doing-account-row                   (:guard   (copy-inst-standard-EXTCODECOPY))
                 (if-not-zero  (*  (- 1 XAHOY)  CONTEXT_WILL_REVERT)
                               (begin
                                 (account-trim-address  ROW_OFFSET_EXTCODECOPY_OOGX_ACCOUNT_ROW
                                                        (copy-inst-raw-address-hi)
                                                        (copy-inst-raw-address-lo))
                                 (eq!          (shift  account/ROMLEX_FLAG   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)  (copy-inst-trigger-CFI))
                                 (account-same-balance                       ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                 (account-same-nonce                         ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                 (account-same-code                          ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                 (account-same-deployment-number-and-status  ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                 (account-turn-on-warmth                     ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                 (account-same-marked-for-selfdestruct       ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                 (standard-dom-sub-stamps                    ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW
                                                                             0))))

(defconstraint   copy-inst-unexceptional-reverted-EXTCODECOPY-undoing-account-row                 (:guard   (copy-inst-standard-EXTCODECOPY))
                 (if-not-zero  (*  (- 1 XAHOY)  CONTEXT_WILL_REVERT)
                               (begin
                                 (account-same-address-as ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW
                                                          ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW  )
                                 (eq!          (shift  account/ROMLEX_FLAG     ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW)  (copy-inst-trigger-CFI))
                                 (account-undo-balance-update                  ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                 (account-undo-nonce-update                    ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                 (account-undo-code-update                     ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                 (account-undo-deployment-status-update        ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                 (account-undo-warmth-update                   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_DOING_ROW)
                                 (account-same-marked-for-selfdestruct         ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW)
                                 (revert-dom-sub-stamps                        ROW_OFFSET_EXTCODECOPY_NO_XAHOY_REVERT_ACCOUNT_UNDOING_ROW
                                                                               1))))

(defconstraint   copy-inst-unexceptional-unreverted-EXTCODECOPY-account-row                       (:guard   (copy-inst-standard-EXTCODECOPY))
                 (if-not-zero  (*  (- 1 XAHOY)  (- 1 CONTEXT_WILL_REVERT))
                               (begin
                                 (account-trim-address  ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW
                                                        (copy-inst-raw-address-hi)
                                                        (copy-inst-raw-address-lo))
                                 (eq!          (shift  account/ROMLEX_FLAG   ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)  (copy-inst-trigger-CFI))
                                 (account-same-balance                       ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                 (account-same-nonce                         ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                 (account-same-code                          ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                 (account-same-deployment-number-and-status  ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                 (account-turn-on-warmth                     ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                 (account-same-marked-for-selfdestruct       ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW)
                                 (standard-dom-sub-stamps                    ROW_OFFSET_EXTCODECOPY_NO_XAHOY_NO_REVERT_ACCOUNT_ROW
                                                                             0))))
