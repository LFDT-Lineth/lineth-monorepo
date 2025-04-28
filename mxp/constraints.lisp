(module mxp)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.1 binary constraints   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; done with binary@prove in columns.lisp

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.2 counter constancy    ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint counter-constancy ()
  (begin (counter-constancy CT CN)
         (counter-constancy CT INST)
         (counter-constancy CT DEPLOYS)
         (counter-constancy CT OFFSET_1_LO)
         (counter-constancy CT OFFSET_1_HI)
         (counter-constancy CT OFFSET_2_LO)
         (counter-constancy CT OFFSET_2_HI)
         (counter-constancy CT SIZE_1_LO)
         (counter-constancy CT SIZE_1_HI)
         (counter-constancy CT SIZE_2_LO)
         (counter-constancy CT SIZE_2_HI)
         (counter-constancy CT WORDS)
         (counter-constancy CT WORDS_NEW)
         (counter-constancy CT C_MEM)
         (counter-constancy CT C_MEM_NEW)
         (counter-constancy CT COMP)
         (counter-constancy CT MXPX)
         (counter-constancy CT EXPANDS)
         (counter-constancy CT QUAD_COST)
         (counter-constancy CT LIN_COST)
         (counter-constancy CT GAS_MXP)))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.3 ROOB flag    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint setting-roob-type-1 (:guard [MXP_TYPE 1])
  (vanishes! ROOB))

(defconstraint setting-roob-type-2-3 (:guard (+ [MXP_TYPE 2] [MXP_TYPE 3]))
  (if-not-zero OFFSET_1_HI
               (eq! ROOB 1)
               (vanishes! ROOB)))

(defconstraint setting-roob-type-4 (:guard [MXP_TYPE 4])
  (begin (if-not-zero SIZE_1_HI
                      (eq! ROOB 1))
         (if-not-zero (* OFFSET_1_HI SIZE_1_LO)
                      (eq! ROOB 1))
         (if-zero SIZE_1_HI
                  (if-zero (* OFFSET_1_HI SIZE_1_LO)
                           (vanishes! ROOB)))))

