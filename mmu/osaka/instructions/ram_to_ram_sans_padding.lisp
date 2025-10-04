(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;   RAM_TO_RAM_SANS_PADDING   ;;
;;;;;;;;;;;;;;;;;;;;;;;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (ram-to-ram-sans-padding---last-limb-byte-size)           [OUT 1])
(defun    (ram-to-ram-sans-padding---middle-sbo)                    [OUT 2])
(defun    (ram-to-ram-sans-padding---aligned)                       [BIN 1])
(defun    (ram-to-ram-sans-padding---last-limb-single-source)       [BIN 2])
(defun    (ram-to-ram-sans-padding---initial-slo-increment)         [BIN 3])
(defun    (ram-to-ram-sans-padding---last-limb-is-fast)             [BIN 4])
(defun    (ram-to-ram-sans-padding---rdo)                           macro/SRC_OFFSET_LO)
(defun    (ram-to-ram-sans-padding---rds)                           macro/SIZE)
(defun    (ram-to-ram-sans-padding---rato)                          macro/REF_OFFSET)
(defun    (ram-to-ram-sans-padding---ratc)                          macro/REF_SIZE)
(defun    (ram-to-ram-sans-padding---initial-slo)                   (next prprc/EUC_QUOT))
(defun    (ram-to-ram-sans-padding---initial-sbo)                   (next prprc/EUC_REM))
(defun    (ram-to-ram-sans-padding---initial-cmp)                   (next prprc/WCP_RES))
(defun    (ram-to-ram-sans-padding---initial-real-size)             (+    (* (ram-to-ram-sans-padding---initial-cmp) (ram-to-ram-sans-padding---ratc))
                                                                          (* (- 1 (ram-to-ram-sans-padding---initial-cmp)) (ram-to-ram-sans-padding---rds))))
(defun    (ram-to-ram-sans-padding---initial-tlo)                   (shift prprc/EUC_QUOT 2))
(defun    (ram-to-ram-sans-padding---initial-tbo)                   (shift prprc/EUC_REM 2))
(defun    (ram-to-ram-sans-padding---final-tlo)                     (shift prprc/EUC_QUOT 3))
(defun    (ram-to-ram-sans-padding---totnt-is-one)                  (shift prprc/WCP_RES 3))
(defun    (ram-to-ram-sans-padding---first-limb-byte-size)          (+    (* (ram-to-ram-sans-padding---totnt-is-one) (ram-to-ram-sans-padding---initial-real-size))
                                                                          (* (- 1 (ram-to-ram-sans-padding---totnt-is-one)) (- LLARGE (ram-to-ram-sans-padding---initial-tbo)))))
(defun    (ram-to-ram-sans-padding---first-limb-single-source)      (shift prprc/WCP_RES 4))
(defun    (ram-to-ram-sans-padding---init-tbo-is-zero)              (shift prprc/WCP_RES 5))
(defun    (ram-to-ram-sans-padding---last-limb-is-full)             (force-bin (shift prprc/EUC_QUOT 5)))
(defun    (ram-to-ram-sans-padding---first-limb-is-fast)            (force-bin (* (ram-to-ram-sans-padding---aligned) (ram-to-ram-sans-padding---init-tbo-is-zero)))) ;;""

(defconstraint    ram-to-ram-sans-padding---setting-some-TOTs (:guard (* MACRO IS_RAM_TO_RAM_SANS_PADDING))
                  (begin
                    ;; set nb of rows
                    (vanishes! TOTLZ)
                    (vanishes! TOTRZ)))

(defconstraint    ram-to-ram-sans-padding---1st-preprocessing-row (:guard (* MACRO IS_RAM_TO_RAM_SANS_PADDING))
                  (begin
                    ;; preprocessing row n°1
                    (callToEuc 1
                               (ram-to-ram-sans-padding---rdo)
                               LLARGE)
                    (callToLt  1
                               0
                               (ram-to-ram-sans-padding---ratc)
                               (ram-to-ram-sans-padding---rds))))

