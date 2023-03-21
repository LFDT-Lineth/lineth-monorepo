(module wcp)

;; opcode values
(defconst
  LT                   16
  GT                   17
  SLT                  18
  SGT                  19
  EQ_                  20
  ISZERO               21
  LIMB_SIZE            16
  LIMB_SIZE_MINUS_ONE 15)


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;


;; 2.1.1)
(defconstraint forst-row (:domain {0}) (vanishes STAMP))

;; 2.1.2)
(defconstraint stampIncrements ()
  (vanishes (* (inc STAMP 0)
               (inc STAMP 1))))

;; 2.1.3)
(defconstraint zeroRow (:guard (is-zero STAMP))
  (begin
   (vanishes OLI)
   (vanishes CT)))

;; 2.1.4)
(defconstraint counterReset ()
  (if-not-zero (remains-constant STAMP)
               (vanishes (next CT))))

;; 2.1.5)
(defconstraint heartbeat (:guard STAMP)
  (if-zero OLI
           ;; 2.1.5.a)
           ;; If OLI == 0
           (if-eq-else CT LIMB_SIZE_MINUS_ONE
                       ;; 2.1.5.a).(ii)
                       ;; If CT == LIMB_SIZE_MINUS_ONE (i.e. 15)
                       (inc STAMP 1)
                       ;; 2.1.5.a).(ii)
                       ;; If CT != LIMB_SIZE_MINUS_ONE (i.e. 15)
                       (begin (inc CT 1)
                              (vanishes (shift OLI 1))))
           ;; 2.1.5.a)
           ;; If OLI == 1
           (inc STAMP 1)))
;; 2.1.6)
(defconstraint lastRow (:domain {-1} :guard STAMP)
  (if-zero OLI
           (eq CT LIMB_SIZE_MINUS_ONE)))

;; stamp constancies
(defconstraint stamp-constancies ()
  (begin
   (stamp-constancy STAMP ARG_1_HI)
   (stamp-constancy STAMP ARG_1_LO)
   (stamp-constancy STAMP ARG_2_HI)
   (stamp-constancy STAMP ARG_2_LO)
   (stamp-constancy STAMP RES_HI)
   (stamp-constancy STAMP RES_LO)
   (stamp-constancy STAMP INST)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.2 counter constancy    ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint counter-constancies ()
  (begin
   (counter-constancy CT BIT_1)
   (counter-constancy CT BIT_2)
   (counter-constancy CT BIT_3)
   (counter-constancy CT BIT_4)
   (counter-constancy CT NEG_1)
   (counter-constancy CT NEG_2)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.3 byte decompositions   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; byte decompositions
(defconstraint byte_decompositions ()
  (begin
   (byte-decomposition CT ACC_1 BYTE_1)
   (byte-decomposition CT ACC_2 BYTE_2)
   (byte-decomposition CT ACC_3 BYTE_3)
   (byte-decomposition CT ACC_4 BYTE_4)
   (byte-decomposition CT ACC_5 BYTE_5)
   (byte-decomposition CT ACC_6 BYTE_6)))

;; binary constraints
(defconstraint binary_constraints ()
  (begin
   (is-binary BIT_1)
   (is-binary BIT_2)
   (is-binary BIT_3)
   (is-binary BIT_4)
   (is-binary BITS)
   (is-binary NEG_1)
   (is-binary NEG_2)))

;; bytehood constraints: TODO


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                            ;;
;;    2.4 ONE_LINE_INSTRUCTION constraints    ;;
;;                                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 2.4.1), 2), 3) and 4)
(defconstraint oli-constraints ()
  (begin
   (is-binary OLI)
   (if-eq INST EQ_ (= OLI 1))
   (if-eq INST ISZERO (= OLI 1))
   (if-not-zero (* (- INST EQ_) (- INST ISZERO))
                (vanishes OLI))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;    2.5 BITS and sign bit constraints    ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 2.5.1 DONE
;; 2.5.2 DONE
;; 2.5.3
(defconstraint bits_and_negs (:guard STAMP)
  (if-zero OLI
           (if-zero CT
                    (begin
                     (= BYTE_1 (first-eight-bits-bit-dec))
                     (= BYTE_3 (last-eight-bits-bit-dec))
                     (= NEG_1 BITS)
                     (= NEG_2 (shift BITS 8))))))

(defun (first-eight-bits-bit-dec)
    (+ (* 128 BITS)
       (* 64 (shift BITS 1))
       (* 32 (shift BITS 2))
       (* 16 (shift BITS 3))
       (* 8 (shift BITS 4))
       (* 4 (shift BITS 5))
       (* 2 (shift BITS 6))
       (shift BITS 7)))

(defun (last-eight-bits-bit-dec)
    (+ (* 128 (shift BITS 8))
       (* 64 (shift BITS 9))
       (* 32 (shift BITS 10))
       (* 16 (shift BITS 11))
       (* 8 (shift BITS 12))
       (* 4 (shift BITS 13))
       (* 2 (shift BITS 14))
       (shift BITS 15)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.6 target constraints    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint target-constraints ()
  (begin
   (if-not-zero STAMP
                (begin (if-eq-else ARG_1_HI ARG_2_HI
                                   (= BIT_1 1)
                                   (= BIT_1 0))
                       (if-eq-else ARG_1_LO ARG_2_LO
                                   (= BIT_2 1)
                                   (= BIT_2 0))))
   (if-eq CT LIMB_SIZE_MINUS_ONE
          (begin (= ACC_1 ARG_1_HI)
                 (= ACC_2 ARG_1_LO)
                 (= ACC_3 ARG_2_HI)
                 (= ACC_4 ARG_2_LO)
                 (= ACC_5
                    (- (* (- (* 2 BIT_3) 1)(- ARG_1_HI ARG_2_HI))
                       BIT_3))
                 (= ACC_6
                    (- (* (- (* 2 BIT_4) 1)
                          (- ARG_1_LO ARG_2_LO))
                       BIT_4))))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.7 result constraints    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; eq_ = [[1]] . [[2]]
;; gt_ = [[3]] + [[1]] . [[4]]
;; lt_ = 1 - eq - gt
(defun (eq_) (* BIT_1 BIT_2))
(defun (gt_) (+ BIT_3 (* BIT_1 BIT_4)))
(defun (lt_) (- 1 (eq_) (gt_)))


;; 2.7.1
(defconstraint result_hi () (vanishes RES_HI))

;; 2.7.2
(defconstraint result_lo (:guard STAMP)
  (if-zero OLI
           ;; 2.7.2.(b)
           ;; If OLI == 0
           (begin (if-eq INST LT
                         (= RES_LO (lt_)))
                  (if-eq INST GT
                         (= RES_LO (gt_)))
                  (if-eq INST SLT
                         (if-eq-else NEG_1 NEG_2
                                     (= RES_LO (lt_))
                                     (= RES_LO NEG_1)))
                  (if-eq INST SGT
                         (if-eq-else NEG_1 NEG_2
                                     (= RES_LO (gt_))
                                     (= RES_LO NEG_2))))
           ;; 2.7.2.(a)
           ;; If OLI == 1
           (= RES_LO (eq_))))
