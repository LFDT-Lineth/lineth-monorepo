(module mxp)

(defconst
  G_MEM                   3 ;; 'G_memory' in Ethereum yellow paper
  SHORTCYCLE              3
  LONGCYCLE               16
  TWO_POW_32              4294967296)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.1 counter constancy    ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; 2.2.1
(defconstraint counter-constancy ()
  (begin
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
   (counter-constancy CT MXPC)
   (counter-constancy CT MXPC_NEW)
   (counter-constancy CT MXP_INST)
   (counter-constancy CT COMP)
   (counter-constancy CT MXPX)
   (counter-constancy CT MXPE)))


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.2 ROOB flag    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;


;; 2.2.1
(defconstraint roob-when-type-1 (:guard [MXP_TYPE 1])
    (vanishes ROOB))

;; 2.2.2
(defconstraint roob-when-type-2-3 (:guard (+ [MXP_TYPE 2] [MXP_TYPE 3]))
  (if-zero OFFSET_1_HI
    (vanishes ROOB)
    (is-not-zero ROOB)))

;; 2.2.3
(defconstraint roob-when-mem-4 (:guard [MXP_TYPE 4])
  (begin
    (if-not-zero SIZE_1_HI (= ROOB 1))
    (if-not-zero (* OFFSET_1_HI SIZE_1_LO)
      (= ROOB 1))
    (if-zero SIZE_1_HI
      (if-zero (* OFFSET_1_HI SIZE_1_LO)
        (vanishes ROOB)))))

;; 2.2.4
(defconstraint roob-when-mem-5 (:guard [MXP_TYPE 5])
  (begin
    (if-not-zero SIZE_1_HI (= ROOB 1))
    (if-not-zero SIZE_2_HI (= ROOB 1))
    (if-not-zero (* OFFSET_1_HI SIZE_1_LO) (= ROOB 1))
    (if-not-zero (* OFFSET_2_HI SIZE_2_LO) (= ROOB 1))
    (if-zero SIZE_1_HI
      (if-zero SIZE_2_HI
        (if-zero (* OFFSET_1_HI SIZE_1_LO)
          (if-zero (* OFFSET_2_HI SIZE_2_LO)
            (vanishes ROOB)))))))


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.3 NOOP flag    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;


;; 2.3.1
(defconstraint noop-and-types (:guard (- 1 ROOB)) 
  (begin 
    (if-not-zero (+ [MXP_TYPE 1] [MXP_TYPE 2] [MXP_TYPE 3])
      (= NOOP [MXP_TYPE 1]))
    (if-eq [MXP_TYPE 4] 1
      (= NOOP (is-zero SIZE_1_LO)))
    (if-eq [MXP_TYPE 5] 1
      (= NOOP (is-zero (+ SIZE_1_LO SIZE_2_LO))))))

;; 2.3.2
(defconstraint noop-consequences (:guard NOOP)
  (begin 
    (vanishes DELTA_MXPC)
    (= WORDS_NEW WORDS)
    (= MXPC_NEW MXPC)))

;; 2.3.3
(defconstraint noop-and-roob ()
  (if-not-zero ROOB
    (vanishes NOOP)))


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.4 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;


;; 2.4.1)
(defconstraint first-row (:domain {0}) (vanishes STAMP))

;; 2.4.2)
(defconstraint stamp-increments ()
  (either (remains-constant STAMP)
               (inc STAMP 1)))

;; 2.4.3)
(defconstraint stamp-is-zero ()
  (if-zero STAMP
    (begin
      (vanishes (+ ROOB NOOP MXPX))
      (vanishes CT)
      (vanishes MXP_INST))))

;; 2.4.4)
(defconstraint only-one-type (:guard STAMP)
  (= 1 (reduce + (for i [5] [MXP_TYPE i]))))

;; 2.4.5)
(defconstraint counter-reset ()
  (if-not-zero (remains-constant STAMP)
    (vanishes (next CT))))

