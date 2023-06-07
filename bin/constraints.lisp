(module bin)

(defconst
  ;; opcode values
  SIGNEXTEND              11
  AND                     22
  OR                      23
  XOR                     24
  NOT                     25
  BYTE                    26
  ;; constant values
  LIMB_SIZE               16
  LIMB_SIZE_MINUS_ONE     15)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;; 2.1.1)
(defconstraint firstRow (:domain {0})
  (vanishes STAMP))

;; 2.1.2)
(defconstraint stampIncrements ()
  (vanishes (* (will-inc STAMP 0)
               (will-inc STAMP 1))))

;; 2.1.3)
(defconstraint zeroRow (:guard (is-zero STAMP))
  (begin (vanishes OLI)
         (vanishes CT)))

;; 2.1.4)
(defconstraint counterReset ()
  (if-not-zero (remains-constant STAMP)
               (vanishes (shift CT 1))))

;; 2.1.5)
(defconstraint heartbeat ()
  (if-not-zero STAMP
               (if-zero OLI
                        ;; 2.1.5.a)
                        ;; If OLI == 0
                        (if-eq-else CT LIMB_SIZE_MINUS_ONE
                                    ;; 2.1.5.a).(ii)
                                    ;; If CT == LIMB_SIZE_MINUS_ONE (i.e. 15)
                                    (will-inc STAMP 1)
                                    ;; 2.1.5.a).(ii)
                                    ;; If CT != LIMB_SIZE_MINUS_ONE (i.e. 15)
                                    (begin (will-inc CT 1)
                                           (vanishes (shift OLI 1))))
                        ;; 2.1.5.a)
                        ;; If OLI == 1
                        (will-inc STAMP 1))))
;; 2.1.6)
(defconstraint lastRow (:domain {-1} :guard STAMP)
  (if-zero OLI
           (eq! CT LIMB_SIZE_MINUS_ONE)))

(defconstraint counter-constancies ()
  (begin
   (counter-constancy CT PIVOT)
   (counter-constancy CT BIT_B_4)
   (counter-constancy CT LOW_4)
   (counter-constancy CT NEG)))

