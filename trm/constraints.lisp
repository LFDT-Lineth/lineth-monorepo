(module trm)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;; 1
(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint stamp-increment ()
 (or! (will-remain-constant! STAMP) (will-inc! STAMP 1)))

;; 3
(defconstraint null-stamp-null-columns ()
  (if-zero STAMP
           (begin (vanishes! RAW_ADDRESS_HI)
                  (vanishes! RAW_ADDRESS_LO)
                  (vanishes! TRM_ADDRESS_HI)
                  (vanishes! IS_PRECOMPILE)
                  (vanishes! (next CT))
                  (debug (vanishes! CT))
                  (debug (vanishes! BYTE_HI))
                  (debug (vanishes! BYTE_LO)))))

(defconstraint setting-first ()
  (eq! FIRST
       (- STAMP (prev STAMP))))

(defconstraint heartbeat (:guard STAMP)
  (begin  
         (if-not-zero (- TRM_CT_MAX CT)
                      (begin
                      (will-remain-constant! STAMP)
                      (will-inc! CT 1)))
         (if-zero (- TRM_CT_MAX CT)
                      (begin
                      (will-inc! STAMP 1)
                      (vanishes! (next CT))))))

(defconstraint last-row (:domain {-1})
  (if-not-zero STAMP
               (eq! CT TRM_CT_MAX)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;    2.2 stamp constancy   ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint stamp-constancies ()
  (begin (stamp-constancy STAMP RAW_ADDRESS_HI)
         (stamp-constancy STAMP RAW_ADDRESS_LO)
         (stamp-constancy STAMP IS_PRECOMPILE)
         (stamp-constancy STAMP TRM_ADDRESS_HI)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;    2.4 setting WCP calls   ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (wcpcall-leq offset arg1hi arg1lo arg2hi arg2lo)
(begin (eq! (shift INST offset) WCP_INST_LEQ)
       (eq! (shift ARG_1_HI offset) arg1hi)
       (eq! (shift ARG_1_LO offset) arg1lo)
       (eq! (shift ARG_2_HI offset) arg2hi)
       (eq! (shift ARG_2_LO offset) arg2lo)))

(defun (result-is-true offset)
  (eq! (shift RES offset) 1))

(defconstraint address-is-twenty-bytes (:guard FIRST)
  (begin 
  (wcpcall-leq ROW_OFFSET_ADDRESS 0 TRM_ADDRESS_HI RAW_ADDRESS_LO TWOFIFTYSIX_TO_THE_FOUR 0)
  (result-is-true ROW_OFFSET_ADDRESS)))

(defconstraint leading-bytes-is-twelve-bytes (:guard FIRST)
  (begin 
  (eq! (shift INST ROW_OFFSET_ADDRESS_TRM) WCP_INST_LEQ)
  (vanishes! (shift ARG_1_HI ROW_OFFSET_ADDRESS_TRM))
  (vanishes! (shift ARG_2_HI ROW_OFFSET_ADDRESS_TRM))
  (eq! (shift ARG_2_LO ROW_OFFSET_ADDRESS_TRM) TWOFIFTYSIX_TO_THE_TWELVE_MO)
  (result-is-true ROW_OFFSET_ADDRESS_TRM)))

(defconstraint address-is-not-zero (:guard FIRST)
  (begin 
  (eq! (shift INST ROW_OFFSET_NON_ZERO_ADDR) EVM_INST_ISZERO)
  (eq! (shift ARG_1_HI ROW_OFFSET_NON_ZERO_ADDR) TRM_ADDRESS_HI)
  (eq! (shift ARG_2_HI ROW_OFFSET_NON_ZERO_ADDR) RAW_ADDRESS_LO)
  ))

(defconstraint address-is-prc-range (:guard FIRST)
(wcpcall-leq ROW_OFFSET_PRC_ADDR TRM_ADDRESS_HI RAW_ADDRESS_LO 0 NUMBER_OF_PRECOMPILES))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;    2.5 target constraints  ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint setting-precompile (:guard FIRST)
 (eq! IS_PRECOMPILE
      (* (shift RES ROW_OFFSET_PRC_ADDR)
         (- 1 (shift RES ROW_OFFSET_NON_ZERA_ADDR)))))

(defun (leading-byte) 
  (shift ARG_1_LO ROW_OFFSET_ADDRESS_TRM))

(defconstraint proving-trm (:guard FIRST)
(eq! RAW_ADDRESS_HI
     (+ (* TWOFIFTYSIX_TO_THE_FOUR (leading-byte))
        TRM_ADDRESS_HI)))