;; 2.4.6)
(defconstraint roob-or-noop ()
  (if-not-zero (+ ROOB NOOP)
    (begin
      (inc STAMP 1)
      (= MXPX ROOB))))

;; 2.4.7
(defconstraint real-instructions ()
  (if-not-zero STAMP
    (if-zero ROOB
      (if-zero NOOP
        (if-zero MXPX
          (if-eq-else CT SHORTCYCLE
            (inc STAMP 1)
            (inc CT 1))
          (if-eq-else CT LONGCYCLE
            (inc STAMP 1)
            (inc CT 1)))))))

;; 2.4.8
(defconstraint dont-terminate-mid-instructions (:domain {-1})
  (if-not-zero STAMP
               (if-zero (force-bool (+ ROOB NOOP))
                        (= CT (if-zero MXPX SHORTCYCLE LONGCYCLE)))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    2.5 Byte decompositions    ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 2.5.1
(defconstraint byte-decompositions ()
  (begin
    (for k [1:4]
      (byte-decomposition CT [ACC k] [BYTE k]))
    (byte-decomposition CT ACC_A BYTE_A)
    (byte-decomposition CT ACC_W BYTE_W)
    (byte-decomposition CT ACC_Q BYTE_Q)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    Specialized constraints    ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (standard-regime)
  (*
    STAMP
    (- 1 (+ NOOP ROOB)))) ;; NOOP + ROOB is binary cf noop section


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.6 Max offsets    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 2.6.1
(defconstraint max-offset-type-2 (:guard (standard-regime))
  (if-eq [MXP_TYPE 2] 1
    (begin
      (= MAX_OFFSET_1 (+ OFFSET_1_LO 31))
      (vanishes MAX_OFFSET_2))))

;; 2.6.2
(defconstraint max-offset-type-3 (:guard (standard-regime))
  (if-eq [MXP_TYPE 3] 1
    (begin
      (= MAX_OFFSET_1 OFFSET_1_LO)
      (vanishes MAX_OFFSET_2))))

;; 2.6.3
(defconstraint max-offset-type-4 (:guard (standard-regime))
  (if-eq [MXP_TYPE 4] 1
    (begin
      (= MAX_OFFSET_1 (+ OFFSET_1_LO (- SIZE_1_LO 1)))
      (vanishes MAX_OFFSET_2))))
  
;; 2.6.4
(defconstraint max-offset-type-5 (:guard (standard-regime))
  (if-eq [MXP_TYPE 5] 1
    (begin
      (if-zero SIZE_1_LO
        (vanishes MAX_OFFSET_1)
        (= MAX_OFFSET_1 (+ OFFSET_1_LO (- SIZE_1_LO 1))))
      (if-zero SIZE_2_LO
        (vanishes MAX_OFFSET_2)
        (= MAX_OFFSET_2 (+ OFFSET_2_LO (- SIZE_2_LO 1)))))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    2.7 Offsets are out of bounds    ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 2.7.1
(defconstraint offsets-out-of-bounds (:guard (standard-regime))
  (if-eq MXPX 1
    (if-eq CT LONGCYCLE
      (vanishes (*
        (- (- MAX_OFFSET_1 TWO_POW_32) [ACC 1])
        (- (- MAX_OFFSET_2 TWO_POW_32) [ACC 2]))))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    2.8 Offsets are in bounds   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (offsets-are-in-bounds)
  (*
    (is-zero (- CT SHORTCYCLE))
    (- 1 MXPX)))

;; 2.8.1
(defconstraint size-in-evm-words (:guard (* (standard-regime) (offsets-are-in-bounds)))
  (if-eq [MXP_TYPE 4] 1
    (begin
     (= SIZE_1_LO (- (* 32 ACC_W) BYTE_R))
     (= (prev BYTE_R) (+ (- 256 32) BYTE_R)))))

;; 2.8.2
(defconstraint offsets-are-small (:guard (* (standard-regime) (offsets-are-in-bounds)))
  (begin 
    (= [ACC 1] MAX_OFFSET_1)
    (= [ACC 2] MAX_OFFSET_2)))

;; 2.8.3
(defconstraint comp-offsets (:guard (* (standard-regime) (offsets-are-in-bounds)))
  (=
    (+ [ACC 3] (- 1 COMP))
    (* (- MAX_OFFSET_1 MAX_OFFSET_2) (- (* 2 COMP) 1))))

;; 2.8.4
(defconstraint define-max-offset (:guard (* (standard-regime) (offsets-are-in-bounds)))
  (= MAX_OFFSET 
    (+ (* COMP MAX_OFFSET_1)
       (* (- 1 COMP) MAX_OFFSET_2))))

;; 2.8.5
(defconstraint define-a (:guard (* (standard-regime) (offsets-are-in-bounds)))
  (begin
    (=
      (+ MAX_OFFSET 1)
      (- (* 32 ACC_A) (shift BYTE_R -2)))
    (=
      (shift BYTE_R -3)
      (+ (- 256 32) (shift BYTE_R -2)))))

;; 2.8.5
(defconstraint mem-expension-took-place (:guard (* (standard-regime) (offsets-are-in-bounds)))
  (=
    (+ [ACC 4] MXPE)
    (* (- ACC_A WORDS) (- (* 2 MXPE) 1))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.9 No expansion event    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint no-extansion (:guard (* (standard-regime) (offsets-are-in-bounds)))
  (if-zero MXPE
    (begin
      (= WORDS_NEW WORDS)
      (= MXPC_NEW MXPC))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;    2.10 Expansion event    ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (expansion-happened)
  (* (offsets-are-in-bounds) MXPE))

;; 2.10.1
(defconstraint update-words (:guard (* (standard-regime) (expansion-happened)))
  (= WORDS_NEW ACC_A))


(defun (q)
  (+ ACC_Q (+
              (* TWO_POW_32 (shift BYTE_QQ -2))
              (* (* 256 TWO_POW_32) (shift BYTE_QQ -3)))))


;; 2.10.2
(defconstraint euclidean-div (:guard (* (standard-regime) (expansion-happened)))
  (begin
    (=
      (* ACC_A ACC_A)
        (+ 
          (* 512 (q))
          (+ (* 256 (prev BYTE_QQ)) BYTE_QQ)))
    (vanishes (* (prev BYTE_QQ) (- 1 (prev BYTE_QQ))))))


;; 2.10.2
(defconstraint define-mxpc-new (:guard (* (standard-regime) (expansion-happened)))
  (= MXPC_NEW (+ (* G_MEM ACC_A) (q))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;    2.11 Expansion gas    ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 2.11.1
(defconstraint expansion-gas (:guard (* (standard-regime) (offsets-are-in-bounds)))
  (= DELTA_MXPC
    (+
      (- MXPC_NEW MXPC) ;; quadratic cost
      (+ (* MXP_GBYTE SIZE_1_LO) (* MXP_GWORD ACC_W))))) ;; linear cost


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;    2.12 Consistency Constraints    ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defpermutation (CN_perm STAMP_perm MXPC_perm MXPC_NEW_perm WORDS_perm WORDS_NEW_perm)
                ((↓ CN) (↓ STAMP) MXPC MXPC_NEW WORDS WORDS_NEW))

;; 2.12.1
(defconstraint consistency ()
  (if-not-zero CN_perm
    (if-eq-else (next CN_perm) CN_perm
      (if-not-zero (remains-constant STAMP_perm)
        (begin
          (= (next WORDS_perm) WORDS_NEW_perm)
          (= (next MXPC_perm) MXPC_NEW_perm)))
      (begin
        (vanishes (next WORDS_perm))
        (vanishes (next MXPC_perm))))))