(defconstraint stamp-constancies ()
  (begin
   (stamp-constancy STAMP ARG_1_HI)
   (stamp-constancy STAMP ARG_1_LO)
   (stamp-constancy STAMP ARG_2_HI)
   (stamp-constancy STAMP ARG_2_LO)
   (stamp-constancy STAMP RES_HI)
   (stamp-constancy STAMP RES_LO)
   (stamp-constancy STAMP INST)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.2 byte decompositions   ;;
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
   (is-binary OLI)
   (is-binary SMALL)
   (is-binary BITS)
   (is-binary NEG)
   (is-binary BIT_B_4)
   (is-binary BIT_1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.3 target constraints    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint target-constraints ()
  (if-eq CT LIMB_SIZE_MINUS_ONE
         (begin
          (= ACC_1 ARG_1_HI)
          (= ACC_2 ARG_1_LO)
          (= ACC_3 ARG_2_HI)
          (= ACC_4 ARG_2_LO)
          (= ACC_5 RES_HI)
          (= ACC_6 RES_LO))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    2.4 binary column constraints    ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; 2.4.1 OLI constraints
(defconstraint oli-constraints ()
  (begin
   (if-not-zero
    (*  (- INST BYTE)
        (- INST SIGNEXTEND))
    (= OLI 0))
   (if-eq INST BYTE
          (if-zero ARG_1_HI
                   (= OLI 0)
                   (= OLI 1)))
   (if-eq INST SIGNEXTEND
          (if-zero ARG_1_HI
                   (= OLI 0)
                   (= OLI 1)))))

;; 2.4.2 BITS and related columns
(defconstraint bits-and-related ()
  (if-eq CT LIMB_SIZE_MINUS_ONE
         (begin (=  BYTE_2
                    (+ (* 128 (shift BITS -7))
                       (* 64 (shift BITS -6))
                       (* 32 (shift BITS -5))
                       (* 16 (shift BITS -4))
                       (* 8 (shift BITS -3))
                       (* 4 (shift BITS -2))
                       (* 2 (shift BITS -1))
                       BITS))
                (=  LOW_4
                    (+ (* 8 (shift BITS -3))
                       (* 4 (shift BITS -2))
                       (* 2 (shift BITS -1))
                       BITS))
                (=  BIT_B_4
                    (shift BITS -4))
                (=  PIVOT
                    (+ (* 128 (shift BITS -15))
                       (* 64 (shift BITS -14))
                       (* 32 (shift BITS -13))
                       (* 16 (shift BITS -12))
                       (* 8 (shift BITS -11))
                       (* 4 (shift BITS -10))
                       (* 2 (shift BITS -9))
                       (shift BITS -8)))
                (= NEG (shift BITS -15)))))

;; 2.4.3 [[1]] constraints
(defconstraint bit_1 ()
  (begin
   (if-eq INST BYTE
          (plateau-constraint CT BIT_1 LOW_4))
   (if-eq INST SIGNEXTEND
          (plateau-constraint CT BIT_1 (- LIMB_SIZE_MINUS_ONE LOW_4)))))

;; 2.4.4 SMALL constraints
(defconstraint small ()
  (if-eq CT LIMB_SIZE_MINUS_ONE
         (if-zero ARG_1_HI
                  (if-eq-else
                   ARG_1_LO
                   (+
                    (* 16 (shift BITS -4))
                    (* 8 (shift BITS -3))
                    (* 4 (shift BITS -2))
                    (* 2 (shift BITS -1))
                    BITS)
                   (= SMALL 1)
                   (vanishes SMALL)))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.3 pivot constraints    ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (pivot_case_byte)
    (begin (if-zero LOW_4
                    ;; LOW_4 == 0
                    (if-zero CT
                             ;; CT == 0
                             (if-zero BIT_B_4
                                      ;; b4 == 0
                                      (= PIVOT BYTE_3)
                                      ;; b4 == 1
                                      (= PIVOT BYTE_4))))
           (if-not-zero LOW_4
                        ;; LOW_4 != 0
                        (if-zero (shift BIT_1 -1)
                                 ;; [[1]][i-1] == 0
                                 (if-not-zero BIT_1
                                              ;; [[1]][i] == 1
                                              (if-zero BIT_B_4
                                                       ;; b4 == 0
                                                       (= PIVOT BYTE_3)
                                                       ;; b4 == 1
                                                       (= PIVOT BYTE_4)))))))

(defun (pivot_case_signextend)
    (begin
     (if-eq LOW_4 LIMB_SIZE_MINUS_ONE
            ;; LOW_4 == 15
            (if-zero CT
                     (if-zero BIT_B_4
                              ;; b4 == 0
                              (= PIVOT BYTE_4)
                              ;; b4 == 1
                              (= PIVOT BYTE_3))))
     (if-not-zero (- LOW_4 LIMB_SIZE_MINUS_ONE)
                  ;; LOW_4 != 15
                  (if-zero (shift BIT_1 -1)
                           ;; [[1]][i-1] == 0
                           (if-not-zero BIT_1
                                        ;; [[1]][i] == 1
                                        (if-zero BIT_B_4
                                                 ;; b4 == 0
                                                 (= PIVOT BYTE_4)
                                                 ;; b4 == 1
                                                 (= PIVOT BYTE_3)))))))

(defconstraint pivot ()
  (if-not-zero STAMP
               ;; BINARY_STAMP != 0
               (if-zero OLI
                        ;; OLI == 0
                        (begin (if-eq INST BYTE (pivot_case_byte))
                               (if-eq INST SIGNEXTEND (pivot_case_signextend))))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.3 result constraints    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (result_signextend)
                (if-zero SMALL
                    ;; SMALL == 0
                    (begin
                        (= RES_HI ARG_2_HI)
                        (= RES_LO ARG_2_LO))
                    ;; SMALL == 1
                    (begin
                        (if-zero BIT_B_4
                            ;; b4 == 0
                            (begin
                                (= BYTE_5 (* NEG 255))
                                (if-zero BIT_1
                                    ;; [[1]] == 0
                                    (= BYTE_6 (* NEG 255))
                                    ;; [[1]] == 1
                                    (= BYTE_6 BYTE_4)))
                            ;; b4 == 1
                            (begin
                                (if-zero BIT_1
                                    ;; [[1]] == 0
                                    (= BYTE_5 (* NEG 255))
                                    ;; [[1]] == 1
                                    (= BYTE_5 BYTE_3))
                                (= RES_LO ARG_2_LO))))))

(defconstraint result ()
  (if-not-zero STAMP
               (if-not-zero OLI
                            ;; OLI == 1
                            (begin (if-eq INST BYTE
                                          (begin (vanishes RES_HI)
                                                 (vanishes RES_LO)))
                                   (if-eq INST SIGNEXTEND
                                          (begin (= RES_HI ARG_2_HI)
                                                 (= RES_LO ARG_2_LO))))
                            ;; OLI == 0
                            (begin (if-eq INST AND
                                          (begin (= BYTE_5 AND_BYTE_HI)
                                                 (= BYTE_6 AND_BYTE_LO)))
                                   (if-eq INST OR
                                          (begin (= BYTE_5 OR_BYTE_HI)
                                                 (= BYTE_6 OR_BYTE_LO)))
                                   (if-eq INST XOR
                                          (begin (= BYTE_5 XOR_BYTE_HI)
                                                 (= BYTE_6 XOR_BYTE_LO)))
                                   (if-eq INST NOT
                                          (begin (= BYTE_5 NOT_BYTE_HI)
                                                 (= BYTE_6 NOT_BYTE_LO)))
                                   (if-eq INST BYTE
                                          (begin (vanishes RES_HI)
                                                 (= RES_LO (* SMALL PIVOT))))
                                   (if-eq INST SIGNEXTEND
                                          (result_signextend))))))

;; IS_DATA
(defconstraint is_data ()
  (if-zero STAMP
           (vanishes IS_DATA)
           (eq! IS_DATA 1)))
