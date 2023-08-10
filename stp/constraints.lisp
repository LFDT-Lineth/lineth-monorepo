(module stp)

(defconst
  DIV                       0x04
  LT                        0x10
  ISZERO                    0x15
  max_ct_CALL                  4
  max_ct_CREATE                2
  G_create                 32000
  G_warmaccess               100
  G_coldaccountaccess       2600
  G_callvalue               9000
  G_newaccount             25000
  )

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint first-row (:domain {0})
               (vanishes! STAMP))

(defconstraint stamp-increments ()
               (vanishes! (* (will-inc STAMP 1) (will-remain-constant STAMP))))

(defconstraint initial-vanishings ()
               (if-zero STAMP
                        (begin 
                          (vanishes! CT)
                          (vanishes! (+ WCP_FLAG MOD_FLAG)))))

(defconstraint counter-reset ()
               (if-not-zero (will-remain-constant STAMP)
                            (vanishes! (next CT))))

(defconstraint heartbeat (:guard STAMP)
               (if-zero INST_TYPE
                        ;; INST_TYPE = 0 i.e. dealing will CALL-type instructions
                        (if-eq-else CT max_ct_CALL
                                    ;; CT == max_ct_CALL
                                    (will-inc STAMP 1)
                                    ;; CT != max_ct_CALL
                                    (will-inc CT 1))
                        ;; INST_TYPE = 1 i.e. dealing will CREATE-type instructions
                        (if-eq-else CT max_ct_CREATE
                                    ;; CT == max_ct_CREATE
                                    (will-inc STAMP 1)
                                    ;; CT != max_ct_CREATE
                                    (will-inc CT 1))))

(defconstraint final-row (:domain {-1})
               (if-not-zero STAMP
                            (if-zero INST_TYPE
                                     ;; INST_TYPE = 0  <==>  CALL-type instruction
                                     (= CT max_ct_CALL)
                                     ;; INST_TYPE = 1  <==>  CREATE-type instruction
                                     (= CT max_ct_CREATE))))


;;;;;;;;;;;;;;;;;;;;;;
;;                  ;;
;;    2.2 binary    ;;
;;                  ;;
;;;;;;;;;;;;;;;;;;;;;;


(defconstraint binary-constraints ()
               (begin
                 (is-binary WCP_FLAG)
                 (is-binary MOD_FLAG)
                 (is-binary INST_TYPE)
                 (is-binary CCTV)))

