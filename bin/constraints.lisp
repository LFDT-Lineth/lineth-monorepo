(module bin)

(defpurefun (if-eq-else A B THEN ELSE)
  (if-zero (- A B)
           THEN
           ELSE))

;; 2.2  Shorthands
(defun (flag-sum)
  (+ IS_AND IS_OR IS_XOR IS_NOT IS_BYTE IS_SIGNEXTEND))

(defun (weight-sum)
  (+ (* IS_AND EVM_INST_AND)
     (* IS_OR EVM_INST_OR)
     (* IS_XOR EVM_INST_XOR)
     (* IS_NOT EVM_INST_NOT)
     (* IS_BYTE EVM_INST_BYTE)
     (* IS_SIGNEXTEND EVM_INST_SIGNEXTEND)))

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
  (or! (will-inc! STAMP 0) (will-inc! STAMP 1)))

(defconstraint new-stamp-reset-ct ()
  (if-not-zero (- (next STAMP) STAMP)
               (vanishes! (next CT))))

(defconstraint isnot-ctmax ()
  (if-eq IS_NOT 1 (eq! CT_MAX LLARGEMO)))

(defconstraint isbyte-ctmax ()
  (if-eq (+ IS_BYTE IS_SIGNEXTEND) 1
         (if-zero ARG_1_HI
                  (eq! CT_MAX LLARGEMO)
                  (vanishes! CT_MAX))))

(defconstraint ct-small ()
  (eq! 1
       (~ (- CT LLARGE))))

(defconstraint countereset (:guard STAMP)
  (if-eq-else CT CT_MAX (will-inc! STAMP 1) (will-inc! CT 1)))

(defconstraint last-row (:domain {-1})
  (eq! CT CT_MAX))

(defconstraint counter-constancies ()
  (begin (counter-constancy CT ARG_1_HI)
         (counter-constancy CT ARG_1_LO)
         (counter-constancy CT ARG_2_HI)
         (counter-constancy CT ARG_2_LO)
         (counter-constancy CT RES_HI)
         (counter-constancy CT RES_LO)
         (counter-constancy CT INST)
         (counter-constancy CT CT_MAX)
         (counter-constancy CT PIVOT)
         (counter-constancy CT BIT_B_4)
         (counter-constancy CT LOW_4)
         (counter-constancy CT NEG)
         (counter-constancy CT SMALL)))

;;    2.6 byte decompositions
(defconstraint byte_decompositions ()
  (begin (byte-decomposition CT ACC_1 BYTE_1)
         (byte-decomposition CT ACC_2 BYTE_2)
         (byte-decomposition CT ACC_3 BYTE_3)
         (byte-decomposition CT ACC_4 BYTE_4)
         (byte-decomposition CT ACC_5 BYTE_5)
         (byte-decomposition CT ACC_6 BYTE_6)))

;;    2.7 target constraints
(defun (requires-byte-decomposition)
  (+ IS_AND
     IS_OR
     IS_XOR
     IS_NOT
     (* CT_MAX (+ IS_BYTE IS_SIGNEXTEND))))

(defconstraint target-constraints (:guard (requires-byte-decomposition))
  (if-eq CT CT_MAX
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
;; 2.8.2 BITS and related columns
(defconstraint bits-and-related (:guard (+ IS_BYTE IS_SIGNEXTEND))
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
(defconstraint bit_1 (:guard CT_MAX)
  (begin (if-eq IS_BYTE 1 (plateau-constraint CT BIT_1 LOW_4))
         (if-eq IS_SIGNEXTEND 1
                (plateau-constraint CT BIT_1 (- LLARGEMO LOW_4)))))

;; 2.8.4 SMALL constraints
(defconstraint small (:guard (+ IS_BYTE IS_SIGNEXTEND))
  (if-eq CT LLARGEMO
         (if-eq-else ARG_1_LO (+ (* 16 (shift BITS -4))
                        (* 8 (shift BITS -3))
                        (* 4 (shift BITS -2))
                        (* 2 (shift BITS -1))
                        BITS)
                     (eq! SMALL 1)
                     (vanishes! SMALL))))

;;    2.9 pivot constraints
(defconstraint pivot (:guard CT_MAX)
  (begin (if-eq IS_BYTE 1
                (if-zero LOW_4
                         (if-zero CT
                                  (if-zero BIT_B_4
                                           (eq! PIVOT BYTE_3)
                                           (eq! PIVOT BYTE_4)))
                         (if-zero (+ (prev BIT_1) (- 1 BIT_1))
                                  (if-zero BIT_B_4
                                           (eq! PIVOT BYTE_3)
                                           (eq! PIVOT BYTE_4)))))
         (if-eq IS_SIGNEXTEND 1
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
  (if-zero CT_MAX
           (begin (vanishes! RES_HI)
                  (vanishes! RES_LO))
           (begin (vanishes! RES_HI)
                  (eq! RES_LO (* SMALL PIVOT)))))

(defconstraint is-signextend-result (:guard IS_SIGNEXTEND)
  (if-zero CT_MAX
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