(defconstraint    ram-to-ram-sans-padding---2nd-preprocessing-row (:guard (* MACRO IS_RAM_TO_RAM_SANS_PADDING))
                  (begin
                    ;; preprocessing row n°2
                    (callToEuc 2
                               (ram-to-ram-sans-padding---rato)
                               LLARGE)
                    (callToEq  2
                               0
                               (ram-to-ram-sans-padding---initial-sbo)
                               (ram-to-ram-sans-padding---initial-tbo))
                    (eq! (ram-to-ram-sans-padding---aligned) (shift prprc/WCP_RES 2))))

(defconstraint    ram-to-ram-sans-padding---3rd-preprocessing-row (:guard (* MACRO IS_RAM_TO_RAM_SANS_PADDING))
                  (begin
                    ;; preprocessing row n°3
                    (callToEuc 3
                               (+ (ram-to-ram-sans-padding---rato) (- (ram-to-ram-sans-padding---initial-real-size) 1))
                               LLARGE)
                    (callToEq  3
                               0
                               TOTNT
                               1)
                    (eq! TOTNT
                         (+ (- (ram-to-ram-sans-padding---final-tlo) (ram-to-ram-sans-padding---initial-tlo)) 1))
                    (if-zero (ram-to-ram-sans-padding---totnt-is-one)
                             (eq! (ram-to-ram-sans-padding---last-limb-byte-size)
                                  (+ 1 (shift prprc/EUC_REM 3)))
                             (eq! (ram-to-ram-sans-padding---last-limb-byte-size) (ram-to-ram-sans-padding---initial-real-size)))))

(defconstraint    ram-to-ram-sans-padding---4th-preprocessing-row (:guard (* MACRO IS_RAM_TO_RAM_SANS_PADDING))
                  (begin
                    ;; preprocessing row n°4
                    (callToLt   4
                                0
                                (+ (ram-to-ram-sans-padding---initial-sbo) (- (ram-to-ram-sans-padding---first-limb-byte-size) 1))
                                LLARGE)
                    (callToEuc  4
                                (+ (ram-to-ram-sans-padding---middle-sbo) (- (ram-to-ram-sans-padding---last-limb-byte-size) 1))
                                LLARGE)
                    (if-zero (ram-to-ram-sans-padding---aligned)
                             (if-eq-else (ram-to-ram-sans-padding---first-limb-single-source) 1
                                         (eq! (ram-to-ram-sans-padding---middle-sbo)
                                              (+ (ram-to-ram-sans-padding---initial-sbo)
                                                 (ram-to-ram-sans-padding---first-limb-byte-size)))
                                         (eq! (ram-to-ram-sans-padding---middle-sbo)
                                              (- (+ (ram-to-ram-sans-padding---initial-sbo)
                                                    (ram-to-ram-sans-padding---first-limb-byte-size))
                                                 LLARGE))))
                    (if-eq-else (ram-to-ram-sans-padding---totnt-is-one) 1
                                (eq! (ram-to-ram-sans-padding---last-limb-single-source)
                                     (ram-to-ram-sans-padding---first-limb-single-source))
                                (eq! (ram-to-ram-sans-padding---last-limb-single-source)
                                     (force-bin (- 1 (shift prprc/EUC_QUOT 4)))))
                    (if-eq-else (ram-to-ram-sans-padding---aligned) 1
                                (eq! (ram-to-ram-sans-padding---initial-slo-increment) 1)
                                (eq! (ram-to-ram-sans-padding---initial-slo-increment)
                                     (- 1 (ram-to-ram-sans-padding---first-limb-single-source))))))

(defconstraint    ram-to-ram-sans-padding---5th-preprocessing-row (:guard (* MACRO IS_RAM_TO_RAM_SANS_PADDING))
                  (begin
                    ;; preprocessing row n°5
                    (callToIszero 5
                                  0
                                  (ram-to-ram-sans-padding---initial-tbo))
                    (callToEuc    5
                                  (ram-to-ram-sans-padding---last-limb-byte-size)
                                  LLARGE)
                    (eq! (ram-to-ram-sans-padding---last-limb-is-fast)
                         (* (ram-to-ram-sans-padding---aligned) (ram-to-ram-sans-padding---last-limb-is-full)))))

