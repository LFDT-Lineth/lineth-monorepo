(module mxp)

(defconst 
  G_MEM      3 ;; 'G_memory' in Ethereum yellow paper
  SHORTCYCLE 3
  LONGCYCLE  16
  TWO_POW_32 4294967296
  RETURN 0xf3)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.1 binary constraints   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint binary-constraints ()
  (begin (is-binary ROOB)
         (is-binary NOOP)
         (is-binary MXPX)
         (is-binary DEPLOYS)
         (is-binary COMP)
         (is-binary EXPANDS)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.2 counter constancy    ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint counter-constancy ()
  (begin (counter-constancy CT INST)
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
(defconstraint roob-when-type-1 (:guard [MXP_TYPE 1])
  (vanishes! ROOB))

(defconstraint roob-when-type-2-3 (:guard (+ [MXP_TYPE 2] [MXP_TYPE 3]))
  (if-not-zero OFFSET_1_HI
               (= ROOB 1)
               (vanishes! ROOB)))

(defconstraint roob-when-mem-4 (:guard [MXP_TYPE 4])
  (begin (if-not-zero SIZE_1_HI
                      (= ROOB 1))
         (if-not-zero (* OFFSET_1_HI SIZE_1_LO)
                      (= ROOB 1))
         (if-zero SIZE_1_HI
                  (if-zero (* OFFSET_1_HI SIZE_1_LO)
                           (vanishes! ROOB)))))

(defconstraint roob-when-mem-5 (:guard [MXP_TYPE 5])
  (begin (if-not-zero SIZE_1_HI
                      (= ROOB 1))
         (if-not-zero SIZE_2_HI
                      (= ROOB 1))
         (if-not-zero (* OFFSET_1_HI SIZE_1_LO)
                      (= ROOB 1))
         (if-not-zero (* OFFSET_2_HI SIZE_2_LO)
                      (= ROOB 1))
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
(defconstraint noop-and-roob ()
  (if-not-zero ROOB
               (vanishes! NOOP)))

(defconstraint noop-and-types ()
  (if-zero ROOB
           (begin (if-not-zero (+ [MXP_TYPE 1] [MXP_TYPE 2] [MXP_TYPE 3])
                               (= NOOP [MXP_TYPE 1]))
                  (if-eq [MXP_TYPE 4] 1
                         (= NOOP (is-zero SIZE_1_LO)))
                  (if-eq [MXP_TYPE 5] 1
                         (= NOOP
                            (* (is-zero SIZE_1_LO) (is-zero SIZE_2_LO)))))))

(defconstraint noop-consequences (:guard NOOP)
  (begin (vanishes! QUAD_COST)
         (vanishes! LIN_COST)
         (= WORDS_NEW WORDS)
         (= C_MEM_NEW C_MEM)))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.5 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint stamp-increments ()
  (any! (will-remain-constant! STAMP) (will-inc! STAMP 1)))

(defconstraint stamp-is-zero ()
  (if-zero STAMP
           (begin (vanishes! (+ ROOB NOOP MXPX))
                  (vanishes! CT)
                  (vanishes! INST))))

(defconstraint only-one-type (:guard STAMP)
  (= 1
     (reduce + (for i [5] [MXP_TYPE i]))))

(defconstraint counter-reset ()
  (if-not-zero (will-remain-constant! STAMP)
               (vanishes! (next CT))))

(defconstraint roob-or-noop ()
  (if-not-zero (+ ROOB NOOP)
               (begin (will-inc! STAMP 1)
                      (= MXPX ROOB))))

(defconstraint real-instructions ()
  (if-not-zero STAMP
               (if-not-zero (+ ROOB NOOP)
                                 (if-zero MXPX
                                          (if-eq-else CT SHORTCYCLE
                                                      (will-inc! STAMP 1)
                                                      (will-inc! CT 1))
                                          (if-eq-else CT LONGCYCLE
                                                      (will-inc! STAMP 1)
                                                      (will-inc! CT 1))))))

(defconstraint dont-terminate-mid-instructions (:domain {-1})
  (if-not-zero STAMP
               (if-zero (force-bool (+ ROOB NOOP))
                        (= CT (if-zero MXPX
                                    SHORTCYCLE
                                    LONGCYCLE)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    2.5 Byte decompositions    ;;
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
  (* STAMP
     (- 1 NOOP ROOB))) ;; NOOP + ROOB is binary cf noop section

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    3.1 Max offsets    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint max-offset-type-2 (:guard (standing-hypothesis))
  (if-eq [MXP_TYPE 2] 1
         (begin (= MAX_OFFSET_1 (+ OFFSET_1_LO 31))
                (vanishes! MAX_OFFSET_2))))

(defconstraint max-offset-type-3 (:guard (standing-hypothesis))
  (if-eq [MXP_TYPE 3] 1
         (begin (= MAX_OFFSET_1 OFFSET_1_LO)
                (vanishes! MAX_OFFSET_2))))

(defconstraint max-offset-type-4 (:guard (standing-hypothesis))
  (if-eq [MXP_TYPE 4] 1
         (begin (= MAX_OFFSET_1
                   (+ OFFSET_1_LO (- SIZE_1_LO 1)))
                (vanishes! MAX_OFFSET_2))))

(defconstraint max-offset-type-5 (:guard (standing-hypothesis))
  (if-eq [MXP_TYPE 5] 1
         (begin (if-zero SIZE_1_LO
                         (vanishes! MAX_OFFSET_1)
                         (= MAX_OFFSET_1
                            (+ OFFSET_1_LO (- SIZE_1_LO 1))))
                (if-zero SIZE_2_LO
                         (vanishes! MAX_OFFSET_2)
                         (= MAX_OFFSET_2
                            (+ OFFSET_2_LO (- SIZE_2_LO 1)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    3.2 Offsets are out of bounds    ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint offsets-out-of-bounds (:guard (standing-hypothesis))
  (if-eq MXPX 1
         (if-eq CT LONGCYCLE
                (vanishes! (* (- (- MAX_OFFSET_1 TWO_POW_32) [ACC 1])
                              (- (- MAX_OFFSET_2 TWO_POW_32) [ACC 2]))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    3.3 Offsets are in bounds   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (offsets-are-in-bounds)
  (* (is-zero (- CT SHORTCYCLE))
     (- 1 MXPX)))

(defconstraint size-in-evm-words (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (if-eq [MXP_TYPE 4] 1
         (begin (= SIZE_1_LO
                   (- (* 32 ACC_W) BYTE_R))
                (= (prev BYTE_R)
                   (+ (- 256 32) BYTE_R)))))

(defconstraint offsets-are-small (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (begin (= [ACC 1] MAX_OFFSET_1)
         (= [ACC 2] MAX_OFFSET_2)))

(defconstraint comp-offsets (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (= (+ [ACC 3] (- 1 COMP))
     (* (- MAX_OFFSET_1 MAX_OFFSET_2)
        (- (* 2 COMP) 1))))

(defconstraint define-max-offset (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (= MAX_OFFSET
     (+ (* COMP MAX_OFFSET_1)
        (* (- 1 COMP) MAX_OFFSET_2))))

(defconstraint define-a (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (begin (= (+ MAX_OFFSET 1)
            (- (* 32 ACC_A) (shift BYTE_R -2)))
         (= (shift BYTE_R -3)
            (+ (- 256 32) (shift BYTE_R -2)))))

(defconstraint mem-expension-took-place (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (= (+ [ACC 4] EXPANDS)
     (* (- ACC_A WORDS)
        (- (* 2 EXPANDS) 1))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    3.4.2 No expansion event  ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint no-extansion (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (if-zero EXPANDS
           (begin (= WORDS_NEW WORDS)
                  (= C_MEM_NEW C_MEM))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;    3.4.3 Expansion event   ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (expansion-happened)
  (* (offsets-are-in-bounds) EXPANDS))

(defconstraint update-words (:guard (* (standing-hypothesis) (expansion-happened)))
  (= WORDS_NEW ACC_A))

(defun (q)
  (+ ACC_Q
     (+ (* TWO_POW_32 (shift BYTE_QQ -2))
        (* (* 256 TWO_POW_32) (shift BYTE_QQ -3)))))

(defconstraint euclidean-div (:guard (* (standing-hypothesis) (expansion-happened)))
  (begin (= (* ACC_A ACC_A)
            (+ (* 512 (q))
               (+ (* 256 (prev BYTE_QQ))
                  BYTE_QQ)))
         (vanishes! (* (prev BYTE_QQ)
                       (- 1 (prev BYTE_QQ))))))

(defconstraint define-mxpc-new (:guard (* (standing-hypothesis) (expansion-happened)))
  (= C_MEM_NEW
     (+ (* G_MEM ACC_A) (q))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;    3.4.4 Expansion gas   ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint expansion-gas (:guard (* (standing-hypothesis) (offsets-are-in-bounds)))
  (begin
    (= QUAD_COST (- C_MEM_NEW C_MEM))
    (= LIN_COST (+ (* GBYTE SIZE_1_LO)(* GWORD ACC_W)))
    (if (eq INST RETURN) 
      (= GAS_MXP (+ QUAD_COST (* DEPLOYS LIN_COST)))
      (= GAS_MXP (+ QUAD_COST LIN_COST)))
  )
)

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
               (if-eq-else (next CN_perm) CN_perm
                           (if-not-zero (will-remain-constant! STAMP_perm)
                                        (begin (= (next WORDS_perm) WORDS_NEW_perm)
                                               (= (next C_MEM_perm) C_MEM_NEW_perm)))
                           (begin (vanishes! (next WORDS_perm))
                                  (vanishes! (next C_MEM_perm))))))


