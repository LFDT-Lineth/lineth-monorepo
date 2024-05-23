(module stp)

(defconst 
  max_CT_CALL         4
  max_CT_CALL_OOGX    2
  G_create            32000
  G_warmaccess        100
  G_coldaccountaccess 2600
  G_callvalue         9000
  G_newaccount        25000
  G_callstipend       2300)

(defconstraint exclusive-flags ()
  (vanishes! (* WCP_FLAG MOD_FLAG)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;    2.2 inst decoding    ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (flag_sum)
  (+ IS_CREATE IS_CREATE2 IS_CALL IS_CALLCODE IS_DELEGATECALL IS_STATICCALL))

(defun (inst_sum)
  (+ (* EVM_INST_CREATE IS_CREATE)
     (* EVM_INST_CREATE2 IS_CREATE2)
     (* EVM_INST_CALL IS_CALL)
     (* EVM_INST_CALLCODE IS_CALLCODE)
     (* EVM_INST_DELEGATECALL IS_DELEGATECALL)
     (* EVM_INST_STATICCALL IS_STATICCALL)))

(defconstraint no-stamp-no-flag ()
  (if-zero STAMP
           (vanishes! (flag_sum))
           (eq! (flag_sum) 1)))

(defconstraint inst-flag-relation ()
  (eq! INSTRUCTION (inst_sum)))

(defun (is_create)
  (+ IS_CREATE IS_CREATE2))

(defun (is_call)
  (+ IS_CALL IS_CALLCODE IS_DELEGATECALL IS_STATICCALL))

(defun (cctv)
  (+ IS_CALL IS_CALLCODE))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.3 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint stamp-increments ()
  (vanishes! (any! (will-inc! STAMP 1) (will-remain-constant! STAMP))))

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
                           (eq! CT_MAX 4)
                           (eq! CT_MAX 2))
                  (if-zero OOGX
                           (eq! CT_MAX 2)
                           (eq! CT_MAX 1)))))

