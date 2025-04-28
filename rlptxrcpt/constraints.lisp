(module rlptxrcpt)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    4.1 Global Constraints    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;    4.1.1 Constancy columns  ;;
;; Def block-constant
(defun (block-constant C)
  (if-not-zero ABS_TX_NUM
               (will-remain-constant! C)))

;; Def counter-constant
(defun (counter-constant C)
  (if-not-zero CT
               (remained-constant! C)))

;; Def counter-incrementing.
(defun (counter-incrementing C)
  (if-not-zero CT
               (or! (remained-constant! C) (did-inc! C 1))))

;; Definition phase-constancy.
(defpurefun (phase-constancy PHASE C)
  (if-eq (* PHASE (prev PHASE)) 1
         (remained-constant! C)))

;; Definition phase-incrementing
(defpurefun (phase-incrementing PHASE C)
  (if-eq (* PHASE (prev PHASE)) 1
         (or! (remained-constant! C) (did-inc! C 1))))

;; Definition phase-decrementing
(defpurefun (phase-decrementing PHASE C)
  (if-eq (* PHASE (prev PHASE)) 1
         (or! (remained-constant! C) (did-dec! C 1))))

;; Constancies
(defconstraint block-constancies ()
  (begin (block-constant ABS_TX_NUM_MAX)
         (block-constant ABS_LOG_NUM_MAX)))

(defconstraint counter-constancies ()
  (begin (for i [2 : 4] (counter-constant [INPUT i]))
         (counter-constant nSTEP)
         (counter-constant IS_PREFIX)
         (counter-constant DEPTH_1)
         (counter-constant IS_TOPIC)
         (counter-constant IS_DATA)))

(defconstraint special-ct-constancy ()
  (if-not-zero (+ (- 1 [PHASE 5]) (- 1 DEPTH_1) IS_PREFIX (- 1 IS_DATA))
               (counter-constant [INPUT 1])))

(defconstraint ct-incrementings ()
  (begin (if-zero (* IS_DATA IS_PREFIX)
                  (counter-incrementing LC))
         (counter-incrementing LC_CORRECTION)))

(defconstraint phase4-decrementing ()
  (phase-decrementing [PHASE 4] IS_PREFIX))

(defconstraint phase5-incrementing ()
  (phase-incrementing [PHASE 5] DEPTH_1))

(defconstraint istopic-incrementing ()
  (phase-incrementing IS_TOPIC INDEX_LOCAL))

(defconstraint phase1-constant ()
  (phase-constancy [PHASE 1] TXRCPT_SIZE))

;;    4.1.2 Global Phase Constraints    ;;
(defconstraint impose-phase-id ()
  (eq! PHASE_ID
       (+ (reduce +
                  (for k [1 : 5] (* k [PHASE k])))
          (* SUBPHASE_ID_WEIGHT_IS_PREFIX IS_PREFIX)
          (* SUBPHASE_ID_WEIGHT_IS_OT IS_TOPIC)
          (* SUBPHASE_ID_WEIGHT_IS_OD IS_DATA)
          (* SUBPHASE_ID_WEIGHT_DEPTH DEPTH_1)
          (* SUBPHASE_ID_WEIGHT_INDEX_LOCAL IS_TOPIC INDEX_LOCAL))))

(defconstraint initial-stamp (:domain {0})
  (begin (vanishes! ABS_TX_NUM)
         (vanishes! ABS_LOG_NUM)))

(defconstraint phase-exclusion ()
  (if-zero ABS_TX_NUM
           (vanishes! (reduce + (for i [5] [PHASE i])))
           (eq! 1
                (reduce + (for i [5] [PHASE i])))))

(defconstraint ABS_TX_NUM-evolution ()
  (if (or! (eq! [PHASE 1] 0) (remained-constant! [PHASE 1]))
           ;; no change
           (remained-constant! ABS_TX_NUM)
           ;; increment
           (did-inc! ABS_TX_NUM 1)))

(defconstraint ABS_LOG_NUM-evolution ()
  (if-zero (+ (- 1 [PHASE 5]) (- 1 DEPTH_1) (- 1 IS_PREFIX) IS_TOPIC IS_DATA CT)
           (did-inc! ABS_LOG_NUM 1)
           (remained-constant! ABS_LOG_NUM)))