(defconstraint setting-roob-type-5 (:guard [MXP_TYPE 5])
  (begin (if-not-zero SIZE_1_HI
                      (eq! ROOB 1))
         (if-not-zero SIZE_2_HI
                      (eq! ROOB 1))
         (if-not-zero (* OFFSET_1_HI SIZE_1_LO)
                      (eq! ROOB 1))
         (if-not-zero (* OFFSET_2_HI SIZE_2_LO)
                      (eq! ROOB 1))
         (if-zero SIZE_1_HI
                  (if-zero SIZE_2_HI
                           (if-zero (* OFFSET_1_HI SIZE_1_LO)
                                    (if-zero (* OFFSET_2_HI SIZE_2_LO)
                                             (vanishes! ROOB)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.4 NOOP flag    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint noop-automatic-vanishing ()
  (if-not-zero ROOB
               (vanishes! NOOP)))

(defconstraint setting-noop ()
  (if-zero ROOB
           (begin (if-not-zero (+ [MXP_TYPE 1] [MXP_TYPE 2] [MXP_TYPE 3])
                               (eq! NOOP [MXP_TYPE 1]))
                  (if-eq [MXP_TYPE 4] 1
                         (if (eq! SIZE_1_LO 0)
                             (eq! NOOP 1)
                             (eq! NOOP 0)))
                  (if-eq [MXP_TYPE 5] 1
                         (if (and! (eq! SIZE_1_LO 0) (eq! SIZE_2_LO 0))
                             (eq! NOOP 1)
                             (eq! NOOP 0))))))

(defconstraint noop-consequences (:guard NOOP)
  (begin (vanishes! QUAD_COST)
         (vanishes! LIN_COST)
         (eq! WORDS_NEW WORDS)
         (eq! C_MEM_NEW C_MEM)))

;;;;;;;;;;;;;;;;;;;;
;;                ;;
;;    2.5 MTNTOP  ;;
;;                ;;
;;;;;;;;;;;;;;;;;;;;

(defconstraint setting-mtntop ()
  (begin
    (debug (is-binary MTNTOP))
    (debug (if-zero [MXP_TYPE 4]
            (vanishes! MTNTOP)))
    (if-not-zero MXPX
            (vanishes! MTNTOP))
    (if-not-zero [MXP_TYPE 4]
            (if-zero MXPX
              (if-zero SIZE_1_LO
                (vanishes! MTNTOP)
                (eq! MTNTOP 1))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;  2.6 The S1NZNOMXPX and S2NZNOMXPX flags  ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint setting-s1nznomp-s2nznomp ()
  (begin
    (debug (is-binary S1NZNOMXPX))
    (debug (is-binary S2NZNOMXPX))
    (debug (counter-constancy CT S1NZNOMXPX))
    (debug (counter-constancy CT S2NZNOMXPX))
    (if-not-zero MXPX
      (begin (vanishes! S1NZNOMXPX)
             (vanishes! S2NZNOMXPX))
      (begin (if-not-zero SIZE_1_LO
                (eq! S1NZNOMXPX 1))
             (if-not-zero SIZE_1_HI
                (eq! S1NZNOMXPX 1))
             (if-zero SIZE_1_LO
                (if-zero SIZE_1_HI
                  (vanishes! S1NZNOMXPX)))
             (if-not-zero SIZE_2_LO
                (eq! S2NZNOMXPX 1))
             (if-not-zero SIZE_2_HI
                (eq! S2NZNOMXPX 1))
             (if-zero SIZE_2_LO
                (if-zero SIZE_2_HI
                  (vanishes! S2NZNOMXPX)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.7 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint stamp-increments ()
  (or! (will-remain-constant! STAMP) (will-inc! STAMP 1)))

(defconstraint automatic-vanishing-when-padding ()
  (if-zero STAMP
           (begin (vanishes! (+ ROOB NOOP MXPX))
                  (vanishes! CT)
                  (vanishes! INST))))

(defconstraint type-flag-sum (:guard STAMP)
  (eq! 1
       (reduce + (for i [5] [MXP_TYPE i]))))

(defconstraint counter-reset ()
  (if-not (will-remain-constant! STAMP)
          (vanishes! (next CT))))

(defconstraint stamp-increment-when-roob-or-noop ()
  (if-not-zero (+ ROOB NOOP)
               (begin (will-inc! STAMP 1)
                      (eq! MXPX ROOB))))

(defconstraint non-trivial-instruction-counter-cycle ()
  (if-not-zero STAMP
               ;; ROOB + NOOP is binary
               (if-not-zero (- 1 (+ ROOB NOOP))
                            (if-zero MXPX
                                     (if-eq-else CT CT_MAX_NON_TRIVIAL
                                                 (will-inc! STAMP 1)
                                                 (will-inc! CT 1))
                                     (if-eq-else CT CT_MAX_NON_TRIVIAL_BUT_MXPX
                                                 (will-inc! STAMP 1)
                                                 (will-inc! CT 1))))))

(defconstraint final-row (:domain {-1})
  (if-not-zero STAMP
               (if-zero (force-bin (+ ROOB NOOP))
                        (eq! CT (if-zero MXPX
                                      CT_MAX_NON_TRIVIAL
                                      CT_MAX_NON_TRIVIAL_BUT_MXPX)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    2.8 Byte decompositions    ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint byte-decompositions ()
  (begin (for k [1:4] (byte-decomposition CT [ACC k] [BYTE k]))
         (byte-decomposition CT ACC_A BYTE_A)
         (byte-decomposition CT ACC_W BYTE_W)
         (byte-decomposition CT ACC_Q BYTE_Q)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    Specialized constraints    ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (standing-hypothesis)
  (* STAMP (- 1 NOOP ROOB))) ;; NOOP + ROOB is binary cf noop section

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    3.1 Max offsets    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint max-offsets-1-and-2-type-2 (:guard (standing-hypothesis))
  (if-eq [MXP_TYPE 2] 1
         (begin (eq! MAX_OFFSET_1 (+ OFFSET_1_LO 31))
                (vanishes! MAX_OFFSET_2))))

(defconstraint max-offsets-1-and-2-type-3 (:guard (standing-hypothesis))
  (if-eq [MXP_TYPE 3] 1
         (begin (eq! MAX_OFFSET_1 OFFSET_1_LO)
                (vanishes! MAX_OFFSET_2))))

(defconstraint max-offsets-1-and-2-type-4 (:guard (standing-hypothesis))
  (if-eq [MXP_TYPE 4] 1
         (begin (eq! MAX_OFFSET_1
                     (+ OFFSET_1_LO (- SIZE_1_LO 1)))
                (vanishes! MAX_OFFSET_2))))

(defconstraint max-offsets-1-and-2-type-5 (:guard (standing-hypothesis))
  (if-eq [MXP_TYPE 5] 1
         (begin (if-zero SIZE_1_LO
                         (vanishes! MAX_OFFSET_1)
                         (eq! MAX_OFFSET_1
                              (+ OFFSET_1_LO (- SIZE_1_LO 1))))
                (if-zero SIZE_2_LO
                         (vanishes! MAX_OFFSET_2)
                         (eq! MAX_OFFSET_2
                              (+ OFFSET_2_LO (- SIZE_2_LO 1)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    3.2 Offsets are out of bounds    ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint offsets-out-of-bounds (:guard (standing-hypothesis))
  (if-eq MXPX 1
         (if-eq CT CT_MAX_NON_TRIVIAL_BUT_MXPX
                (or! (eq! (- MAX_OFFSET_1 TWO_POW_32) [ACC 1])
                     (eq! (- MAX_OFFSET_2 TWO_POW_32) [ACC 2])))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    3.3 Offsets are in bounds   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (offsets-are-in-bounds)
  (if (eq! CT CT_MAX_NON_TRIVIAL)
      (- 1 MXPX) 0))

(defconstraint size-in-evm-words (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (if-eq [MXP_TYPE 4] 1
         (begin (eq! SIZE_1_LO
                     (- (* 32 ACC_W) BYTE_R))
                (eq! (prev BYTE_R)
                     (+ (- 256 32) BYTE_R)))))

(defconstraint max-offsets-1-and-2-are-small (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (begin (eq! [ACC 1] MAX_OFFSET_1)
         (eq! [ACC 2] MAX_OFFSET_2)))

(defconstraint comparing-max-offsets-1-and-2 (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (eq! (+ [ACC 3] (- 1 COMP))
       (* (- MAX_OFFSET_1 MAX_OFFSET_2)
          (- (* 2 COMP) 1))))

(defconstraint defining-max-offset (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (eq! MAX_OFFSET
       (+ (* COMP MAX_OFFSET_1)
          (* (- 1 COMP) MAX_OFFSET_2))))

(defconstraint defining-accA (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (begin (eq! (+ MAX_OFFSET 1)
              (- (* 32 ACC_A) (shift BYTE_R -2)))
         (eq! (shift BYTE_R -3)
              (+ (- 256 32) (shift BYTE_R -2)))))

(defconstraint mem-expansion-took-place (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (eq! (+ [ACC 4] EXPANDS)
       (* (- ACC_A WORDS)
          (- (* 2 EXPANDS) 1))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    3.4.2 No expansion event  ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint no-expansion (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (if-zero EXPANDS
           (begin (eq! WORDS_NEW WORDS)
                  (eq! C_MEM_NEW C_MEM))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;    3.4.3 Expansion event   ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (expansion-happened)
  (* (offsets-are-in-bounds) EXPANDS))

(defconstraint setting-words-new (:guard (* (standing-hypothesis) (expansion-happened)))
  (eq! WORDS_NEW ACC_A))

(defun (large-quotient)
  (+ ACC_Q
     (+ (* TWO_POW_32 (shift BYTE_QQ -2))
        (* (* 256 TWO_POW_32) (shift BYTE_QQ -3)))))

(defconstraint euclidean-division-of-square-of-accA (:guard (* (standing-hypothesis) (expansion-happened)))
  (begin (eq! (* ACC_A ACC_A)
              (+ (* 512 (large-quotient))
                 (+ (* 256 (prev BYTE_QQ))
                    BYTE_QQ)))
         (or! (eq! 0 (prev BYTE_QQ))
              (eq! 1 (prev BYTE_QQ)))))

(defconstraint setting-c-mem-new (:guard (* (standing-hypothesis) (expansion-happened)))
  (eq! C_MEM_NEW
       (+ (* GAS_CONST_G_MEMORY ACC_A) (large-quotient))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;    3.4.4 Expansion gas   ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint setting-quad-cost-and-lin-cost (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (begin (eq! QUAD_COST (- C_MEM_NEW C_MEM))
         (eq! LIN_COST
              (+ (* GBYTE SIZE_1_LO) (* GWORD ACC_W)))))

(defconstraint setting-gas-mxp (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (if-eq-else INST EVM_INST_RETURN
      (eq! GAS_MXP
           (+ QUAD_COST (* DEPLOYS LIN_COST)))
      (eq! GAS_MXP (+ QUAD_COST LIN_COST))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;    2.12 Consistency Constraints    ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defpermutation
  (CN_perm
   STAMP_perm
   C_MEM_perm
   C_MEM_NEW_perm
   WORDS_perm
   WORDS_NEW_perm)

  ((↓ CN)
   (↓ STAMP)
   C_MEM
   C_MEM_NEW
   WORDS
   WORDS_NEW)
  )

(defconstraint consistency ()
  (if-not-zero CN_perm
               (if-eq-else (prev CN_perm) CN_perm
                           (if-not-zero (- (prev STAMP_perm) STAMP_perm)
                                        (begin (eq! WORDS_perm (prev WORDS_NEW_perm))
                                               (eq! C_MEM_perm (prev C_MEM_NEW_perm))))
                           (begin (vanishes! WORDS_perm)
                                  (vanishes! C_MEM_perm)))))
