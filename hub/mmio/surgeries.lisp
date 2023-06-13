(module mmio)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;              6.3 RAM to RAM               ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;RamLimbExcision
(defconstraint ram-limb-excision ()
                (if-eq MICRO_INSTRUCTION RamLimbExcision
                    (begin
                        (vanishes! CN_A)
                        (= CN_B CN_T)
                        (vanishes! CN_C)

                        (vanishes! INDEX_A)
                        (= INDEX_B TLO)
                        (vanishes! INDEX_C)

                        (vanishes! VAL_A)            ;TODO remove
                        (vanishes! VAL_C)            ;TODO remove
                        (vanishes! VAL_A_NEW)        ;TODO remove
                        (vanishes! VAL_C_NEW)        ;TODO remove
                        (vanishes! ERF)              ;TODO remove
                        (vanishes! FAST)             ;TODO remove

                        (excision
                            VAL_B VAL_B_NEW
                            BYTE_B
                            ACC_1 POW_256_1
                            TBO SIZE
                            BIN_1 BIN_2 CT))))

;RamToRamSlideChunk
(defconstraint ram-to-ram-slide-chunk ()
                (if-eq MICRO_INSTRUCTION RamToRamSlideChunk
                    (begin
                        (= CN_A CN_S)
                        (= CN_B CN_T)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (= INDEX_B TLO)
                        (vanishes! INDEX_C)

                        (= VAL_A_NEW VAL_A)

                        (vanishes! VAL_C)            ;TODO remove
                        (vanishes! VAL_C_NEW)        ;TODO remove
                        (vanishes! ERF)              ;TODO remove
                        (vanishes! FAST)             ;TODO remove

                        (one-partial-to-one
                            VAL_A VAL_B VAL_B_NEW
                            BYTE_A BYTE_B
                            ACC_1 ACC_2
                            POW_256_1
                            SBO TBO SIZE
                            BIN_1 BIN_2 BIN_3 BIN_4 CT))))

;RamToRamSlideOverlappingChunk
(defconstraint ram-to-ram-slide-overlapping-chunk ()
                (if-eq MICRO_INSTRUCTION RamToRamSlideOverlappingChunk
                    (begin
                        (eq CN_A CN_S)
                        (eq CN_B CN_T)
                        (eq CN_C CN_T)

                        (eq INDEX_A SLO)
                        (eq INDEX_B TLO)
                        (eq INDEX_C (+ TLO 1))

                        (eq VAL_A_NEW VAL_A)

                        (vanishes! ERF)              ;TODO remove
                        (vanishes! FAST)             ;TODO remove

                        (one-partial-to-two
                            VAL_A VAL_B VAL_C
                            VAL_B_NEW VAL_C_NEW
                            BYTE_A BYTE_B BYTE_C
                            ACC_1 ACC_2 ACC_3 ACC_4
                            POW_256_1
                            SBO TBO SIZE
                            BIN_1 BIN_2 BIN_3 BIN_4 BIN_5 CT))))




;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;              6.4 Exo to RAM               ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;ExoToRamSlideChunk
(defconstraint exo-to-ram-slide-chunk ()
                (if-eq MICRO_INSTRUCTION ExoToRamSlideChunk
                    (begin
                        (vanishes! CN_A)
                        (= CN_B CN_T)
                        (vanishes! CN_C)

                        (vanishes! INDEX_A)
                        (= INDEX_B TLO)
                        (vanishes! INDEX_C)
                        (= INDEX_X SLO)

                        (vanishes! VAL_A)            ;TODO remove
                        (vanishes! VAL_C)            ;TODO remove
                        (vanishes! VAL_A_NEW)        ;TODO remove
                        (vanishes! VAL_C_NEW)        ;TODO remove
                        (vanishes! ERF)              ;TODO remove
                        (vanishes! FAST)             ;TODO remove

                        (one-partial-to-one
                            VAL_X VAL_B VAL_B_NEW
                            BYTE_X BYTE_B
                            ACC_1 ACC_2
                            POW_256_1
                            SBO TBO SIZE
                            BIN_1 BIN_2 BIN_3 BIN_4 CT))))

