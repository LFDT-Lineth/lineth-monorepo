(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;   ANY_TO_RAM_WITH_PADDING   ;;
;;;;;;;;;;;;;;;;;;;;;;;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;       some data case        ;;
;;;;;;;;;;;;;;;;;;;;;;;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (any-to-ram-with-padding---some-data---tlo-increment-after-first-dt)      [BIN 1])
(defun    (any-to-ram-with-padding---some-data---aligned)                           [BIN 2])
(defun    (any-to-ram-with-padding---some-data---middle-tbo)                        [OUT 1])
(defun    (any-to-ram-with-padding---some-data---last-dt-single-target)             [BIN 3])
(defun    (any-to-ram-with-padding---some-data---last-dt-size)                      [OUT 2])
(defun    (any-to-ram-with-padding---some-data---tlo-increment-at-transition)       [BIN 4])
(defun    (any-to-ram-with-padding---some-data---first-pbo)                         [OUT 3])
(defun    (any-to-ram-with-padding---some-data---first-padding-size)                [OUT 4])
(defun    (any-to-ram-with-padding---some-data---last-padding-size)                 [OUT 5])
(defun    (any-to-ram-with-padding---some-data---data-src-is-ram)                   [BIN 5])
(defun    (any-to-ram-with-padding---some-data---totnt-is-one)                      (shift prprc/WCP_RES 4))
(defun    (any-to-ram-with-padding---some-data---only-dt-size)                      (any-to-ram-with-padding---trsf-size))
(defun    (any-to-ram-with-padding---some-data---first-dt-size)                     (- LLARGE (any-to-ram-with-padding---some-data---min-sbo)))
(defun    (any-to-ram-with-padding---some-data---min-src-offset)                    (+ macro/SRC_OFFSET_LO macro/REF_OFFSET))
(defun    (any-to-ram-with-padding---some-data---min-slo)                           (shift prprc/EUC_QUOT 5))
(defun    (any-to-ram-with-padding---some-data---min-sbo)                           (shift prprc/EUC_REM 5))
(defun    (any-to-ram-with-padding---some-data---max-src-offset)                    (+ (any-to-ram-with-padding---some-data---min-src-offset) (- (any-to-ram-with-padding---trsf-size) 1)))
(defun    (any-to-ram-with-padding---some-data---max-slo)                           (shift prprc/EUC_QUOT 6))
(defun    (any-to-ram-with-padding---some-data---max-sbo)                           (shift prprc/EUC_REM 6))
(defun    (any-to-ram-with-padding---some-data---only-dt-single-target)             (force-bin (- 1 (shift prprc/EUC_QUOT 7))))
(defun    (any-to-ram-with-padding---some-data---only-dt-maxes-out-target)          (shift prprc/WCP_RES 7))
(defun    (any-to-ram-with-padding---some-data---first-dt-single-target)            (force-bin (- 1 (shift prprc/EUC_QUOT 8))))
(defun    (any-to-ram-with-padding---some-data---first-dt-maxes-out-target)         (shift prprc/WCP_RES 8))
(defun    (any-to-ram-with-padding---some-data---last-dt-maxes-out-target)          (shift prprc/WCP_RES 9))
(defun    (any-to-ram-with-padding---some-data---first-padding-offset)              (+ (any-to-ram-with-padding---min-tgt-offset) (any-to-ram-with-padding---trsf-size)))
(defun    (any-to-ram-with-padding---some-data---first-plo)                         (shift prprc/EUC_QUOT 10))
(defun    (any-to-ram-with-padding---some-data---last-plo)                          (any-to-ram-with-padding---max-tlo))
(defun    (any-to-ram-with-padding---some-data---last-pbo)                          (any-to-ram-with-padding---max-tbo))
(defun    (any-to-ram-with-padding---some-data---totrz-is-one)                      (shift prprc/WCP_RES 10))
(defun    (any-to-ram-with-padding---some-data---micro-cns)                         (* (any-to-ram-with-padding---some-data---data-src-is-ram) macro/SRC_ID))
(defun    (any-to-ram-with-padding---some-data---micro-id1)                         (* (- 1 (any-to-ram-with-padding---some-data---data-src-is-ram)) macro/SRC_ID)) ;; ""