(defconstraint    ram-to-ram-sans-padding---constant-mmio-values (:guard (* MACRO IS_RAM_TO_RAM_SANS_PADDING))
                  (begin (eq! (shift micro/CN_S NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO) macro/SRC_ID)
                         (eq! (shift micro/CN_T NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO) macro/TGT_ID)))

(defconstraint    ram-to-ram-sans-padding---first-mmio-values (:guard (* MACRO IS_RAM_TO_RAM_SANS_PADDING))
                  (begin (eq! (shift micro/SIZE NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO)
                              (ram-to-ram-sans-padding---first-limb-byte-size))
                         (eq! (shift micro/SLO NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO) (ram-to-ram-sans-padding---initial-slo))
                         (eq! (shift micro/SBO NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO) (ram-to-ram-sans-padding---initial-sbo))
                         (eq! (shift micro/TLO NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO) (ram-to-ram-sans-padding---initial-tlo))
                         (eq! (shift micro/TBO NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO) (ram-to-ram-sans-padding---initial-tbo))))

(defconstraint    ram-to-ram-sans-padding---mmio-instruction-writting (:guard IS_RAM_TO_RAM_SANS_PADDING)
                  (begin (if-eq (force-bin (+ NT_FIRST NT_MDDL)) 1
                                (will-inc! micro/TLO 1))
                         (if-eq NT_FIRST 1
                                (eq! (next micro/SLO) (+ micro/SLO (ram-to-ram-sans-padding---initial-slo-increment))))
                         (if-eq NT_MDDL 1 (will-inc! micro/SLO 1))
                         (if-eq NT_ONLY 1
                                (if-zero (ram-to-ram-sans-padding---last-limb-is-fast)
                                         (if-zero (ram-to-ram-sans-padding---last-limb-single-source)
                                                  (eq! micro/INST MMIO_INST_RAM_TO_RAM_TWO_SOURCE)
                                                  (eq! micro/INST MMIO_INST_RAM_TO_RAM_PARTIAL))
                                         (eq! micro/INST MMIO_INST_RAM_TO_RAM_TRANSPLANT)))
                         (if-eq NT_FIRST 1
                                (if-zero (shift (ram-to-ram-sans-padding---first-limb-is-fast)
                                                (- 0 NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO))
                                         (if-zero (shift (ram-to-ram-sans-padding---first-limb-single-source)
                                                         (- 0 NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO))
                                                  (eq! micro/INST MMIO_INST_RAM_TO_RAM_TWO_SOURCE)
                                                  (eq! micro/INST MMIO_INST_RAM_TO_RAM_PARTIAL))
                                         (eq! micro/INST MMIO_INST_RAM_TO_RAM_TRANSPLANT)))
                         (if-eq NT_MDDL 1
                                (begin (if-eq-else (ram-to-ram-sans-padding---aligned) 1
                                                   (eq! micro/INST MMIO_INST_RAM_TO_RAM_TRANSPLANT)
                                                   (eq! micro/INST MMIO_INST_RAM_TO_RAM_TWO_SOURCE))
                                       (eq! micro/SIZE LLARGE)
                                       (eq! micro/SBO (ram-to-ram-sans-padding---middle-sbo))
                                       (vanishes! micro/TBO)))
                         (if-eq NT_LAST 1
                                (begin (if-eq-else (ram-to-ram-sans-padding---last-limb-is-fast) 1
                                                   (eq! micro/INST MMIO_INST_RAM_TO_RAM_TRANSPLANT)
                                                   (if-zero (ram-to-ram-sans-padding---last-limb-single-source)
                                                            (eq! micro/INST MMIO_INST_RAM_TO_RAM_TWO_SOURCE)
                                                            (eq! micro/INST MMIO_INST_RAM_TO_RAM_PARTIAL)))
                                       (eq! micro/SIZE (ram-to-ram-sans-padding---last-limb-byte-size))
                                       (eq! micro/SBO (ram-to-ram-sans-padding---middle-sbo))
                                       (vanishes! micro/TBO)))))
