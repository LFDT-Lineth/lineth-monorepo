(module stp)

(defconst
  STP_CT_MAX_CALL           4
  STP_CT_MAX_CALL_OOGX      2
  STP_CT_MAX_CREATE         2
  STP_CT_MAX_CREATE_OOGX    1
  )

(defconstraint exclusive-flags ()
  (or! (eq! WCP_FLAG 0) (eq! MOD_FLAG 0)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;    2.2 inst decoding    ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun    (flag_sum)    (+ IS_CREATE
                           IS_CREATE2
                           IS_CALL
                           IS_CALLCODE
                           IS_DELEGATECALL
                           IS_STATICCALL))

(defun    (inst_sum)    (+ (* EVM_INST_CREATE       IS_CREATE)
                           (* EVM_INST_CREATE2      IS_CREATE2)
                           (* EVM_INST_CALL         IS_CALL)
                           (* EVM_INST_CALLCODE     IS_CALLCODE)
                           (* EVM_INST_DELEGATECALL IS_DELEGATECALL)
                           (* EVM_INST_STATICCALL   IS_STATICCALL)))

(defconstraint    no-stamp-no-flag ()
                  (if-zero STAMP
                           (eq! (flag_sum) 0)
                           (eq! (flag_sum) 1)))

(defconstraint inst-flag-relation ()
  (eq! INSTRUCTION (inst_sum)))

(defun (is_create)     (+ IS_CREATE IS_CREATE2))
(defun (is_call)       (+ IS_CALL IS_CALLCODE IS_DELEGATECALL IS_STATICCALL))
(defun (cctv)          (+ IS_CALL IS_CALLCODE))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.3 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint first-row (:domain {0}) ;; ""
  (vanishes! STAMP))

(defconstraint stamp-increments ()
  (or! (will-inc! STAMP 1) (will-remain-constant! STAMP)))

(defconstraint initial-vanishings ()
  (if-zero STAMP
           (begin (vanishes! CT)
                  (vanishes! CT_MAX)
                  (vanishes! (+ WCP_FLAG MOD_FLAG)))))

(defconstraint counter-reset ()
  (if-not-zero (will-remain-constant! STAMP)
               (vanishes! (next CT))))

(defconstraint heartbeat (:guard STAMP)
  (begin (if-eq-else CT CT_MAX (will-inc! STAMP 1) (will-inc! CT 1))
         (if-zero (is_create)
                  (if-zero OOGX
                           (eq!    CT_MAX    STP_CT_MAX_CALL)
                           (eq!    CT_MAX    STP_CT_MAX_CALL_OOGX))
                  (if-zero OOGX
                           (eq!    CT_MAX    STP_CT_MAX_CREATE)
                           (eq!    CT_MAX    STP_CT_MAX_CREATE_OOGX)))))


