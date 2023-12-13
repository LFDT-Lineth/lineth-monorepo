(module bin)

(defconst 
  ;; opcode values
  SIGNEXTEND 11
  AND        22
  OR         23
  XOR        24
  NOT        25
  BYTE       26
  ;; constant values
  LLARGE     16
  LLARGEMO   15)

(defpurefun (if-eq-else A B THEN ELSE)
  (if-zero (- A B)
           THEN
           ELSE))

;;   2.1 binary constraints   
;; binary constraints
(defconstraint binary_constraints ()
  (begin (is-binary IS_AND)
         (is-binary IS_OR)
         (is-binary IS_XOR)
         (is-binary IS_NOT)
         (is-binary IS_BYTE)
         (is-binary IS_SIGNEXTEND)
         (is-binary SMALL)
         (is-binary BITS)
         (is-binary NEG)
         (is-binary BIT_B_4)
         (is-binary BIT_1)))

;; 2.2  Shorthands
(defun (flag-sum)
  (+ IS_AND IS_OR IS_XOR IS_NOT IS_BYTE IS_SIGNEXTEND))

(defun (weight-sum)
  (+ (* IS_AND AND)
     (* IS_OR OR)
     (* IS_XOR XOR)
     (* IS_NOT NOT)
     (* IS_BYTE BYTE)
     (* IS_SIGNEXTEND SIGNEXTEND)))

;; 2.3 Instruction decoding
(defconstraint no-bin-no-flag ()
  (if-zero STAMP
           (vanishes! (flag-sum))
           (eq! (flag-sum) 1)))

(defconstraint inst-to-flag ()
  (eq! INST (weight-sum)))

;; 2.4 Heartbeat
(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint stamp-increments ()
  (vanishes! (* (will-inc! STAMP 0) (will-inc! STAMP 1))))

(defconstraint countereset ()
  (if-not-zero (will-remain-constant! STAMP)
               (vanishes! (next CT))))

(defconstraint oli-incrementation (:guard OLI)
  (will-inc! STAMP 1))

(defconstraint mli-incrementation (:guard MLI)
  (if-eq-else CT LLARGEMO (will-inc! STAMP 1) (will-inc! CT 1)))

(defconstraint last-row (:domain {-1})
  (if-eq MLI 1 (eq! CT LLARGEMO)))

(defconstraint counter-constancies ()
  (begin (counter-constancy CT ARG_1_HI)
         (counter-constancy CT ARG_1_LO)
         (counter-constancy CT ARG_2_HI)
         (counter-constancy CT ARG_2_LO)
         (counter-constancy CT RES_HI)
         (counter-constancy CT RES_LO)
         (counter-constancy CT INST)
         (counter-constancy CT PIVOT)
         (counter-constancy CT BIT_B_4)
         (counter-constancy CT LOW_4)
         (counter-constancy CT NEG)))

;;    2.6 byte decompositions   
(defconstraint byte_decompositions ()
  (begin (byte-decomposition CT ACC_1 BYTE_1)
         (byte-decomposition CT ACC_2 BYTE_2)
         (byte-decomposition CT ACC_3 BYTE_3)
         (byte-decomposition CT ACC_4 BYTE_4)
         (byte-decomposition CT ACC_5 BYTE_5)
         (byte-decomposition CT ACC_6 BYTE_6)))