(defconstraint exclusive-flags ()
               (vanishes! (* WCP_FLAG MOD_FLAG)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.4 constancies    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint counter-constancies ()
               (begin
                 (counter-constancy CT GAS_ACTUAL)
                 (counter-constancy CT GAS_MXP)
                 (counter-constancy CT GAS_COST)
                 (counter-constancy CT GAS_STIPEND)
                 ;
                 (counter-constancy CT GAS_HI)
                 (counter-constancy CT GAS_LO)
                 ;
                 (counter-constancy CT VAL_HI)
                 (counter-constancy CT VAL_LO)
                 ;
                 (counter-constancy CT TO_EXISTS)
                 (counter-constancy CT TO_WARM)
                 ;
                 ;
                 (counter-constancy CT OOGX)
                 (counter-constancy CT INST_TYPE)
                 (counter-constancy CT CCTV)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                    ;;
;;    3.2 Constraints for CREATE-type instructions    ;;
;;                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (first-row-of-CREATE)       (* (did-remain-constant STAMP) INST_TYPE))
(defun (create-gActual)            GAS_ACTUAL)
(defun (create-gPrelim)            (+ GAS_MXP G_create))
(defun (create-gDiff)              (- (create-gActual) (create-gPrelim)))
(defun (create-oneSixtyFourth)     (shift RES_LO 2))
(defun (create-LgDiff)             (- (create-gDiff) (create-oneSixtyFourth)))

(defconstraint CREATE-type (:guard (first-row-of-CREATE))
               (begin
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;;   ------------->   row i   ;;
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (vanishes! ARG_ONE_HI)
                 (= ARG_ONE_LO (create-gActual))
                 (vanishes! ARG_TWO_LO)
                 (= EXO_INST LT)
                 (vanishes! RES_LO)
                 (= WCP_FLAG 1)
                 ;; (= DIV_FLAG 0)
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;;   ------------->   row i + 1   ;;
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (vanishes! (next ARG_ONE_HI))
                 (= (next ARG_ONE_LO) (create-gActual))
                 (= (next ARG_TWO_LO) (create-gPrelim))
                 (= (next EXO_INST) LT)
                 (= (next RES_LO) OOGX)
                 (= (next WCP_FLAG) 1)
                 ;; (= (next DIV_FLAG) 0)
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;;   ------------->   row i + 2   ;;
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (vanishes! (shift ARG_ONE_HI 2))
                 (= (shift ARG_ONE_LO 2) (create-gDiff))
                 (= (shift ARG_TWO_LO 2) 64)
                 (= (shift EXO_INST 2) DIV)
                 ;; (= (shift RES_LO 2) ...)
                 (vanishes! (shift WCP_FLAG 2))
                 (= (shift DIV_FLAG 2) (- 1 OOGX))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;;   Setting GAS_COST and GAS_STPD   ;;
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero OOGX
                          ;; OOGX == 0
                          (begin
                            (= GAS_COST (+ (create-gPrelim) (create-LgDiff)))
                            (= GAS_STIPEND (create-LgDiff)))
                          ;; OOGX == 1
                          (begin
                            (= GAS_COST (create-gPrelim))
                            (vanishes! GAS_STIPEND)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                  ;;
;;    3.2 Constraints for CALL-type instructions    ;;
;;                                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; all of these definitions implicitly assume that the current row (i) is such that
;; - STAMP[i] != STAMP[i - 1]       <= first row of the instruction
;; - INST_TYPE = 0                  <= CALL-type instruction
(defun (first-row-of-CALL)          (* (did-remain-constant STAMP)
                                       (- 1 INST_TYPE)))
(defun (call-valueIsZero)           (next RES_LO))
(defun (call-gActual)               GAS_ACTUAL)
(defun (call-gAccess)               (+ (* TO_WARM G_warmaccess)
                                       (* (- 1 TO_WARM) G_coldaccountaccess)))
(defun (call-gCreate)               (* (- 1 TO_EXISTS)
                                       G_newaccount))
(defun (call-gTransfer)             (* CCTV
                                       (- 1 (valueIsZero))
                                       G_callvalue))
(defun (call-gXtra)                 (+ (call-gAccess)
                                       (call-gCreate)
                                       (call-gTransfer)))
(defun (call-gPrelim)               (+ GAS_MXP
                                       (call-gXtra)))
(defun (call-oneSixtyFourths)       (shift RES_LO 3))
(defun (call-gDiff)                 (- (call-gActual)
                                       (call-gPrelim)))
(defun (call-stpComp)               (shift RES_LO 4))
(defun (call-LgDiff)                (- (call-gDiff)
                                       (call-oneSixtyFourths)))
(defun (call-gMin)                  (+ (* (- 1 (call-stpComp)) (call-LgDiff))
                                       (* (call-stpComp) GAS_LO)))


(defconstraint CALL-type (:guard (first-row-of-CALL))
               (begin
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;;   ------------->   row i   ;;
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (vanishes! ARG_ONE_HI)
                 (= ARG_ONE_LO (call-gActual))
                 (vanishes! ARG_TWO_LO)
                 (= EXO_INST LT)
                 (vanishes! RES_LO)
                 (= WCP_FLAG 1)
                 ;; (= DIV_FLAG 0)
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;;   ------------->   row i + 1   ;;
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (= (next ARG_ONE_HI) VAL_HI)
                 (= (next ARG_ONE_LO) VAL_LO)
                 (vanishes! (next ARG_TWO_LO))
                 (= (next EXO_INST) ISZERO)
                 ;; (= (next RES_LO) ...)
                 (= (next WCP_FLAG) CCTV)
                 (vanishes! (next DIV_FLAG))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;;   ------------->   row i + 2   ;;
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (vanishes! (shift ARG_ONE_HI 2))
                 (= (shift ARG_ONE_LO 2) (call-gActual))
                 (= (shift ARG_TWO_LO 2) (call-gPrelim))
                 (= (shift EXO_INST 2) LT)
                 (= (shift RES_LO 2) OOGX)
                 (= (shift WCP_FLAG 2) 1)
                 ;; (vanishes! (shift DIV_FLAG 2))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;;   ------------->   row i + 3   ;;
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (vanishes! (shift ARG_ONE_HI 3))
                 (if-zero OOGX (= (shift ARG_ONE_LO 3) (call-gDiff)))
                 (= (shift ARG_TWO_LO 3) 64)
                 (= (shift EXO_INST 3) DIV)
                 ;; (= (shift RES_LO 3) ...)
                 (vanishes! (shift WCP_FLAG 3))
                 (= (shift DIV_FLAG 3) (- 1 OOGX))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;;   ------------->   row i + 4   ;;
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (= (shift ARG_ONE_HI 4) GAS_HI)
                 (= (shift ARG_ONE_LO 4) GAS_LO)
                 (if-zero OOGX (= (shift ARG_TWO_LO 4) (call-LgDiff)))
                 (= (shift EXO_INST 4) LT)
                 ;; (= (shift RES_LO 4) ...)
                 (= (shift WCP_FLAG 4) (- 1 OOGX))
                 (vanishes! (shift DIV_FLAG 4))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;;   Setting GAS_COST and GAS_STPD   ;;
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero OOGX
                          ;; OOGX == 0
                          (begin
                            (= GAS_COST (+ (call-gPrelim) (call-gMin)))
                            (= GAS_STIPEND (+ (call-gMin)
                                              (* CCTV (- 1 (call-valueIsZero)) G_callstipend)))
                          ;; OOGX == 1
                          (begin
                            (= GAS_COST (call-gPrelim))
                            (vanishes! GAS_STIPEND))))))
