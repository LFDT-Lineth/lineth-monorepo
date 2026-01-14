(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.Y CREATE   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;    X.Y.9 Generalities for all CREATE's   ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (create-instruction---generic-precondition)    (*    PEEK_AT_SCENARIO    (scenario-shorthand---CREATE---sum)))

;; monstrous ...
(defconstraint    create-instruction---setting-the-stack-pattern                            (:guard    (create-instruction---generic-precondition))
                  (shift (create-stack-pattern    (shift (create-instruction---is-CREATE2)  2))  -2))

(defconstraint    create-instruction---setting-the-deployment-address-stack-output          (:guard    (create-instruction---generic-precondition))
                  (begin    (eq!    (create-instruction---STACK-output-hi)    (*    (scenario-shorthand---CREATE---deployment-success)    (create-instruction---createe-address-hi)))
                            (eq!    (create-instruction---STACK-output-lo)    (*    (scenario-shorthand---CREATE---deployment-success)    (create-instruction---createe-address-lo)))
                            ))

(defconstraint    create-instruction---triggering-the-HASHINFO-lookup-and-settings          (:guard    (create-instruction---generic-precondition))
                  (maybe-request-hash   CREATE_first_stack_row___row_offset
                                        (create-instruction---trigger_HASHINFO)))

(defconstraint    create-instruction---setting-the-first-context-row                        (:guard    (create-instruction---generic-precondition))
                  (read-context-data    CREATE_current_context_row___row_offset
                                        CONTEXT_NUMBER))

(defconstraint    create-instruction---setting-the-static-exception                         (:guard    (create-instruction---generic-precondition))
                  (eq!    (create-instruction---STACK-staticx)
                          (create-instruction---current-context-is-static)))

(defconstraint    create-instruction---setting-the-module-flags-of-the-miscellaneous-row    (:guard    (create-instruction---generic-precondition))
                  (begin
                    (eq!    (shift    misc/EXP_FLAG    CREATE_miscellaneous_row___row_offset)    0)
                    (eq!    (shift    misc/MMU_FLAG    CREATE_miscellaneous_row___row_offset)    (create-instruction---trigger_MMU))
                    (eq!    (shift    misc/MXP_FLAG    CREATE_miscellaneous_row___row_offset)    (create-instruction---trigger_MXP))
                    (eq!    (shift    misc/OOB_FLAG    CREATE_miscellaneous_row___row_offset)    (create-instruction---trigger_OOB))
                    (eq!    (shift    misc/STP_FLAG    CREATE_miscellaneous_row___row_offset)    (create-instruction---trigger_STP))
                    ))

(defconstraint    create-instruction---setting-the-MXP-instruction                          (:guard    (create-instruction---generic-precondition))
                  (if-not-zero    (shift    misc/MXP_FLAG    CREATE_miscellaneous_row___row_offset)
                                  (set-MXP-instruction---single-mxp-offset-instructions   CREATE_miscellaneous_row___row_offset       ;; row offset kappa
                                                                                          (create-instruction---instruction)          ;; instruction
                                                                                          0                                           ;; bit modifying the behaviour of RETURN pricing
                                                                                          (create-instruction---STACK-offset-hi)      ;; offset high
                                                                                          (create-instruction---STACK-offset-lo)      ;; offset low
                                                                                          (create-instruction---STACK-size-hi)        ;; size high
                                                                                          (create-instruction---STACK-size-lo))       ;; size low
                                                                                          ))

(defconstraint    create-instruction---setting-the-memory-expansion-exception               (:guard    (create-instruction---generic-precondition))
                  (if-zero    (shift    misc/MXP_FLAG    CREATE_miscellaneous_row___row_offset)
                              ;; MXP_FLAG  ≡  0
                              (vanishes!    (create-instruction---STACK-mxpx))
                              ;; MXP_FLAG  ≡  1
                              (eq!          (create-instruction---STACK-mxpx)
                                            (create-instruction---MXP-mxpx))
                              ))

(defconstraint    create-instruction---setting-the-STP-instruction                          (:guard    (create-instruction---generic-precondition))
                  (if-not-zero    (shift    misc/STP_FLAG    CREATE_miscellaneous_row___row_offset)
                                  (set-STP-instruction-create    CREATE_miscellaneous_row___row_offset  ;; relative row offset
                                                                 (create-instruction---instruction)     ;; instruction
                                                                 (create-instruction---STACK-value-hi)  ;; value to transfer, high part
                                                                 (create-instruction---STACK-value-lo)  ;; value to transfer, low  part
                                                                 (create-instruction---MXP-gas)         ;; memory expansion gas
                                                                 )))

(defconstraint    create-instruction---setting-the-out-of-gas-exception
                  (:guard    (create-instruction---generic-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-zero    (shift    misc/STP_FLAG    CREATE_miscellaneous_row___row_offset)
                              ;; STP_FLAG  ≡  0
                              (vanishes!    (create-instruction---STACK-oogx))
                              ;; STP_FLAG  ≡  1
                              (eq!          (create-instruction---STACK-oogx)
                                            (create-instruction---STP-oogx))
                              ))

(defconstraint    create-instruction---setting-the-OOB-instruction---exceptional-case
                  (:guard    (create-instruction---generic-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (create-instruction---trigger_OOB_X)
                                  (set-OOB-instruction---xcreate    CREATE_miscellaneous_row___row_offset         ;; offset
                                                                    (create-instruction---STACK-size-hi)          ;; init code size    (high part)
                                                                    (create-instruction---STACK-size-lo)          ;; init code size    (low  part,  stack argument of CREATE-type instruction)
                                                                    )))

(defconstraint    create-instruction---setting-the-OOB-instruction---unexceptional-case
                  (:guard    (create-instruction---generic-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (create-instruction---trigger_OOB_U)
                                  (set-OOB-instruction---create    CREATE_miscellaneous_row___row_offset         ;; offset
                                                                   (create-instruction---STACK-value-hi)         ;; value    (high part)
                                                                   (create-instruction---STACK-value-lo)         ;; value    (low  part,  stack argument of CALL-type instruction)
                                                                   (create-instruction---creator-balance)        ;; balance  (from caller account)
                                                                   (create-instruction---createe-nonce)          ;; callee's nonce
                                                                   (create-instruction---createe-has-code)       ;; callee's HAS_CODE
                                                                   (create-instruction---current-context-csd)    ;; current  call  stack  depth
                                                                   (create-instruction---creator-nonce)          ;; creator account nonce
                                                                   (create-instruction---init-code-size)         ;; init code size
                                                                   )))

(defun    (create-instruction---createe-nonce)       (*    (scenario-shorthand---CREATE---load-createe-account)    (shift    account/NONCE       CREATE_first_createe_account_row___row_offset)))
(defun    (create-instruction---createe-has-code)    (*    (scenario-shorthand---CREATE---load-createe-account)    (shift    account/HAS_CODE    CREATE_first_createe_account_row___row_offset)))

(defconstraint    create-instruction---setting-the-CREATE-scenario---exceptional
                  (:guard    (create-instruction---generic-precondition))
                  (eq!   scenario/CREATE_EXCEPTION    XAHOY))

(defconstraint    create-instruction---setting-the-CREATE-scenario---unexceptional-generalities
                  (:guard    (create-instruction---generic-precondition))
                  (if-not-zero    (scenario-shorthand---CREATE---unexceptional)
                                  (begin
                                    (eq!    scenario/CREATE_ABORT                                    (create-instruction---OOB-aborting-condition))
                                    (eq!    (scenario-shorthand---CREATE---failure-condition)        (create-instruction---OOB-failure-condition) )
                                    (debug  (eq!    (scenario-shorthand---CREATE---not-rebuffed)     (-    1
                                                                                                           (create-instruction---OOB-aborting-condition)
                                                                                                           (create-instruction---OOB-failure-condition)))))))

(defconstraint    create-instruction---setting-the-CREATE-scenario---WILL_REVERT-scenarios
                  (:guard    (create-instruction---generic-precondition))
                  (if-not-zero    (scenario-shorthand---CREATE---creator-state-change)
                                  (eq!    (scenario-shorthand---CREATE---creator-state-change-will-revert)
                                          CONTEXT_WILL_REVERT)))

(defconstraint    create-instruction---setting-the-CREATE-scenario---not-rebuffed-scenarios
                  (:guard    (create-instruction---generic-precondition))
                  (if-not-zero    (scenario-shorthand---CREATE---not-rebuffed)
                                  (eq!    (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code)
                                          (create-instruction---MXP-s1nznomxpx))))

(defconstraint    create-instruction---setting-the-CREATE-scenario---not-rebuffed-nonempty-init-code
                  (:guard    (create-instruction---generic-precondition))
                  (if-not-zero    (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code)
                                  (eq!    (scenario-shorthand---CREATE---deployment-failure)
                                          (shift    misc/CCSR_FLAG    CREATE_miscellaneous_row___row_offset))))

(defconstraint    create-instruction---setting-the-MMU-instruction                          (:guard    (create-instruction---generic-precondition))
                  (if-not-zero    (shift    misc/MMU_FLAG    CREATE_miscellaneous_row___row_offset)
                                  (set-MMU-instruction---ram-to-exo-with-padding    CREATE_miscellaneous_row___row_offset               ;; offset
                                                                                    CONTEXT_NUMBER                                      ;; source ID
                                                                                    (create-instruction---tgt-id)                       ;; target ID
                                                                                    (create-instruction---aux-id)                       ;; auxiliary ID
                                                                                    ;; src_offset_hi                                       ;; source offset high
                                                                                    (create-instruction---STACK-offset-lo)              ;; source offset low
                                                                                    ;; tgt_offset_lo                                       ;; target offset low
                                                                                    (create-instruction---STACK-size-lo)                ;; size
                                                                                    ;; ref_offset                                          ;; reference offset
                                                                                    (create-instruction---STACK-size-lo)                ;; reference size
                                                                                    0                                                   ;; success bit
                                                                                    ;; limb_1                                              ;; limb 1
                                                                                    ;; limb_2                                              ;; limb 2
                                                                                    (create-instruction---exo-sum)                      ;; weighted exogenous module flag sum
                                                                                    0                                                   ;; phase
                                                                                    )))

(defun    (create-instruction---tgt-id)     (+    (*    (create-instruction---hash-init-code-and-send-to-ROM)    (create-instruction---deployment-cfi))
                                                  (*    (create-instruction---send-init-code-to-ROM)             (create-instruction---deployment-cfi))))

(defun    (create-instruction---aux-id)     (+    (*    (create-instruction---hash-init-code)                    (+    1    HUB_STAMP))
                                                  (*    (create-instruction---hash-init-code-and-send-to-ROM)    (+    1    HUB_STAMP))))

(defun    (create-instruction---exo-sum)    (+    (*    (create-instruction---hash-init-code)                                                 EXO_SUM_WEIGHT_KEC)
                                                  (*    (create-instruction---hash-init-code-and-send-to-ROM)    (+    EXO_SUM_WEIGHT_ROM     EXO_SUM_WEIGHT_KEC))
                                                  (*    (create-instruction---send-init-code-to-ROM)                   EXO_SUM_WEIGHT_ROM)))

(defconstraint    create-instruction---setting-the-next-context-number                      (:guard    (create-instruction---generic-precondition))
                  (begin
                    (if-not-zero    scenario/CREATE_EXCEPTION                                      (next-context-is-caller))
                    (if-not-zero    (scenario-shorthand---CREATE---no-context-change)                  (next-context-is-current))
                    (if-not-zero    (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code)    (next-context-is-new))))

(defun   (create-instruction---upfront-gas-cost)   (+   GAS_CONST_G_CREATE
                                                        (create-instruction---MXP-gas)))
(defun   (create-instruction---full-gas-cost)      (+   (create-instruction---upfront-gas-cost)
                                                        (create-instruction---STP-gas-paid-out-of-pocket)))


(defconstraint    create-instruction---setting-GAS_COST                                     (:guard    (create-instruction---generic-precondition))
                  (begin
                    (if-not-zero    (+    (create-instruction---STACK-staticx)
                                          (create-instruction---STACK-mxpx))
                                    (eq!  GAS_COST  0))
                    (if-not-zero    (+    (create-instruction---STACK-oogx)
                                          (scenario-shorthand---CREATE---unexceptional))
                                    (eq!  GAS_COST  (create-instruction---upfront-gas-cost)))
                    ))

(defconstraint    create-instruction---setting-GAS_NEXT                                     (:guard    (create-instruction---generic-precondition))
                  (begin
                    (if-not-zero    scenario/CREATE_EXCEPTION                                        (eq!   GAS_NEXT  0))
                    (if-not-zero    scenario/CREATE_ABORT                                            (eq!   GAS_NEXT  (-  GAS_ACTUAL  (create-instruction---upfront-gas-cost))))
                    (if-not-zero    (scenario-shorthand---CREATE---failure-condition)                (eq!   GAS_NEXT  (-  GAS_ACTUAL  (create-instruction---full-gas-cost))))
                    (if-not-zero    (scenario-shorthand---CREATE---not-rebuffed-empty-init-code)     (eq!   GAS_NEXT  (-  GAS_ACTUAL  (create-instruction---upfront-gas-cost))))
                    (if-not-zero    (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code)  (eq!   GAS_NEXT  (-  GAS_ACTUAL  (create-instruction---full-gas-cost))))
                  ))