;ExoToRamSlideOverlappingChunk
(defconstraint exo-to-ram-slide-overlapping-chunk ()
                (if-eq MICRO_INSTRUCTION ExoToRamSlideOverlappingChunk
                    (begin
                        (vanishes! CN_A)
                        (= CN_B CN_T)
                        (= CN_C CN_T)

                        (vanishes! INDEX_A)
                        (eq INDEX_B TLO)
                        (eq INDEX_C (+ TLO 1))
                        (= INDEX_X SLO)

                        (vanishes! VAL_A)            ;TODO remove
                        (vanishes! VAL_A_NEW)        ;TODO remove
                        (vanishes! ERF)              ;TODO remove
                        (vanishes! FAST)             ;TODO remove

                        (one-partial-to-two
                            VAL_X VAL_B VAL_C
                            VAL_B_NEW VAL_C_NEW
                            BYTE_X BYTE_B BYTE_C
                            ACC_1 ACC_2 ACC_3 ACC_4
                            POW_256_1
                            SBO TBO SIZE
                            BIN_1 BIN_2 BIN_3 BIN_4 BIN_5 CT))))




;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;              6.5 RAM to Exo               ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;PaddedExoFromOne
(defconstraint padded-exo-from-one ()
                (if-eq MICRO_INSTRUCTION PaddedExoFromOne
                    (begin
                        (= CN_A CN_S)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (vanishes! INDEX_B)
                        (vanishes! INDEX_C)
                        (= INDEX_X TLO)

                        (= VAL_A_NEW VAL_A)

                        (vanishes! VAL_B)            ;TODO remove
                        (vanishes! VAL_C)            ;TODO remove
                        (vanishes! VAL_B_NEW)        ;TODO remove
                        (vanishes! VAL_C_NEW)        ;TODO remove
                        (vanishes! ERF)              ;TODO remove
                        (vanishes! FAST)             ;TODO remove

                        (one-to-one-padded
                            VAL_A VAL_X
                            BYTE_A
                            ACC_1 POW_256_1
                            SBO SIZE
                            BIN_1 BIN_2 BIN_3 CT))))

;PaddedExoFromTwo
(defconstraint padded-exo-from-two ()
                (if-eq MICRO_INSTRUCTION PaddedExoFromTwo
                    (begin
                        (= CN_A CN_S)
                        (= CN_B CN_S)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (= INDEX_B (+ SLO 1))
                        (vanishes! INDEX_C)
                        (= INDEX_X TLO)

                        (= VAL_A_NEW VAL_A)
                        (= VAL_B_NEW VAL_B)

                        (vanishes! VAL_C)            ;TODO remove
                        (vanishes! VAL_C_NEW)        ;TODO remove
                        (vanishes! ERF)              ;TODO remove
                        (vanishes! FAST)             ;TODO remove

                        (two-to-one-padded
                            VAL_A VAL_B VAL_X
                            BYTE_A BYTE_B
                            ACC_1 ACC_2 POW_256_1 POW_256_2
                            SBO SIZE
                            BIN_1 BIN_2 BIN_3 BIN_4 CT))))