(defconstraint no-done-no-end ()
  (if-zero DONE
           (vanishes! PHASE_END)))

(defconstraint still-size-no-end ()
  (if-not-zero PHASE_SIZE
               (vanishes! PHASE_END)))

(defconstraint no-end-no-changephase (:guard ABS_TX_NUM)
  (if-zero PHASE_END
           (eq! (reduce +
                        (for i [5] (* i [PHASE i])))
                (reduce +
                        (for i
                             [5]
                             (* i (next [PHASE i])))))))

(defconstraint phase-transition ()
  (if-eq PHASE_END 1
         (begin (eq! 1
                     (+ (* [PHASE 1] (next [PHASE 2]))
                        (* [PHASE 2] (next [PHASE 3]))
                        (* [PHASE 3] (next [PHASE 4]))
                        (* [PHASE 4] (next [PHASE 5]))
                        (* [PHASE 5] (next [PHASE 1]))))
                (if-eq [PHASE 5] 1 (vanishes! TXRCPT_SIZE)))))

;;    4.1.3 Byte decomposition's loop heartbeat  ;;
(defconstraint ct-imply-done (:guard ABS_TX_NUM)
  (if-eq-else CT (- nSTEP 1) (eq! DONE 1) (vanishes! DONE)))

(defconstraint done-imply-heartbeat (:guard ABS_TX_NUM)
  (if-zero DONE
           (will-inc! CT 1)
           (begin (eq! LC (- 1 LC_CORRECTION))
                  (vanishes! (next CT)))))

;;    4.1.4 Blind Byte and Bit decomposition  ;;
(defconstraint byte-decompositions ()
  (for k [1:4] (byte-decomposition CT [ACC k] [BYTE k])))

(defconstraint bit-decomposition ()
  (if-zero CT
           (eq! BIT_ACC BIT)
           (eq! BIT_ACC
                (+ (* 2 (prev BIT_ACC))
                   BIT))))

;;    4.1.5 Index Update  ;;
(defconstraint index-reset ()
  (if-not-eq ABS_TX_NUM (prev ABS_TX_NUM) (vanishes! INDEX)))

(defconstraint index-evolution ()
  (if-not-eq ABS_TX_NUM
             (+ (prev ABS_TX_NUM) 1)
             (eq! INDEX
                  (+ (prev INDEX) (prev LC)))))

;;      4.1.6 Byte size updates     ;;
(defconstraint globalsize-update ()
  (if-zero [PHASE 1]
           (eq! TXRCPT_SIZE
                (- (prev TXRCPT_SIZE) (* LC nBYTES)))))

(defconstraint phasesize-update ()
  (if-eq 1 (+ (* [PHASE 4] (- 1 IS_PREFIX))
            (* [PHASE 5] DEPTH_1))
         (eq! PHASE_SIZE
              (- (prev PHASE_SIZE) (* LC nBYTES)))))

;;    LC correction nullity    ;;
(defconstraint lccorrection-nullity ()
  (if-zero (+ [PHASE 1] (* [PHASE 5] IS_DATA))
           (vanishes! LC_CORRECTION)))