;;    2.7 target constraints    
(defconstraint target-constraints ()
  (if-eq CT LLARGEMO
         (begin (eq! ACC_1 ARG_1_HI)
                (eq! ACC_2 ARG_1_LO)
                (eq! ACC_3 ARG_2_HI)
                (eq! ACC_4 ARG_2_LO)
                (eq! ACC_5 RES_HI)
                (eq! ACC_6 RES_LO))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    2.8 binary column constraints    ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-oli-mli ()
  (if-zero (+ IS_BYTE IS_SIGNEXTEND)
           (vanishes! OLI)
           (if-zero ARG_1_HI
                    (vanishes! OLI)
                    (eq! OLI 1))))

(defconstraint oli-mli-exclusivity ()
  (eq! (+ OLI MLI) (flag-sum)))

;; 2.8.2 BITS and related columns
(defconstraint bits-and-related ()
  (if-eq CT LLARGEMO
         (begin (eq! PIVOT
                     (+ (* 128 (shift BITS -15))
                        (* 64 (shift BITS -14))
                        (* 32 (shift BITS -13))
                        (* 16 (shift BITS -12))
                        (* 8 (shift BITS -11))
                        (* 4 (shift BITS -10))
                        (* 2 (shift BITS -9))
                        (shift BITS -8)))
                (eq! BYTE_2
                     (+ (* 128 (shift BITS -7))
                        (* 64 (shift BITS -6))
                        (* 32 (shift BITS -5))
                        (* 16 (shift BITS -4))
                        (* 8 (shift BITS -3))
                        (* 4 (shift BITS -2))
                        (* 2 (shift BITS -1))
                        BITS))
                (eq! LOW_4
                     (+ (* 8 (shift BITS -3))
                        (* 4 (shift BITS -2))
                        (* 2 (shift BITS -1))
                        BITS))
                (eq! BIT_B_4 (shift BITS -4))
                (eq! NEG (shift BITS -15)))))

;; 2.8.3 [[1]] constraints
(defconstraint bit_1 ()
  (begin (if-eq IS_BYTE 1 (plateau-constraint CT BIT_1 LOW_4))
         (if-eq IS_SIGNEXTEND 1
                (plateau-constraint CT BIT_1 (- LLARGEMO LOW_4)))))

;; 2.8.4 SMALL constraints
(defconstraint small ()
  (if-eq CT LLARGEMO
         (if-zero ARG_1_HI
                  (if-eq-else ARG_1_LO (+ (* 16 (shift BITS -4))
                                 (* 8 (shift BITS -3))
                                 (* 4 (shift BITS -2))
                                 (* 2 (shift BITS -1))
                                 BITS)
                              (eq! SMALL 1)
                              (vanishes! SMALL)))))

;;    2.9 pivot constraints    
(defconstraint pivot (:guard MLI)
  (begin (if-not-zero IS_BYTE
                      (if-zero LOW_4
                               (if-zero CT
                                        (if-zero BIT_B_4
                                                 (eq! PIVOT BYTE_3)
                                                 (eq! PIVOT BYTE_4)))
                               (if-zero (+ (prev BIT_1) (- 1 BIT_1))
                                        (if-zero BIT_B_4
                                                 (eq! PIVOT BYTE_3)
                                                 (eq! PIVOT BYTE_4)))))
         (if-not-zero IS_SIGNEXTEND
                      (if-eq-else LOW_4 LLARGEMO
                                  (if-zero CT
                                           (if-zero BIT_B_4
                                                    (eq! PIVOT BYTE_4)
                                                    (eq! PIVOT BYTE_3)))
                                  (if-zero (+ (prev BIT_1) (- 1 BIT_1))
                                           (if-zero BIT_B_4
                                                    (eq! PIVOT BYTE_4)
                                                    (eq! PIVOT BYTE_3)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.10 result constraints   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint is-byte-result (:guard IS_BYTE)
  (if-eq-else OLI 1
              (begin (vanishes! RES_HI)
                     (vanishes! RES_LO))
              (begin (vanishes! RES_HI)
                     (eq! RES_LO (* SMALL PIVOT)))))

(defconstraint is-signextend-result (:guard IS_SIGNEXTEND)
  (if-eq-else OLI 1
              (begin (eq! RES_HI ARG_2_HI)
                     (eq! RES_LO ARG_2_LO))
              (if-zero SMALL
                       ;; SMALL == 0
                       (begin (eq! RES_HI ARG_2_HI)
                              (eq! RES_LO ARG_2_LO))
                       ;; SMALL == 1
                       (begin (if-zero BIT_B_4
                                       ;; b4 == 0
                                       (begin (eq! BYTE_5 (* NEG 255))
                                              (if-zero BIT_1
                                                       ;; [[1]] == 0
                                                       (eq! BYTE_6 (* NEG 255))
                                                       ;; [[1]] == 1
                                                       (eq! BYTE_6 BYTE_4)))
                                       ;; b4 == 1
                                       (begin (if-zero BIT_1
                                                       ;; [[1]] == 0
                                                       (eq! BYTE_5 (* NEG 255))
                                                       ;; [[1]] == 1
                                                       (eq! BYTE_5 BYTE_3))
                                              (eq! RES_LO ARG_2_LO)))))))

(defconstraint result-via-lookup (:guard (+ IS_AND IS_OR IS_XOR IS_NOT))
  (begin (eq! BYTE_5 XXX_BYTE_HI)
         (eq! BYTE_6 XXX_BYTE_LO)))