(defconstraint final-row (:domain {-1})
  (if-not-zero STAMP
               (eq! CT CT_MAX)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.4 constancies    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint counter-constancies ()
  (begin (counter-constancy CT INSTRUCTION)
         (counter-constancy CT GAS_ACTUAL)
         (counter-constancy CT GAS_MXP)
         (counter-constancy CT GAS_UPFRONT)
         (counter-constancy CT GAS_STIPEND)
         (counter-constancy CT GAS_OUT_OF_POCKET)
         ;
         (counter-constancy CT GAS_HI)
         (counter-constancy CT GAS_LO)
         ;
         (counter-constancy CT VAL_HI)
         (counter-constancy CT VAL_LO)
         ;
         (counter-constancy CT EXISTS)
         (counter-constancy CT WARM)
         (counter-constancy CT OOGX)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;    2.5 vanishing constraints    ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint callcode-impose-exists ()
  (if-not-zero IS_CALLCODE
               (eq! EXISTS 1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                ;;
;;    3 Constraints for CREATE-type instructions  ;;
;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (first-row-of-CREATE)
  (* (- STAMP (prev STAMP))
     (is_create)))

(defun (first-row-of-unexceptional-CREATE)
  (* (first-row-of-CREATE) (- 1 OOGX)))

(defun (create-gActual)
  GAS_ACTUAL)

(defun (create-gPrelim)
  (+ GAS_MXP G_create))

(defun (create-gDiff)
  (- (create-gActual) (create-gPrelim)))

(defun (create-oneSixtyFourth)
  (shift RES_LO 2))

(defun (create-LgDiff)
  (- (create-gDiff) (create-oneSixtyFourth)))

;; common rows of all CREATE instructions
(defconstraint CREATE-type-common (:guard (first-row-of-CREATE))
  (begin  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         ;;   ------------->   row i   ;;
         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         (vanishes! ARG_1_HI)
         (eq! ARG_1_LO (create-gActual))
         (vanishes! ARG_2_LO)
         (eq! EXO_INST EVM_INST_LT)
         (vanishes! RES_LO)
         (eq! WCP_FLAG 1)
         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         ;;   ------------->   row i + 1   ;;
         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         (vanishes! (next ARG_1_HI))
         (eq! (next ARG_1_LO) (create-gActual))
         (eq! (next ARG_2_LO) (create-gPrelim))
         (eq! (next EXO_INST) EVM_INST_LT)
         (eq! (next RES_LO) OOGX)
         (eq! (next WCP_FLAG) 1)))

;; rows of CREATE instructions that don't produce an OOGX
(defconstraint CREATE-type-no-exception (:guard (first-row-of-unexceptional-CREATE))
  (begin  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         ;;   ------------->   row i + 2   ;;
         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         (vanishes! (shift ARG_1_HI 2))
         (eq! (shift ARG_1_LO 2) (create-gDiff))
         (eq! (shift ARG_2_LO 2) 64)
         (eq! (shift EXO_INST 2) EVM_INST_DIV)
         (eq! (shift MOD_FLAG 2) 1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;   Setting GAS_UPFRONT and GAS_STPD   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint CREATE-type-outputs (:guard (first-row-of-CREATE))
  (begin (eq! GAS_UPFRONT (create-gPrelim))
         (vanishes! GAS_STIPEND)
         (if-zero OOGX
                  (eq! GAS_OUT_OF_POCKET
                       (- (shift ARG_1_LO 2) (shift RES_LO 2)))
                  (vanishes! GAS_OUT_OF_POCKET))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                ;;
;;    4 Constraints for CALL-type instructions    ;;
;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; all of these definitions implicitly assume that the current row (i) is such that
;; - STAMP[i] !eq! STAMP[i - 1]       <eq! first row of the instruction
;; - INST_TYPE eq! 0                  <eq! CALL-type instruction
(defun (first-row-of-CALL)
  (* (remained-constant! STAMP) (is_call)))

(defun (first-row-of-unexceptional-CALL)
  (* (first-row-of-CALL) (- 1 OOGX)))

(defun (call-valueIsZero)
  (next RES_LO))

(defun (call-gActual)
  GAS_ACTUAL)

(defun (call-gAccess)
  (+ (* WARM G_warmaccess)
     (* (- 1 WARM) G_coldaccountaccess)))

(defun (call-gNewAccount)
  (* (- 1 EXISTS) G_newaccount))

(defun (call-gTransfer)
  (* (cctv) (- 1 (call-valueIsZero)) G_callvalue))

(defun (call-gXtra)
  (+ (call-gAccess) (call-gNewAccount) (call-gTransfer)))

(defun (call-gPrelim)
  (+ GAS_MXP (call-gXtra)))

(defun (call-oneSixtyFourths)
  (shift RES_LO 3))

(defun (call-gDiff)
  (- (call-gActual) (call-gPrelim)))

(defun (call-stpComp)
  (shift RES_LO 4))

(defun (call-LgDiff)
  (- (call-gDiff) (call-oneSixtyFourths)))

(defun (call-gMin)
  (+ (* (- 1 (call-stpComp)) (call-LgDiff))
     (* (call-stpComp) GAS_LO)))

;; common rows of all CALL instructions
(defconstraint CALL-type-common (:guard (first-row-of-CALL))
  (begin  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         ;;   ------------->   row i   ;;
         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         (vanishes! ARG_1_HI)
         (eq! ARG_1_LO (call-gActual))
         (vanishes! ARG_2_LO)
         (eq! EXO_INST EVM_INST_LT)
         (vanishes! RES_LO)
         (eq! WCP_FLAG 1)
         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         ;;   ------------->   row i + 1   ;;
         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         (eq! (next ARG_1_HI) VAL_HI)
         (eq! (next ARG_1_LO) VAL_LO)
         (debug (vanishes! (next ARG_2_LO)))
         (eq! (next EXO_INST) EVM_INST_ISZERO)
         (eq! (next WCP_FLAG) (cctv))
         (vanishes! (next MOD_FLAG))
         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         ;;   ------------->   row i + 2   ;;
         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         (vanishes! (shift ARG_1_HI 2))
         (eq! (shift ARG_1_LO 2) (call-gActual))
         (eq! (shift ARG_2_LO 2) (call-gPrelim))
         (eq! (shift EXO_INST 2) EVM_INST_LT)
         (eq! (shift RES_LO 2) OOGX)
         (eq! (shift WCP_FLAG 2) 1)))

;; 
(defconstraint CALL-type-no-exception (:guard (first-row-of-unexceptional-CALL))
  (begin  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         ;;   ------------->   row i + 3   ;;
         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         (vanishes! (shift ARG_1_HI 3))
         (eq! (shift ARG_1_LO 3) (call-gDiff))
         (eq! (shift ARG_2_LO 3) 64)
         (eq! (shift EXO_INST 3) EVM_INST_DIV)
         (eq! (shift MOD_FLAG 3) 1)
         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         ;;   ------------->   row i + 4   ;;
         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
         (eq! (shift ARG_1_HI 4) GAS_HI)
         (eq! (shift ARG_1_LO 4) GAS_LO)
         (eq! (shift ARG_2_LO 4) (call-LgDiff))
         (eq! (shift EXO_INST 4) EVM_INST_LT)
         (eq! (shift WCP_FLAG 4) 1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;   Setting GAS_UPFRONT and GAS_STPD   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint CALL-type-outputs (:guard (first-row-of-CALL))
  (begin (eq! GAS_UPFRONT (call-gPrelim))
         (if-zero OOGX
                  (begin (eq! GAS_OUT_OF_POCKET (call-gMin))
                         (eq! GAS_STIPEND
                              (* (cctv)
                                 (- 1 (next RES_LO))
                                 G_callstipend)))
                  (begin (vanishes! GAS_OUT_OF_POCKET)
                         (vanishes! GAS_STIPEND)))))


