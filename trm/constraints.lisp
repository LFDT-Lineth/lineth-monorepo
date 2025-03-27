(module trm)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;; 1
(defconstraint first-row (:domain {0}) ;; ""
  (vanishes! IOMF))

(defconstraint iomf-increment ()
 (or! (will-remain-constant! IOMF) (will-inc! IOMF 1)))

;; 3
(defconstraint automatic-vanishing-constraints-along-padding-rows ()
  (if-zero IOMF
           (begin (vanishes! RAW_ADDRESS_HI)
                  (vanishes! RAW_ADDRESS_LO)
                  (vanishes! TRM_ADDRESS_HI)
                  (vanishes! IS_PRECOMPILE)
                  (vanishes! (next CT))
                  (debug (vanishes! CT)))))

(defconstraint constraining-FIRST (:guard IOMF)
               (begin
                 (if-zero            CT (eq! FIRST 1))
                 (debug (if-not-zero CT (eq! FIRST 0)))))

(defconstraint counter-cycle-constraints (:guard IOMF)
               (if-zero (- TRM_CT_MAX CT)
                        ;; CT = CT MAX
                        (vanishes! (next CT))
                        ;; CT != CT MAX
                        (will-inc! CT 1)))

(defconstraint finalization-constraint (:domain {-1} :guard IOMF) ;;""
               (eq! CT TRM_CT_MAX))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.2 counter constancies   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint counter-constancies ()
  (begin (counter-constancy CT RAW_ADDRESS_HI)
         (counter-constancy CT RAW_ADDRESS_LO)
         (counter-constancy CT TRM_ADDRESS_HI)
         (counter-constancy CT IS_PRECOMPILE)
         ))


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.4 computations   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (wcpcall   offset
                  inst
                  arg1hi
                  arg1lo
                  arg2hi
                  arg2lo)
  (begin (eq! (shift INST     offset) inst)
         (eq! (shift ARG_1_HI offset) arg1hi)
         (eq! (shift ARG_1_LO offset) arg1lo)
         (eq! (shift ARG_2_HI offset) arg2hi)
         (eq! (shift ARG_2_LO offset) arg2lo)))

(defun (result-is-true offset)
  (eq! (shift RES offset) 1))

;;
;; Processing row n°0
;;
(defconstraint trimmed-address-is-20-bytes-integer (:guard FIRST)
               (begin
                 (wcpcall ROW_OFFSET_ADDRESS
                          EVM_INST_LT
                          TRM_ADDRESS_HI
                          RAW_ADDRESS_LO
                          TWOFIFTYSIX_TO_THE_FOUR
                          0)
                 (result-is-true ROW_OFFSET_ADDRESS)))

;;
;; Processing row n°1
;;
(defconstraint leading-bytes-is-twelve-bytes (:guard FIRST)
  (begin
  (eq!        (shift  INST      ROW_OFFSET_ADDRESS_TRM) WCP_INST_LEQ)
  (vanishes!  (shift  ARG_1_HI  ROW_OFFSET_ADDRESS_TRM))
;;            (shift  ARG_1_LO  ROW_OFFSET_ADDRESS_TRM)   ;; it is what it ... later on: (leading-bytes)
  (vanishes!  (shift  ARG_2_HI  ROW_OFFSET_ADDRESS_TRM))
  (eq!        (shift  ARG_2_LO  ROW_OFFSET_ADDRESS_TRM) TWOFIFTYSIX_TO_THE_TWELVE_MO)
  (result-is-true ROW_OFFSET_ADDRESS_TRM)))

(defun  (leading-bytes)  (shift ARG_1_LO ROW_OFFSET_ADDRESS_TRM))

(defconstraint address-decomposition-constraint (:guard FIRST)
               (eq! RAW_ADDRESS_HI
                    (+ (* TWOFIFTYSIX_TO_THE_FOUR (leading-bytes))
                       TRM_ADDRESS_HI)))

;;
;; Processing row n°2
;;
(defconstraint iszero-check-for-address (:guard FIRST)
  (begin
  (eq!  (shift  INST      ROW_OFFSET_NON_ZERO_ADDR)  EVM_INST_ISZERO)
  (eq!  (shift  ARG_1_HI  ROW_OFFSET_NON_ZERO_ADDR)  TRM_ADDRESS_HI)
  (eq!  (shift  ARG_1_LO  ROW_OFFSET_NON_ZERO_ADDR)  RAW_ADDRESS_LO)
  ))

(defun (address-is-zero)    (shift  RES  ROW_OFFSET_NON_ZERO_ADDR))
(defun (address-is-nonzero) (- 1 (address-is-zero)))

;;
;; Processing row n°3
;;
(defconstraint address-is-prc-range (:guard FIRST)
               (wcpcall   ROW_OFFSET_PRC_ADDR
                          WCP_INST_LEQ
                          TRM_ADDRESS_HI
                          RAW_ADDRESS_LO
                          0
                          MAX_PRC_ADDRESS))

(defun (address-at-most-max-PRC)    (shift  RES  ROW_OFFSET_PRC_ADDR))

(defconstraint justifying-the-precompile-flag (:guard FIRST)
               (eq! IS_PRECOMPILE
                    (* (address-is-nonzero)
                       (address-at-most-max-PRC))))