(defconstraint    any-to-ram-with-padding---some-data---3rd-preprocessing-row (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                  (begin
                    ;; preprocessing row n°3
                    (callToIszero 3 0 macro/EXO_SUM)
                    (eq! (any-to-ram-with-padding---some-data---data-src-is-ram) (shift prprc/WCP_RES 3))))

(defconstraint    any-to-ram-with-padding---some-data---setting-TOTLZ-and-TOTNT (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                  (begin
                    ;; setting nb of rows
                    (vanishes! TOTLZ)
                    (eq!       TOTNT
                               (+ (- (any-to-ram-with-padding---some-data---max-slo) (any-to-ram-with-padding---some-data---min-slo)) 1))))

(defconstraint    any-to-ram-with-padding---some-data---4th-preprocessing-row (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                  (begin
                    ;; preprocessing row n°4
                    (callToEq 4 0 TOTNT 1)
                    (eq! (any-to-ram-with-padding---some-data---last-dt-size) (+ (any-to-ram-with-padding---some-data---max-sbo) 1))))

(defconstraint    any-to-ram-with-padding---some-data---5th-preprocessing-row (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                  (begin
                    ;; preprocessing row n°5
                    (callToEuc 5 (any-to-ram-with-padding---some-data---min-src-offset) LLARGE)
                    (callToEq 5 0 (any-to-ram-with-padding---min-tbo) (any-to-ram-with-padding---some-data---min-sbo))
                    (eq! (any-to-ram-with-padding---some-data---aligned) (shift prprc/WCP_RES 5))))

(defconstraint    any-to-ram-with-padding---some-data---6th-preprocessing-row (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                    ;; preprocessing row n°6
                    (callToEuc 6 (any-to-ram-with-padding---some-data---max-src-offset) LLARGE))

(defconstraint    any-to-ram-with-padding---some-data---7th-preprocessing-row (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                    ;; preprocessing row n°7
                    (if-eq (any-to-ram-with-padding---some-data---totnt-is-one) 1
                           (begin (callToEuc 7
                                             (+ (any-to-ram-with-padding---min-tbo) (- (any-to-ram-with-padding---some-data---only-dt-size) 1))
                                             LLARGE)
                                  (callToEq 7 0 (shift prprc/EUC_REM 7) LLARGEMO))))

(defconstraint    any-to-ram-with-padding---some-data---8th-preprocessing-row (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                    ;; preprocessing row n°8
                    (if-zero (any-to-ram-with-padding---some-data---totnt-is-one)
                             (begin (callToEuc 8
                                               (+ (any-to-ram-with-padding---min-tbo) (- (any-to-ram-with-padding---some-data---first-dt-size) 1))
                                               LLARGE)
                                    (callToEq 8 0 (shift prprc/EUC_REM 8) LLARGEMO)
                                    (if-zero (any-to-ram-with-padding---some-data---first-dt-maxes-out-target)
                                             (eq! (any-to-ram-with-padding---some-data---middle-tbo)
                                                  (+ 1 (shift prprc/EUC_REM 8)))
                                             (vanishes! (any-to-ram-with-padding---some-data---middle-tbo))))))

(defconstraint    any-to-ram-with-padding---some-data---9th-preprocessing-row (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                    ;; preprocessing row n°9
                    (if-zero (any-to-ram-with-padding---some-data---totnt-is-one)
                             (begin (callToEuc 9
                                               (+ (any-to-ram-with-padding---some-data---middle-tbo)
                                                  (- (any-to-ram-with-padding---some-data---last-dt-size) 1))
                                               LLARGE)
                                    (callToEq 9 0 (shift prprc/EUC_REM 9) LLARGEMO)
                                    (eq! (any-to-ram-with-padding---some-data---last-dt-single-target)
                                         (- 1 (shift prprc/EUC_QUOT 9)))
                                    (eq! (any-to-ram-with-padding---some-data---last-dt-maxes-out-target) (shift prprc/WCP_RES 9))
                                    (if-not-zero (any-to-ram-with-padding---some-data---totnt-is-one)
                                                 (vanishes! (any-to-ram-with-padding---some-data---tlo-increment-after-first-dt))
                                                 (if-zero (any-to-ram-with-padding---some-data---first-dt-single-target)
                                                          (eq! (any-to-ram-with-padding---some-data---tlo-increment-after-first-dt) 1)
                                                          (eq! (any-to-ram-with-padding---some-data---tlo-increment-after-first-dt)
                                                               (any-to-ram-with-padding---some-data---first-dt-maxes-out-target)))))))

(defconstraint    any-to-ram-with-padding---some-data---justifying-tlo-increments-at-transition (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                    ;; justifying tlo_increments_at_transition
                    (if-eq-else (any-to-ram-with-padding---some-data---totnt-is-one) 1
                                (if-zero (any-to-ram-with-padding---some-data---only-dt-single-target)
                                         (eq! (any-to-ram-with-padding---some-data---tlo-increment-at-transition) 1)
                                         (eq! (any-to-ram-with-padding---some-data---tlo-increment-at-transition)
                                              (any-to-ram-with-padding---some-data---only-dt-maxes-out-target)))
                                (if-zero (any-to-ram-with-padding---some-data---last-dt-single-target)
                                         (eq! (any-to-ram-with-padding---some-data---tlo-increment-at-transition) 1)
                                         (eq! (any-to-ram-with-padding---some-data---tlo-increment-at-transition)
                                              (any-to-ram-with-padding---some-data---last-dt-maxes-out-target)))))

(defconstraint    any-to-ram-with-padding---some-data---10th-preprocessing-row (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                  (begin
                    ;; preprocessing row n°10
                    (callToEq 10 0 TOTRZ 1)
                    (callToEuc 10 (any-to-ram-with-padding---some-data---first-padding-offset) LLARGE)
                    (if-eq (any-to-ram-with-padding---pure-data) 1 (vanishes! TOTRZ))
                    (if-eq (any-to-ram-with-padding---mixed) 1
                           (eq! TOTRZ
                                (+ (- (any-to-ram-with-padding---some-data---last-plo) (any-to-ram-with-padding---some-data---first-plo)) 1)))
                    (eq! (any-to-ram-with-padding---some-data---first-pbo)
                         (* (any-to-ram-with-padding---mixed) (shift prprc/EUC_REM 10)))
                    (if-eq-else (any-to-ram-with-padding---some-data---totrz-is-one) 1
                                (eq! (any-to-ram-with-padding---some-data---first-padding-size) (any-to-ram-with-padding---padd-size))
                                (begin (eq! (any-to-ram-with-padding---some-data---first-padding-size)
                                            (* (any-to-ram-with-padding---mixed) (- LLARGE (any-to-ram-with-padding---some-data---first-pbo))))
                                       (eq! (any-to-ram-with-padding---some-data---last-padding-size)
                                            (* (any-to-ram-with-padding---mixed) (+ 1 (any-to-ram-with-padding---some-data---last-pbo))))))))

(defconstraint    any-to-ram-with-padding---some-data---setting-micro-instruction-constant-values (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                  (begin
                    ;; initialisation
                    (eq!    (shift micro/CN_S          NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)    (any-to-ram-with-padding---some-data---micro-cns))
                    (eq!    (shift micro/CN_T          NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)    macro/TGT_ID)
                    (eq!    (shift micro/EXO_SUM       NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)    macro/EXO_SUM)
                    (eq!    (shift micro/EXO_ID        NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)    (any-to-ram-with-padding---some-data---micro-id1))
                    (eq!    (shift micro/TOTAL_SIZE    NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)    macro/REF_SIZE)))

(defconstraint    any-to-ram-with-padding---some-data---initializing-SLO-SBO-TLO-TBO (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                  (begin
                    ;; FIRST and ONLY mmio inst shared values
                    (eq! (shift micro/SLO NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) (any-to-ram-with-padding---some-data---min-slo))
                    (eq! (shift micro/SBO NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) (any-to-ram-with-padding---some-data---min-sbo))
                    (eq! (shift micro/TLO NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) (any-to-ram-with-padding---min-tlo))
                    (eq! (shift micro/TBO NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) (any-to-ram-with-padding---min-tbo))))

(defconstraint    any-to-ram-with-padding---some-data---ONLY-and-FIRST-micro-instruction-writing (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
                  (begin
                    (if-eq-else (any-to-ram-with-padding---some-data---totnt-is-one) 1
                                ;; ONLY mmio inst
                                (begin (if-eq-else (any-to-ram-with-padding---some-data---data-src-is-ram) 1
                                                   (if-zero (any-to-ram-with-padding---some-data---only-dt-single-target)
                                                            (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) MMIO_INST_RAM_TO_RAM_TWO_TARGET)
                                                            (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) MMIO_INST_RAM_TO_RAM_PARTIAL))
                                                   (if-zero (any-to-ram-with-padding---some-data---only-dt-single-target)
                                                            (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                                                            (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) MMIO_INST_LIMB_TO_RAM_ONE_TARGET)))
                                       (eq! (shift micro/SIZE NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
                                            (any-to-ram-with-padding---some-data---only-dt-size)))
                                ;; FIRST mmio inst
                                (begin (if-eq-else (any-to-ram-with-padding---some-data---data-src-is-ram) 1
                                                   (if-zero (any-to-ram-with-padding---some-data---first-dt-single-target)
                                                            (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) MMIO_INST_RAM_TO_RAM_TWO_TARGET)
                                                            (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) MMIO_INST_RAM_TO_RAM_PARTIAL))
                                                   (if-zero (any-to-ram-with-padding---some-data---first-dt-single-target)
                                                            (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                                                            (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) MMIO_INST_LIMB_TO_RAM_ONE_TARGET)))
                                       (eq! (shift micro/SIZE NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
                                            (any-to-ram-with-padding---some-data---first-dt-size))))))

(defconstraint    any-to-ram-with-padding---some-data---paying-forward-after-FIRST (:guard IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA)
                    (if-eq NT_FIRST 1
                                (begin (will-inc! micro/SLO 1)
                                       (vanishes! (next micro/SBO))
                                       (if-zero (any-to-ram-with-padding---some-data---tlo-increment-after-first-dt)
                                                (will-remain-constant! micro/TLO)
                                                (will-inc! micro/TLO 1))
                                       (will-eq! micro/TBO (any-to-ram-with-padding---some-data---middle-tbo)))))

(defconstraint    any-to-ram-with-padding---some-data---MDDL-and-LAST-micro-instruction-writing (:guard IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA)
                  (begin
                         (if-eq NT_MDDL 1
                                (begin (if-eq-else (any-to-ram-with-padding---some-data---data-src-is-ram) 1
                                                   (if-zero (any-to-ram-with-padding---some-data---aligned)
                                                            (eq! micro/INST MMIO_INST_RAM_TO_RAM_TWO_TARGET)
                                                            (eq! micro/INST MMIO_INST_RAM_TO_RAM_TRANSPLANT))
                                                   (if-zero (any-to-ram-with-padding---some-data---aligned)
                                                            (eq! micro/INST MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                                                            (eq! micro/INST MMIO_INST_LIMB_TO_RAM_TRANSPLANT)))
                                       (eq! micro/SIZE LLARGE)
                                       (will-inc! micro/SLO 1)
                                       (vanishes! (next micro/SBO))
                                       (will-inc! micro/TLO 1)
                                       (will-eq! micro/TBO (any-to-ram-with-padding---some-data---middle-tbo))))
                         (if-eq NT_LAST 1
                                (begin (if-eq-else (any-to-ram-with-padding---some-data---data-src-is-ram) 1
                                                   (if-zero (any-to-ram-with-padding---some-data---last-dt-single-target)
                                                            (eq! micro/INST MMIO_INST_RAM_TO_RAM_TWO_TARGET)
                                                            (eq! micro/INST MMIO_INST_RAM_TO_RAM_PARTIAL))
                                                   (if-zero (any-to-ram-with-padding---some-data---last-dt-single-target)
                                                            (eq! micro/INST MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                                                            (eq! micro/INST MMIO_INST_LIMB_TO_RAM_ONE_TARGET)))
                                       (eq! micro/SIZE (any-to-ram-with-padding---some-data---last-dt-size))))
                         (if-eq (force-bin (+ RZ_FIRST RZ_ONLY)) 1
                                (begin (eq! micro/INST MMIO_INST_RAM_EXCISION)
                                       (eq! micro/SIZE (any-to-ram-with-padding---some-data---first-padding-size))
                                       (did-inc! micro/TLO (any-to-ram-with-padding---some-data---tlo-increment-at-transition))
                                       (eq! micro/TBO (any-to-ram-with-padding---some-data---first-pbo))))
                         (if-eq (force-bin (+ RZ_FIRST RZ_MDDL)) 1
                                (begin (will-inc! micro/TLO 1)
                                       (vanishes! (next micro/TBO))))
                         (if-eq RZ_MDDL 1 (eq! micro/INST MMIO_INST_RAM_VANISHES))
                         (if-eq RZ_LAST 1
                                (begin (eq! micro/INST MMIO_INST_RAM_EXCISION)
                                       (eq! micro/SIZE (any-to-ram-with-padding---some-data---last-padding-size))))))
