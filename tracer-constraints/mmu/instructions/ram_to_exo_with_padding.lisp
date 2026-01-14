(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;   RAM_TO_EXO_WITH_PADDING   ;;
;;;;;;;;;;;;;;;;;;;;;;;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (ram-to-exo-with-padding---aligned)                    (next prprc/WCP_RES))
(defun    (ram-to-exo-with-padding---initial-slo)                (next prprc/EUC_QUOT))
(defun    (ram-to-exo-with-padding---initial-sbo)                (next prprc/EUC_REM))
(defun    (ram-to-exo-with-padding---has-right-padding)          (shift prprc/WCP_RES 2))
(defun    (ram-to-exo-with-padding---padding-size)               (* (ram-to-exo-with-padding---has-right-padding) (- macro/REF_SIZE macro/SIZE)))
(defun    (ram-to-exo-with-padding---extraction-size)            (+ (* (ram-to-exo-with-padding---has-right-padding) macro/SIZE)
                                                                    (* (- 1 (ram-to-exo-with-padding---has-right-padding)) macro/REF_SIZE)))
(defun    (ram-to-exo-with-padding---last-limb-is-full)          (shift prprc/WCP_RES 3))
(defun    (ram-to-exo-with-padding---last-limb-byte-size)        (+ (* (ram-to-exo-with-padding---last-limb-is-full) LLARGE)
                                                                    (* (- 1 (ram-to-exo-with-padding---last-limb-is-full)) (shift prprc/EUC_REM 3))))
(defun    (ram-to-exo-with-padding---last-limb-single-source)    (shift prprc/WCP_RES 4))

(defconstraint    ram-to-exo-with-padding---setting-TOTLZ (:guard (* MACRO IS_RAM_TO_EXO_WITH_PADDING))
                  ;; setting nb of LEFT ZEROS rows
                  (vanishes! TOTLZ))

(defconstraint    ram-to-exo-with-padding---transferring-results-to-BIN-and-OUT-columns (:guard (* MACRO IS_RAM_TO_EXO_WITH_PADDING))
                  (begin
                    ;; setting bins and out
                    (eq! [BIN 1] (ram-to-exo-with-padding---aligned))
                    (eq! [OUT 1] (ram-to-exo-with-padding---last-limb-byte-size))
                    (eq! [BIN 2] (ram-to-exo-with-padding---last-limb-single-source))
                    (eq! [BIN 3] (ram-to-exo-with-padding---last-limb-is-full)))) ;; " "

(defconstraint    ram-to-exo-with-padding---1st-preprocessing-row (:guard (* MACRO IS_RAM_TO_EXO_WITH_PADDING))
                  (begin
                    ;; setting prprc row n°1
                    (callToEuc    1
                                  macro/SRC_OFFSET_LO
                                  LLARGE)
                    (callToIszero 1
                                  0
                                  (ram-to-exo-with-padding---initial-sbo))))

(defconstraint    ram-to-exo-with-padding---2nd-preprocessing-row (:guard (* MACRO IS_RAM_TO_EXO_WITH_PADDING))
                  (begin
                    ;; setting prprc row n°2
                    (callToLt     2
                                  0
                                  macro/SIZE
                                  macro/REF_SIZE)
                    (callToEuc    2
                                  (ram-to-exo-with-padding---padding-size)
                                  LLARGE)
                    (eq!    TOTRZ    (shift prprc/EUC_QUOT 2))))

(defconstraint    ram-to-exo-with-padding---3rd-preprocessing-row (:guard (* MACRO IS_RAM_TO_EXO_WITH_PADDING))
                  (begin
                    ;; setting prprc row n°3
                    (callToIszero 3
                                  0
                                  (shift prprc/EUC_REM 3))
                    (callToEuc    3
                                  (ram-to-exo-with-padding---extraction-size)
                                  LLARGE)
                    (eq!    TOTNT    (shift prprc/EUC_CEIL 3))))