;FullExoFromTwo
(defconstraint full-exo-from-two ()
                (if-eq MICRO_INSTRUCTION FullExoFromTwo
                    (begin
                        (= CN_A CN_S)
                        (= CN_B CN_S)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (= INDEX_B (+ SLO 1))
                        (vanishes! INDEX_C)
                        (= INDEX_X TLO)

                        (= VAL_A_NEW VAL_A)
                        (= VAL_B_NEW VAL_B)

                        (vanishes! VAL_C)            ;TODO remove
                        (vanishes! VAL_C_NEW)        ;TODO remove
                        (vanishes! ERF)              ;TODO remove
                        (vanishes! FAST)             ;TODO remove

                        (two-to-one-full
                            VAL_A VAL_B VAL_X
                            BYTE_A BYTE_B
                            ACC_1 ACC_2 POW_256_1
                            SBO BIN_1 BIN_2 CT))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;              6.6 Stack to RAM               ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;FullStackToRam
(defconstraint full-stack-to-ram ()
                (if-eq MICRO_INSTRUCTION FullStackToRam
                    (begin
                        (= CN_A CN_T)
                        (= CN_B CN_T)
                        (= CN_C CN_T)

                        (= INDEX_A TLO)
                        (= INDEX_B (+ TLO 1))
                        (= INDEX_C (+ TLO 2))

                        (vanishes! ERF)              ;TODO remove
                        (vanishes! FAST)             ;TODO remove

                        (two-full-to-three
                            VAL_A VAL_C								;T1 T3
                            VAL_HI VAL_LO							;S1 S2
                            VAL_A_NEW VAL_B_NEW VAL_C_NEW			;T1_NEW T_2_NEW T3_NEW
                            BYTE_A BYTE_C							;T1B T3B
                            BYTE_HI BYTE_LO                         ;S1B S3B
                            ACC_1 ACC_2 ACC_3 ACC_4 ACC_5 ACC_6
                            POW_256_1 TBO BIN_1 BIN_2 CT))))

;LsbFromStackToRAM
(defconstraint lsb-from-stack-to-ram ()
                (if-eq MICRO_INSTRUCTION LsbFromStackToRAM
                    (begin
                        (= CN_A CN_T)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (= INDEX_A TLO)
                        (vanishes! INDEX_B)
                        (vanishes! INDEX_C)

                        (vanishes! VAL_B)			;TODO remove
                        (vanishes! VAL_C)			;TODO remove
                        (vanishes! VAL_B_NEW)		;TODO remove
                        (vanishes! VAL_C_NEW)		;TODO remove
                        (vanishes! ERF)              ;TODO remove
                        (vanishes! FAST)             ;TODO remove

                        (byte-swap
                            VAL_LO VAL_A VAL_A_NEW
                            BYTE_LO BYTE_A
                            ACC_1 POW_256_1
                            TBO BIN_1 BIN_2 CT))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                              ;;
;;              6.7 RAM to stack: aligned offsets               ;;
;;                                                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;FirstFastSecondPadded
(defconstraint first-fast-second-padded ()
                (if-eq MICRO_INSTRUCTION FirstFastSecondPadded
                    (begin
                        (= CN_A CN_S)
                        (= CN_B CN_S)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (= INDEX_B (+ SLO 1))
                        (vanishes! INDEX_C)

                        (= VAL_A_NEW VAL_A)
                        (= VAL_B_NEW VAL_B)
                        (= VAL_HI VAL_A)

                        (vanishes! VAL_C)			;TODO remove
                        (vanishes! VAL_C_NEW)		;TODO remove
                        (vanishes! ERF)              ;TODO remove
                        (vanishes! FAST)             ;TODO remove

                        (one-to-one-padded
                            VAL_B VAL_LO
                            BYTE_B
                            ACC_1 POW_256_1
                            0 SIZE BIN_1 BIN_2 BIN_3 CT))))

;FirstPaddedSecondZero
(defconstraint first-padded-second-zero ()
                (if-eq MICRO_INSTRUCTION FirstPaddedSecondZero
                    (begin
                        (= CN_A CN_S)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (vanishes! INDEX_B)
                        (vanishes! INDEX_C)

                        (= VAL_A_NEW VAL_A)
                        (vanishes! VAL_LO)

                        (vanishes! VAL_B)			;TODO remove
                        (vanishes! VAL_C)			;TODO remove
                        (vanishes! VAL_B_NEW)		;TODO remove
                        (vanishes! VAL_C_NEW)		;TODO remove
                        (vanishes! ERF)              ;TODO remove
                        (vanishes! FAST)             ;TODO remove

                        (one-to-one-padded
                            VAL_A VAL_HI
                            BYTE_A
                            ACC_1 POW_256_1
                            0 SIZE BIN_1 BIN_2 BIN_3 CT))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                  ;;
;;              6.8 RAM to stack: non-aligned offsets               ;;
;;                                                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;Exceptional_RamToStack_3To2Full
(defconstraint exceptional-ram-to-stack-three-to-two-full ()
                (if-eq MICRO_INSTRUCTION Exceptional_RamToStack_3To2Full
                    (begin
                        (vanishes! CN_A)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (vanishes! INDEX_A)
                        (vanishes! INDEX_B)
                        (vanishes! INDEX_C)

                        (vanishes! VAL_A_NEW)        ;TODO remove
                        (vanishes! VAL_B_NEW)        ;TODO remove
                        (vanishes! VAL_C_NEW)        ;TODO remove
                        (vanishes! FAST)				;TODO remove
                        (vanishes! ERF)				;TODO remove

                        (three-to-two-full
                            VAL_A VAL_B VAL_C
                            VAL_HI VAL_LO
                            BYTE_A BYTE_B BYTE_C
                            BIN_1 BIN_2
                            POW_256_1 SBO
                            ACC_1 ACC_2 ACC_3 ACC_4 CT))))

;NA_RamToStack_3To2Full
(defconstraint na-ram-to-stack-three-to-two-full ()
                (if-eq MICRO_INSTRUCTION NA_RamToStack_3To2Full
                    (begin
                        (= CN_A CN_S)
                        (= CN_B CN_S)
                        (= CN_C CN_S)

                        (= INDEX_A SLO)
                        (= INDEX_B (+ SLO 1))
                        (= INDEX_C (+ SLO 2))

                        (= VAL_A_NEW VAL_A)
                        (= VAL_B_NEW VAL_B)
                        (= VAL_C_NEW VAL_C)

                        (vanishes! FAST)				;TODO remove
                        (vanishes! ERF)				;TODO remove

                        (three-to-two-full
                            VAL_A VAL_B VAL_C
                            VAL_HI VAL_LO
                            BYTE_A BYTE_B BYTE_C
                            BIN_1 BIN_2
                            POW_256_1 SBO
                            ACC_1 ACC_2 ACC_3 ACC_4 CT))))

;NA_RamToStack_3To2Padded
(defconstraint na-ram-to-stack-three-to-two-padded ()
                (if-eq MICRO_INSTRUCTION NA_RamToStack_3To2Padded
                    (begin
                        (= CN_A CN_S)
                        (= CN_B CN_S)
                        (= CN_C CN_S)

                        (= INDEX_A SLO)
                        (= INDEX_B (+ SLO 1))
                        (= INDEX_C (+ SLO 2))

                        (= VAL_A_NEW VAL_A)
                        (= VAL_B_NEW VAL_B)
                        (= VAL_C_NEW VAL_C)

                        (vanishes! FAST)				;TODO remove
                        (vanishes! ERF)				;TODO remove

                        (two-to-one-full
                            VAL_A VAL_B VAL_HI
                            BYTE_A BYTE_B
                            ACC_1 ACC_2 POW_256_1
                            SBO BIN_1 BIN_2 CT)

                        (two-to-one-padded
                            VAL_B VAL_C VAL_LO
                            BYTE_B BYTE_C
                            ACC_3 ACC_4 POW_256_1 POW_256_2
                            SBO SIZE
                            BIN_1 BIN_3 BIN_2 BIN_4 CT)))) ;Mind the order !! cf spec

;NA_RamToStack_2To2Padded
(defconstraint na-ram-to-stack-two-to-two-padded ()
                (if-eq MICRO_INSTRUCTION NA_RamToStack_2To2Padded
                    (begin
                        (= CN_A CN_S)
                        (= CN_B CN_S)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (= INDEX_B (+ SLO 1))
                        (vanishes! INDEX_C)

                        (= VAL_A_NEW VAL_A)
                        (= VAL_B_NEW VAL_B)

                        (vanishes! VAL_C)            ;TODO remove
                        (vanishes! VAL_C_NEW)        ;TODO remove
                        (vanishes! FAST)				;TODO remove
                        (vanishes! ERF)				;TODO remove

                        (two-to-one-full
                            VAL_A VAL_B VAL_HI
                            BYTE_A BYTE_B
                            ACC_1 ACC_2 POW_256_1
                            SBO BIN_1 BIN_2 CT)

                        (one-to-one-padded
                            VAL_B VAL_LO
                            BYTE_B
                            ACC_3 POW_256_2
                            SBO SIZE
                            BIN_1 BIN_3 BIN_4 CT)))) ;Mind the order !! cf spec

;NA_RamToStack_2To1FullAndZero
(defconstraint na-ram-to-stack-two-to-one-full-and-zero ()
                (if-eq MICRO_INSTRUCTION NA_RamToStack_2To1FullAndZero
                    (begin
                        (= CN_A CN_S)
                        (= CN_B CN_S)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (= INDEX_B (+ SLO 1))
                        (vanishes! INDEX_C)

                        (= VAL_A_NEW VAL_A)
                        (= VAL_B_NEW VAL_B)
                        (vanishes! VAL_LO)


                        (vanishes! VAL_C)            ;TODO remove
                        (vanishes! VAL_C_NEW)        ;TODO remove
                        (vanishes! FAST)				;TODO remove
                        (vanishes! ERF)				;TODO remove

                        (two-to-one-full
                            VAL_A VAL_B VAL_HI
                            BYTE_A BYTE_B
                            ACC_1 ACC_2 POW_256_1
                            SBO BIN_1 BIN_2 CT))))

;NA_RamToStack_2To1PaddedAndZero
(defconstraint na-ram-to-stack-two-to-one-padded-and-zero ()
                (if-eq MICRO_INSTRUCTION NA_RamToStack_2To1PaddedAndZero
                    (begin
                        (= CN_A CN_S)
                        (= CN_B CN_S)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (= INDEX_B (+ SLO 1))
                        (vanishes! INDEX_C)

                        (= VAL_A_NEW VAL_A)
                        (= VAL_B_NEW VAL_B)
                        (vanishes! VAL_LO)


                        (vanishes! VAL_C)            ;TODO remove
                        (vanishes! VAL_C_NEW)        ;TODO remove
                        (vanishes! FAST)				;TODO remove
                        (vanishes! ERF)				;TODO remove

                        (two-to-one-padded
                            VAL_A VAL_B VAL_HI
                            BYTE_A BYTE_B
                            ACC_1 ACC_2 POW_256_1 POW_256_2
                            SBO SIZE BIN_1 BIN_2 BIN_3 BIN_4 CT))))

;NA_RamToStack_1To1PaddedAndZero
(defconstraint na-ram-to-stack-one-to-one-padded-and-zero ()
                (if-eq MICRO_INSTRUCTION NA_RamToStack_1To1PaddedAndZero
                    (begin
                        (= CN_A CN_S)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (vanishes! INDEX_B)
                        (vanishes! INDEX_C)

                        (= VAL_A_NEW VAL_A)
                        (vanishes! VAL_LO)


                        (vanishes! VAL_B)            ;TODO remove
                        (vanishes! VAL_C)            ;TODO remove
                        (vanishes! VAL_B_NEW)        ;TODO remove
                        (vanishes! VAL_C_NEW)        ;TODO remove
                        (vanishes! FAST)				;TODO remove
                        (vanishes! ERF)				;TODO remove

                        (one-to-one-padded
                            VAL_A VAL_HI
                            BYTE_A
                            ACC_1 POW_256_1
                            SBO SIZE BIN_1 BIN_2 BIN_3 CT))))
