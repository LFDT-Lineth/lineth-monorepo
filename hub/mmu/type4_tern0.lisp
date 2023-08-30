(module mmu)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                                                   ;;;;
;;;;    4.7 TYPE 4 and TERN = 0: the pure data case    ;;;;
;;;;                                                   ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    4.7.1 preprocessing    ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defun (euclidean-divisions-tern0)
                (begin
                    (=  (+ (* 16 ACC_3) NIB_3)  (+ REFO OFF_2_LO))
                    (=  (+ (* 16 ACC_4) NIB_4)  (+ REFO OFF_2_LO (- SIZE_IMPORTED 1)))
                    (=  (+ (* 16 ACC_5) NIB_5)  OFF_1_LO)
                    (=  (+ (* 16 ACC_6) NIB_6)  (+ OFF_1_LO  (- SIZE_IMPORTED 1)))))

(defun (comparisons-tern0)
                (begin
                    (=  (+ NIB_1 BIT_1)  (* (- NIB_5 NIB_3) (- (* 2 BIT_1) 1)))
                    (=  (+ NIB_2 BIT_2)  (* (- NIB_4 NIB_6) (- (* 2 BIT_2) 1)))))

(defun (workflow-parameters-tern0)
                (begin
                    (=  (+ TOT ACC_3) (+ ACC_4 1))
                    (if-eq-else NIB_3 NIB_5
                        (= ALIGNED 1)
                        (= ALIGNED 0))
                    (if-eq-else TOT 1
                        (= BIT_3 1)
                        (= BIT_3 0))
                    (if-zero BIT_3
                        (vanishes! BIT_4)
                        (=  (+ NIB_5 SIZE_IMPORTED)  (+ (* LLARGE BIT_4) NIB_7 1)))))

(defun (initial-slo-sbo-tlo-tbo-tern0)
                (begin
                    (= SLO ACC_3)
                    (= SBO NIB_3)
                    (= TLO ACC_5)
                    (= TBO NIB_5)
                    (= (next SLO) SLO)
                    (= (next SBO) SBO)
                    (= (next TLO) TLO)
                    (= (next TBO) TBO)))

(defconstraint preprocessing-tern0 ()
                (if-zero (* (- PRE type4CC) (- PRE type4CD) (- PRE type4RD))
                    (if-eq TERN tern0
                        (if-eq IS_MICRO 0
                            (if-eq (next IS_MICRO) 1
                                (begin
                                    (euclidean-divisions-tern0)
                                    (comparisons-tern0)
                                    (workflow-parameters-tern0)
                                    (initial-slo-sbo-tlo-tbo-tern0)))))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;    4.7.2 micro-instruction writing    ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (modify-one-limb)
                (begin
                    (if-eq PRE type4CC (= MICRO_INST ExoToRamSlideChunk))
                    (if-eq PRE type4RD (= MICRO_INST RamToRamSlideChunk))
                    (if-eq PRE type4CD
                        (if-zero INFO
                            (= MICRO_INST RamToRamSlideChunk)
                            (= MICRO_INST ExoToRamSlideChunk)))))

(defun (modify-two-limbs)
                (begin
                    (if-eq PRE type4CC (= MICRO_INST ExoToRamSlideOverlappingChunk))
                    (if-eq PRE type4RD (= MICRO_INST RamToRamSlideOverlappingChunk))
                    (if-eq PRE type4CD
                        (if-zero INFO
                            (= MICRO_INST RamToRamSlideOverlappingChunk)
                            (= MICRO_INST ExoToRamSlideOverlappingChunk)))))

(defun (swap-one-limb)
                (begin
                    (if-eq PRE type4CC (= MICRO_INST ExoToRam))
                    (if-eq PRE type4RD (= MICRO_INST RamToRam))
                    (if-eq PRE type4CD
                        (if-zero INFO
                            (= MICRO_INST RamToRam)
                            (= MICRO_INST ExoToRam)))))

(defun (bit_3-equals-one-case)
                (begin
                    (= SIZE SIZE_IMPORTED)
                    (if-zero BIT_4
                        (modify-one-limb)
                        (modify-two-limbs))))

(defun (bit_3-equals-zero-case)
                (begin
                    (= SLO (prev (+ SLO IS_MICRO)))
                    (if-zero (prev IS_MICRO)
                        (will-eq! TLO (+ TLO ALIGNED BIT_1))
                        (if-not-zero TOT (will-inc! TLO 1)))
                    (if-zero (prev IS_MICRO)
                        (begin
                            (= SIZE (- LLARGE NIB_3))
                            (if-zero BIT_1
                                (modify-one-limb)
                                (modify-two-limbs)))
                        (begin
                            (vanishes! SBO)
                            (=
                                (+ TBO NIB_3 (* LLARGE (+ ALIGNED BIT_1)))
                                (+ NIB_5 LLARGE))
                            (if-not-zero TOT
                                (if-not-zero ALIGNED
                                    (swap-one-limb)
                                    (modify-two-limbs))
                                (begin
                                    (= SIZE (+ NIB_4 1))
                                    (if-zero BIT_2
                                        (modify-one-limb)
                                        (modify-two-limbs))))))))

(defconstraint micro-instruction-writing-tern0 ()
                (if-zero (* (- PRE type4CC) (- PRE type4CD) (- PRE type4RD))
                    (if-eq TERN tern0
                        (if-eq IS_MICRO 1
                            (if-not-zero BIT_3
                                (bit_3-equals-one-case)
                                (bit_3-equals-zero-case))))))