(defconstraint    ram-to-exo-with-padding---4th-preprocessing-row (:guard (* MACRO IS_RAM_TO_EXO_WITH_PADDING))
                  (begin
                    ;; setting prprc row n°4
                    (callToLt     4
                                  0
                                  (+ (ram-to-exo-with-padding---initial-sbo) (- (ram-to-exo-with-padding---last-limb-byte-size) 1))
                                  LLARGE)))

(defconstraint    ram-to-exo-with-padding---preprocessing (:guard (* MACRO IS_RAM_TO_EXO_WITH_PADDING))
                  (begin
                    ;; setting mmio constant values
                    (eq!    (shift    micro/CN_S           NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)    macro/SRC_ID)
                    (eq!    (shift    micro/SUCCESS_BIT    NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)    macro/SUCCESS_BIT)
                    (eq!    (shift    micro/EXO_SUM        NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)    macro/EXO_SUM)
                    (eq!    (shift    micro/PHASE          NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)    macro/PHASE)
                    (eq!    (shift    micro/EXO_ID         NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)    macro/TGT_ID)
                    (eq!    (shift    micro/KEC_ID         NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)    macro/AUX_ID)
                    (eq!    (shift    micro/TOTAL_SIZE     NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)    macro/REF_SIZE)))

(defconstraint    ram-to-exo-with-padding---std-progression (:guard (* IS_RAM_TO_EXO_WITH_PADDING MICRO))
                  (begin
                    (standard-progression micro/TLO)
                    (vanishes! micro/TBO)))

(defconstraint    ram-to-exo-with-padding---first-row (:guard (* IS_RAM_TO_EXO_WITH_PADDING NT_FIRST))
                  (begin
                    (if-zero [BIN 1]
                             (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                             (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TRANSPLANT))
                    (eq! micro/SIZE LLARGE)
                    (eq! micro/SLO
                         (shift (ram-to-exo-with-padding---initial-slo) (- 0 NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)))
                    (eq! micro/SBO
                         (shift (ram-to-exo-with-padding---initial-sbo) (- 0 NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)))))

(defconstraint    ram-to-exo-with-padding---middle (:guard (* IS_RAM_TO_EXO_WITH_PADDING NT_MDDL))
                  (begin
                    (if-zero [BIN 1]
                             (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                             (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TRANSPLANT))
                    (eq! micro/SIZE LLARGE)
                    (did-inc! micro/SLO 1)
                    (remained-constant! micro/SBO)))

(defconstraint    ram-to-exo-with-padding---last (:guard (* IS_RAM_TO_EXO_WITH_PADDING NT_LAST))
                  (begin
                    (did-inc! micro/SLO 1)
                    (remained-constant! micro/SBO)))

(defconstraint    ram-to-exo-with-padding---only (:guard (* IS_RAM_TO_EXO_WITH_PADDING NT_ONLY))
                  (begin
                    (eq! micro/SLO
                         (shift (ram-to-exo-with-padding---initial-slo) (- 0 NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)))
                    (eq! micro/SBO
                         (shift (ram-to-exo-with-padding---initial-sbo) (- 0 NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)))))

(defconstraint    ram-to-exo-with-padding---last-or-only-common (:guard (* IS_RAM_TO_EXO_WITH_PADDING (force-bin (+ NT_LAST NT_ONLY))))
                  (begin
                    (eq! micro/SIZE [OUT 1])
                    (if-zero [BIN 2]
                             (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                             (if-zero [BIN 3]
                                      (eq! micro/INST MMIO_INST_RAM_TO_LIMB_ONE_SOURCE)
                                      (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TRANSPLANT)))))

(defconstraint    ram-to-exo-with-padding---right-zeroes (:guard (* IS_RAM_TO_EXO_WITH_PADDING (rzro-row)))
                  (eq! micro/INST MMIO_INST_LIMB_VANISHES))