(defconstraint final-row (:domain {-1}) ;; ""
  (if-not-zero STAMP
               (eq! CT CT_MAX)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.4 constancies    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint counter-constancies ()
  (begin (counter-constancy    CT    INSTRUCTION)
         (counter-constancy    CT    GAS_ACTUAL)
         (counter-constancy    CT    GAS_MXP)
         (counter-constancy    CT    GAS_UPFRONT)
         (counter-constancy    CT    GAS_STIPEND)
         (counter-constancy    CT    GAS_OUT_OF_POCKET)
         ;;
         (counter-constancy    CT    GAS_HI)
         (counter-constancy    CT    GAS_LO)
         ;;
         (counter-constancy    CT    VAL_HI)
         (counter-constancy    CT    VAL_LO)
         ;;
         (counter-constancy    CT    EXISTS)
         (counter-constancy    CT    WARM)
         (counter-constancy    CT    OOGX)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;    2.5 vanishing constraints    ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    CREATE-type---debug---vanishing-constraints   (:guard    (is_create))
                  (begin
                    (vanishes!    GAS_HI)
                    (vanishes!    GAS_LO)))

(defconstraint    CALL-type---debug---non-value-transferring-opcodes-have-zero-value    ()
                  (if-not-zero    (+    IS_DELEGATECALL    IS_STATICCALL)
                                  (begin
                                    (vanishes!    VAL_HI)
                                    (vanishes!    VAL_LO))))

(defconstraint    CALL-type---debug---account-existence-only-matters-for-CALL    ()
                  (if-not-zero    (+    IS_CALLCODE    IS_DELEGATECALL    IS_STATICCALL)
                                  (vanishes!    EXISTS)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                ;;
;;    3 Constraints for CREATE-type instructions  ;;
;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (first-row-of-CREATE)                 (* (- STAMP (prev STAMP)) (is_create)))
(defun (first-row-of-unexceptional-CREATE)   (* (first-row-of-CREATE) (- 1 OOGX)))
(defun (create-gActual)                         GAS_ACTUAL) ;; ""
(defun (create-gPrelim)                      (+ GAS_MXP    GAS_CONST_G_CREATE))
(defun (create-gDiff)                        (- (create-gActual) (create-gPrelim)))
(defun (create-oneSixtyFourth)               (shift RES_LO 2))
(defun (create-LgDiff)                       (- (create-gDiff) (create-oneSixtyFourth)))

;; common rows of all CREATE instructions
(defconstraint    CREATE-type---common---row-i-plus-0
                  (:guard (first-row-of-CREATE))
                  (begin
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    ;;   ------------->   row i + 0   ;;
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    (vanishes!    ARG_1_HI)
                    (eq!          ARG_1_LO (create-gActual))
                    (vanishes!    ARG_2_LO)
                    (eq!          EXO_INST EVM_INST_LT)
                    (vanishes!    RES_LO)
                    (eq!          WCP_FLAG 1)))

(defconstraint    CREATE-type---common---row-i-plus-1
                  (:guard (first-row-of-CREATE))
                  (begin
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    ;;   ------------->   row i + 1   ;;
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    (vanishes!  (next ARG_1_HI))
                    (eq!        (next ARG_1_LO)    (create-gActual))
                    (eq!        (next ARG_2_LO)    (create-gPrelim))
                    (eq!        (next EXO_INST)    EVM_INST_LT)
                    (eq!        (next RES_LO)      OOGX)
                    (eq!        (next WCP_FLAG)    1)))

;; rows of CREATE instructions that don't produce an OOGX
(defconstraint    CREATE-type---unexceptional---row-i-plus-2
                  (:guard (first-row-of-unexceptional-CREATE))
                  (begin
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    ;;   ------------->   row i + 2   ;;
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    (vanishes!  (shift ARG_1_HI 2))
                    (eq!        (shift ARG_1_LO 2)    (create-gDiff))
                    (eq!        (shift ARG_2_LO 2)    64)
                    (eq!        (shift EXO_INST 2)    EVM_INST_DIV)
                    (eq!        (shift MOD_FLAG 2)    1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;   Setting GAS_UPFRONT and GAS_STPD   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint CREATE-type-outputs (:guard (first-row-of-CREATE))
               (begin (eq!       GAS_UPFRONT (create-gPrelim))
                      (vanishes! GAS_STIPEND)
                      (if-zero   OOGX
                                 (eq!       GAS_OUT_OF_POCKET    (- (shift ARG_1_LO 2) (shift RES_LO 2)))
                                 (vanishes! GAS_OUT_OF_POCKET))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                ;;
;;    4 Constraints for CALL-type instructions    ;;
;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (call---first-row-common)                 (*    (remained-constant! STAMP) (is_call)))
(defun (call---first-row-unexceptional)          (*    (call---first-row-common) (- 1 OOGX)))


(defun (call---transfers-value)                  (*    (cctv)    (-    1    (next RES_LO))))
(defun (call---zero-value)                       (*    (cctv)    (next RES_LO)))
(defun (call---gas-actual)                       GAS_ACTUAL)
(defun (call---gas-access-cost)                  (if-not-zero    WARM
                                                                 GAS_CONST_G_WARM_ACCESS
                                                                 GAS_CONST_G_COLD_ACCOUNT_ACCESS))
(defun (call---gas-value-transfer-cost)          (*    (call---transfers-value)
                                                       GAS_CONST_G_CALL_VALUE))
(defun (call---gas-new-account-cost)             (*    IS_CALL
                                                       (-    1    EXISTS)
                                                       (call---transfers-value)
                                                       GAS_CONST_G_NEW_ACCOUNT))
(defun (call---gas-extra)                        (+    (call---gas-access-cost)
                                                       (call---gas-new-account-cost)
                                                       (call---gas-value-transfer-cost)))
(defun (call---gas-prelim)                       (+    GAS_MXP
                                                       (call---gas-extra)))
(defun (call---one-sixty-fourth)                 (shift    RES_LO    3))
(defun (call---gas-to-L-comparison)              (shift    RES_LO    4))
(defun (call---gas-diff)                         (-    (call---gas-actual)
                                                       (call---gas-prelim)))
(defun (call---L-of-gas-diff)                    (-    (call---gas-diff)
                                                       (call---one-sixty-fourth)))
(defun (call---gas-Min)                          (if-zero    (force-bin    (call---gas-to-L-comparison))
                                                             (call---L-of-gas-diff)
                                                             GAS_LO))

;; common rows for CALL instructions
(defconstraint    CALL-type---common-part---row-i-plus-0
                  (:guard (call---first-row-common))
                  (begin
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    ;;   ------------->   row i + 0  ;;
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    (vanishes!   ARG_1_HI)
                    (eq!         ARG_1_LO (call---gas-actual))
                    (vanishes!   ARG_2_LO)
                    (eq!         EXO_INST EVM_INST_LT)
                    (vanishes!   RES_LO)
                    (eq!         WCP_FLAG 1)))

(defconstraint    CALL-type---common-part---row-i-plus-1
                  (:guard (call---first-row-common))
                  (begin
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    ;;   ------------->   row i + 1   ;;
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    (eq!               (next ARG_1_HI)    VAL_HI)
                    (eq!               (next ARG_1_LO)    VAL_LO)
                    (debug (vanishes!  (next ARG_2_LO)))
                    (eq!               (next EXO_INST)    EVM_INST_ISZERO)
                    (eq!               (next WCP_FLAG)    (cctv))
                    (vanishes!         (next MOD_FLAG))))

(defconstraint    CALL-type---common-part---row-i-plus-2
                  (:guard (call---first-row-common))
                  (begin
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    ;;   ------------->   row i + 2   ;;
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    (vanishes!  (shift ARG_1_HI 2))
                    (eq!        (shift ARG_1_LO 2)    (call---gas-actual))
                    (eq!        (shift ARG_2_LO 2)    (call---gas-prelim))
                    (eq!        (shift EXO_INST 2)    EVM_INST_LT)
                    (eq!        (shift RES_LO   2)    OOGX)
                    (eq!        (shift WCP_FLAG 2)    1)))

(defconstraint CALL-type---unexceptional---row-i-plus-3
               (:guard (call---first-row-unexceptional))
               (begin
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;;   ------------->   row i + 3   ;;
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (vanishes! (shift ARG_1_HI 3))
                 (eq! (shift ARG_1_LO 3) (call---gas-diff))
                 (eq! (shift ARG_2_LO 3) 64)
                 (eq! (shift EXO_INST 3) EVM_INST_DIV)
                 (eq! (shift MOD_FLAG 3) 1)))

(defconstraint CALL-type---unexceptional---row-i-plus-4
               (:guard (call---first-row-unexceptional))
               (begin
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;;   ------------->   row i + 4   ;;
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq! (shift ARG_1_HI 4) GAS_HI)
                 (eq! (shift ARG_1_LO 4) GAS_LO)
                 (eq! (shift ARG_2_LO 4) (call---L-of-gas-diff))
                 (eq! (shift EXO_INST 4) EVM_INST_LT)
                 (eq! (shift WCP_FLAG 4) 1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;   Setting GAS_UPFRONT   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    CALL-type---setting-upfront-gas
                  (:guard (call---first-row-common))
                  (eq! GAS_UPFRONT (call---gas-prelim)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;   Setting GAS_PAID_OUT_OF_POCKET   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    CALL-type---setting-gas-paid-out-of-pocket
                  (:guard (call---first-row-common))
                  (if-zero OOGX
                           (eq!       GAS_OUT_OF_POCKET    (call---gas-Min))
                           (vanishes! GAS_OUT_OF_POCKET)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;   Setting GAS_STIPEND   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    CALL-type---setting-gas-stipend (:guard (call---first-row-common))
                  (if-zero    OOGX
                              (eq! GAS_STIPEND
                                   (* (cctv)
                                      (- 1 (next RES_LO))
                                      GAS_CONST_G_CALL_STIPEND))
                              (vanishes! GAS_STIPEND)))