;;    4.1.8 Finalisation Constraints    ;;
(defconstraint finalisation (:domain {-1})
  (if-not-zero ABS_TX_NUM
               (begin (eq! PHASE_END 1)
                      (eq! [PHASE 5] 1)
                      (eq! ABS_TX_NUM ABS_TX_NUM_MAX)
                      (eq! ABS_LOG_NUM ABS_LOG_NUM_MAX))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    4.2 Phase constraints   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;    4.2.1 Phase 1 : RLP prefix  ;;
(defconstraint phase1-init (:guard [PHASE 1]);; 4.1.1
  (if-zero (prev [PHASE 1])
           (begin (vanishes! (+ (- 1 IS_PREFIX) PHASE_END (next IS_PREFIX)))
                  (eq! nSTEP 1)
                  (if-zero [INPUT 1]
                           (eq! LC_CORRECTION 1)
                           (begin (vanishes! LC_CORRECTION)
                                  (eq! LIMB
                                       (* [INPUT 1] (^ 256 LLARGEMO)))
                                  (eq! nBYTES 1))))))

(defconstraint phase1-rlprefix (:guard [PHASE 1])
  (if-zero IS_PREFIX
           (begin (eq! nSTEP 8)
                  (vanishes! LC_CORRECTION)
                  (eq! [INPUT 1] TXRCPT_SIZE)
                  (rlpPrefixOfByteString [INPUT 1] CT nSTEP DONE [PHASE 1] ACC_SIZE POWER BIT [ACC 1] [ACC 2] LC LIMB nBYTES)
                  (if-eq DONE 1 (eq! PHASE_END 1)))))

;;    4.2.2 Phase 2 : status code Rz  ;;
(defconstraint phase2 ()
  (if-eq [PHASE 2] 1
         (begin (eq! nSTEP 1)
                (if-zero [INPUT 1]
                         (eq! LIMB
                              (* RLP_PREFIX_INT_SHORT (^ 256 LLARGEMO)))
                         (eq! LIMB
                              (* [INPUT 1] (^ 256 LLARGEMO))))
                (eq! nBYTES 1)
                (eq! PHASE_END 1))))

;; 4.2.3 Phase 3 : cumulative gas Ru  ;;
(defconstraint phase3 ()
  (if-eq [PHASE 3] 1
         (begin (eq! nSTEP 8)
                (rlpPrefixInt [INPUT 1] CT nSTEP DONE [BYTE 1] [ACC 1] ACC_SIZE POWER BIT BIT_ACC LIMB LC nBYTES)
                (if-eq DONE 1
                       (begin (limbShifting [INPUT 1] POWER ACC_SIZE LIMB nBYTES)
                              (eq! PHASE_END 1))))))

;;  Phase 4: bloom filter Rb    ;;
(defconstraint phase4-prefix (:guard [PHASE 4])
  (if-zero (prev [PHASE 4])
           (begin (vanishes! (+ (- 1 IS_PREFIX) PHASE_END (next IS_PREFIX)))
                  (eq! PHASE_SIZE 256)
                  (eq! nSTEP 1)
                  (eq! LIMB
                       (+ (* (+ RLP_PREFIX_INT_LONG 2) (^ 256 LLARGEMO))
                          (* PHASE_SIZE (^ 256 13))))
                  (eq! nBYTES 3)
                  (vanishes! INDEX_LOCAL))))

(defconstraint phase4-bloom-concatenation (:guard [PHASE 4])
  (if-zero IS_PREFIX
           (begin (eq! nSTEP LLARGE)
                  (if-eq DONE 1
                         (begin (for k
                                     [1 : 4]
                                     (begin (eq! [ACC k] [INPUT k])
                                            (eq! [INPUT k]
                                                 (shift LIMB (- k 4)))
                                            (eq! (shift nBYTES (- k 4))
                                                 LLARGE)))
                                (eq! (+ (shift LC -4) (shift LC -3))
                                     1)
                                (if-zero PHASE_SIZE
                                         (eq! PHASE_END 1))))
                  (eq! INDEX_LOCAL
                       (+ (prev INDEX_LOCAL)
                          (* (prev LC)
                             (- 1 (prev IS_PREFIX))))))))

;;  Phase 5: log series Rl    ;;
(defconstraint phase5-init (:guard [PHASE 5])
  (if-zero (prev [PHASE 5])
           (vanishes! (+ DEPTH_1 (- 1 IS_PREFIX) IS_TOPIC IS_DATA))))

(defconstraint phase5-phaseRlpPrefix (:guard [PHASE 5])
  (if-zero DEPTH_1
           (begin (eq! [INPUT 1] PHASE_SIZE)
                  (if-zero [INPUT 1]
                           (begin (eq! nSTEP 1)
                                  (eq! LIMB
                                       (* RLP_PREFIX_LIST_SHORT (^ 256 LLARGEMO)))
                                  (eq! nBYTES 1)
                                  (eq! PHASE_END 1))
                           (begin (eq! nSTEP 8)
                                  (rlpPrefixOfByteString [INPUT 1] CT nSTEP DONE [PHASE 5] ACC_SIZE POWER BIT [ACC 1] [ACC 2] LC LIMB nBYTES)
                                  (if-eq DONE 1
                                         (vanishes! (+ (- 1 (next DEPTH_1))
                                                       (- 1 (next IS_PREFIX))
                                                       (next IS_TOPIC)
                                                       (next IS_DATA)))))))))

(defconstraint phase5-logentryRlpPrefix (:guard [PHASE 5])
  (if-eq 1 (* DEPTH_1 IS_PREFIX (- 1 IS_TOPIC) (- 1 IS_DATA))
         (begin (eq! [INPUT 1] LOG_ENTRY_SIZE)
                (eq! nSTEP 8)
                (rlpPrefixOfByteString [INPUT 1] CT nSTEP DONE [PHASE 5] ACC_SIZE POWER BIT [ACC 1] [ACC 2] LC LIMB nBYTES)
                (if-eq DONE 1
                       (vanishes! (+ (next IS_PREFIX) (next IS_TOPIC) (next IS_DATA)))))))

(defconstraint phase5-rlpAddress (:guard [PHASE 5])
  (if-zero (+ IS_PREFIX IS_TOPIC IS_DATA)
           (begin (eq! nSTEP 3)
                  (eq! LC 1)
                  (if-eq DONE 1
                         (begin (eq! (shift LIMB -2)
                                     (* (+ RLP_PREFIX_INT_SHORT 20) (^ 256 LLARGEMO)))
                                (eq! (shift nBYTES -2) 1)
                                (eq! (prev LIMB)
                                     (* [INPUT 1] (^ 256 12)))
                                (eq! (prev nBYTES) 4)
                                (eq! LIMB [INPUT 2])
                                (eq! nBYTES LLARGE)
                                (vanishes! (+ (- 1 (next IS_PREFIX))
                                              (- 1 (next IS_TOPIC))
                                              (next IS_DATA))))))))

(defconstraint phase5-topic-prefix (:guard [PHASE 5])
  (if-eq (* IS_PREFIX IS_TOPIC) 1
         (begin (vanishes! INDEX_LOCAL)
                (eq! nSTEP 1)
                (if-zero LOCAL_SIZE
                         (begin (eq! LIMB
                                     (* RLP_PREFIX_LIST_SHORT (^ 256 LLARGEMO)))
                                (eq! nBYTES 1)
                                (eq! (next [INPUT 2]) INDEX_LOCAL)
                                (vanishes! (+ (- 1 (next IS_PREFIX))
                                              (next IS_TOPIC)
                                              (- 1 (next IS_DATA)))))
                         (begin (if-eq-else LOCAL_SIZE 33
                                            (begin (eq! LIMB
                                                        (* (+ RLP_PREFIX_LIST_SHORT LOCAL_SIZE)
                                                           (^ 256 LLARGEMO)))
                                                   (eq! nBYTES 1))
                                            (begin (eq! LIMB
                                                        (+ (* (+ RLP_PREFIX_LIST_LONG 1) (^ 256 LLARGEMO))
                                                           (* LOCAL_SIZE (^ 256 14))))
                                                   (eq! nBYTES 2)))
                                (vanishes! (+ (next IS_PREFIX)
                                              (- 1 (next IS_TOPIC))
                                              (next IS_DATA))))))))

(defconstraint phase5-topic (:guard [PHASE 5])
  (if-zero (+ IS_PREFIX (- 1 IS_TOPIC))
           (begin (eq! nSTEP 3)
                  (eq! LC 1)
                  (if-eq DONE 1
                         (begin (eq! (+ INDEX_LOCAL (shift INDEX_LOCAL -2))
                                     (* 2
                                        (+ (shift INDEX_LOCAL -3) 1)))
                                (eq! (shift LIMB -2)
                                     (* (+ RLP_PREFIX_INT_SHORT 32) (^ 256 LLARGEMO)))
                                (eq! (shift nBYTES -2) 1)
                                (eq! (prev LIMB) [INPUT 1])
                                (eq! (prev nBYTES) LLARGE)
                                (eq! LIMB [INPUT 2])
                                (eq! nBYTES LLARGE)
                                (if-zero LOCAL_SIZE
                                         (begin (eq! (next [INPUT 2]) INDEX_LOCAL)
                                                (vanishes! (+ (- 1 (next IS_PREFIX))
                                                              (next IS_TOPIC)
                                                              (- 1 (next IS_DATA)))))
                                         (vanishes! (+ (next IS_PREFIX)
                                                       (- 1 (next IS_TOPIC))
                                                       (next IS_DATA)))))))))

(defconstraint phase5-dataprefix (:guard [PHASE 5])
  (if-eq (* IS_PREFIX IS_DATA) 1
         (begin (eq! [INPUT 1] LOCAL_SIZE)
                (if-zero LOCAL_SIZE
                         (begin (eq! nSTEP 1)
                                (vanishes! LC_CORRECTION)
                                (eq! LIMB
                                     (* RLP_PREFIX_INT_SHORT (^ 256 LLARGEMO)))
                                (eq! nBYTES 1)
                                (vanishes! LOG_ENTRY_SIZE)
                                (if-zero PHASE_SIZE
                                         (eq! PHASE_END 1)
                                         (vanishes! (+ (- 1 (next IS_PREFIX))
                                                       (next IS_TOPIC)
                                                       (next IS_DATA)))))
                         (begin (eq! nSTEP 8)
                                (if-eq-else LOCAL_SIZE 1
                                            (begin (if-not-eq CT (- nSTEP 2) (vanishes! LC))
                                                   (rlpPrefixInt [INPUT 3]
                                                                 CT
                                                                 nSTEP
                                                                 DONE
                                                                 [BYTE 1]
                                                                 [ACC 1]
                                                                 ACC_SIZE
                                                                 POWER
                                                                 BIT
                                                                 BIT_ACC
                                                                 LIMB
                                                                 LC
                                                                 nBYTES)
                                                   (if-eq DONE 1
                                                          (begin (eq! (+ (prev LC_CORRECTION) LC_CORRECTION)
                                                                      1)
                                                                 (eq! (* [INPUT 3] (^ 256 LLARGEMO))
                                                                      (next [INPUT 1])))))
                                            (begin (vanishes! LC_CORRECTION)
                                                   (counter-incrementing LC)
                                                   (rlpPrefixOfByteString [INPUT 1]
                                                                          CT
                                                                          nSTEP
                                                                          DONE
                                                                          [PHASE 1]
                                                                          ACC_SIZE
                                                                          POWER
                                                                          BIT
                                                                          [ACC 1]
                                                                          [ACC 2]
                                                                          LC
                                                                          LIMB
                                                                          nBYTES)))
                                (if-eq DONE 1
                                       (vanishes! (+ (next IS_PREFIX)
                                                     (next IS_TOPIC)
                                                     (- 1 (next IS_DATA))))))))))

(defconstraint phase5-data (:guard [PHASE 5])
  (if-zero (+ IS_PREFIX (- 1 IS_DATA))
           (begin (eq! INDEX_LOCAL CT)
                  (eq! LC 1)
                  (eq! LIMB [INPUT 1])
                  (if-zero DONE
                           (eq! nBYTES LLARGE)
                           (begin (vanishes! LOCAL_SIZE)
                                  (vanishes! LOG_ENTRY_SIZE)
                                  (if-zero PHASE_SIZE
                                           (eq! PHASE_END 1)
                                           (vanishes! (+ (- 1 (next IS_PREFIX))
                                                         (next IS_TOPIC)
                                                         (next IS_DATA)))))))))

(defconstraint phase5-logEntrySize-update (:guard [PHASE 5])
  (if-zero (+ (- 1 DEPTH_1)
              (* IS_PREFIX (- 1 IS_TOPIC) (- 1 IS_DATA)))
           (eq! LOG_ENTRY_SIZE
                (- (prev LOG_ENTRY_SIZE) (* LC nBYTES)))))

(defconstraint phase5-localsize-update (:guard [PHASE 5])
  (if-zero (+ IS_PREFIX
              (- 1 (+ IS_TOPIC IS_DATA)))
           (eq! LOCAL_SIZE
                (- (prev LOCAL_SIZE) (* LC nBYTES)))))


