(module wcp)

(defun (flag-sum)
  (+ (one-line-inst) (variable-length-inst)))

(defun (weight-sum)
  (+
    (* EVM_INST_LT     IS_LT)
    (* EVM_INST_GT     IS_GT)
    (* EVM_INST_SLT    IS_SLT)
    (* EVM_INST_SGT    IS_SGT)
    (* EVM_INST_EQ     IS_EQ)
    (* EVM_INST_ISZERO IS_ISZERO)
    (* WCP_INST_GEQ    IS_GEQ)
    (* WCP_INST_LEQ    IS_LEQ)))

(defun (one-line-inst)
  (+ IS_EQ IS_ISZERO))

(defun (variable-length-inst)
  (+ IS_LT IS_GT IS_LEQ IS_GEQ IS_SLT IS_SGT))

(defconstraint inst-decoding ()
  (if-zero STAMP
           (vanishes! (flag-sum))
           (eq! (flag-sum) 1)))

(defconstraint setting-flag ()
  (begin (eq! INST (weight-sum))
         (eq! OLI (one-line-inst))
         (eq! VLI (variable-length-inst))))

(defconstraint counter-constancies ()
  (begin (counter-constancy CT ARG_1_HI)
         (counter-constancy CT ARG_1_LO)
         (counter-constancy CT ARG_2_HI)
         (counter-constancy CT ARG_2_LO)
         (counter-constancy CT RES)
         (counter-constancy CT INST)
         (counter-constancy CT CT_MAX)
         (counter-constancy CT BIT_3)
         (counter-constancy CT BIT_4)
         (counter-constancy CT NEG_1)
         (counter-constancy CT NEG_2)))

(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint stamp-increments ()
  (or! (will-remain-constant! STAMP) (will-inc! STAMP 1)))

(defconstraint counter-reset ()
  (if-not-zero (will-remain-constant! STAMP)
               (vanishes! (next CT))))

(defconstraint setting-ct-max ()
  (if-eq OLI 1 (vanishes! CT_MAX)))

(defconstraint heartbeat (:guard STAMP)
  (if-eq-else CT CT_MAX (will-inc! STAMP 1) (will-inc! CT 1)))

(defconstraint ct-upper-bond ()
  (eq! (~ (- LLARGE CT))
       1))

(defconstraint lastRow (:domain {-1})
  (eq! CT CT_MAX))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.6 byte decompositions   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; byte decompositions
(defconstraint byte_decompositions ()
  (begin (byte-decomposition CT ACC_1 BYTE_1)
         (byte-decomposition CT ACC_2 BYTE_2)
         (byte-decomposition CT ACC_3 BYTE_3)
         (byte-decomposition CT ACC_4 BYTE_4)
         (byte-decomposition CT ACC_5 BYTE_5)
         (byte-decomposition CT ACC_6 BYTE_6)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;    2.7 BITS and sign bit constraints    ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint bits-and-negs (:guard (+ IS_SLT IS_SGT))
  (if-eq CT LLARGEMO
         (begin (eq! (shift BYTE_1 (- 0 LLARGEMO))
                     (first-eight-bits-bit-dec))
                (eq! (shift BYTE_3 (- 0 LLARGEMO))
                     (last-eight-bits-bit-dec))
                (eq! NEG_1
                     (shift BITS (- 0 LLARGEMO)))
                (eq! NEG_2
                     (shift BITS (- 0 7))))))

(defconstraint no-neg-if-small ()
  (if-not-zero (- CT_MAX LLARGEMO)
               (begin (vanishes! NEG_1)
                      (vanishes! NEG_2))))

(defun (first-eight-bits-bit-dec)
  (reduce +
          (for i
               [0 :7]
               (* (^ 2 i)
                  (shift BITS
                         (- 0 (+ i 8)))))))

(defun (last-eight-bits-bit-dec)
  (reduce +
          (for i
               [0 :7]
               (* (^ 2 i)
                  (shift BITS (- 0 i))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.6 target constraints    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint target-constraints ()
  (begin (if-not-zero STAMP
                      (begin (if-eq-else ARG_1_HI ARG_2_HI (eq! BIT_1 1) (vanishes! BIT_1))
                             (if-eq-else ARG_1_LO ARG_2_LO (eq! BIT_2 1) (vanishes! BIT_2))))
         (if-eq VLI 1
                (if-eq CT CT_MAX
                       (begin (eq! ACC_1 ARG_1_HI)
                              (eq! ACC_2 ARG_1_LO)
                              (eq! ACC_3 ARG_2_HI)
                              (eq! ACC_4 ARG_2_LO)
                              (eq! ACC_5
                                   (- (* (- (* 2 BIT_3) 1)
                                         (- ARG_1_HI ARG_2_HI))
                                      BIT_3))
                              (eq! ACC_6
                                   (- (* (- (* 2 BIT_4) 1)
                                         (- ARG_1_LO ARG_2_LO))
                                      BIT_4)))))
         (if-eq IS_ISZERO 1
                (begin (vanishes! ARG_2_HI)
                       (vanishes! ARG_2_LO)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.7 result constraints    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; eq_ = [[1]] . [[2]]
;; gt_ = [[3]] + [[1]] . [[4]]
;; lt_ = 1 - eq - gt
(defun (eq_)
  (* BIT_1 BIT_2))

(defun (gt_)
  (+ BIT_3 (* BIT_1 BIT_4)))

(defun (lt_)
  (- 1 (eq_) (gt_)))

;; 2.7.2
(defconstraint result ()
  (begin (if-eq OLI 1 (eq! RES (eq_)))
         (if-eq IS_LT 1 (eq! RES (lt_)))
         (if-eq IS_GT 1 (eq! RES (gt_)))
         (if-eq IS_LEQ 1
                (eq! RES (+ (lt_) (eq_))))
         (if-eq IS_GEQ 1
                (eq! RES (+ (gt_) (eq_))))
         (if-eq IS_SLT 1
                (if-eq-else NEG_1 NEG_2 (eq! RES (lt_)) (eq! RES NEG_1)))
         (if-eq IS_SGT 1
                (if-eq-else NEG_1 NEG_2 (eq! RES (gt_)) (eq! RES NEG_2)))))


