(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;   INVALID_CODE_PREFIX   ;;
;;;;;;;;;;;;;;;;;;;;;;;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (invalid-code-prefix---slo)    (next prprc/EUC_QUOT))
(defun    (invalid-code-prefix---sbo)    (next prprc/EUC_REM))

(defconstraint    invalid-code-prefix---setting-the-TOTs (:guard (* MACRO IS_INVALID_CODE_PREFIX))
                  (begin
                    (vanishes! TOTLZ)
                    (eq!       TOTNT NB_MICRO_ROWS_TOT_INVALID_CODE_PREFIX)
                    (vanishes! TOTRZ)))

(defconstraint    invalid-code-prefix---pre-processing (:guard (* MACRO IS_INVALID_CODE_PREFIX))
                  (begin  ;; setting tot nb of mmio inst
                    ;; setting prprc row n°1
                    (callToEuc 1
                               macro/SRC_OFFSET_LO
                               LLARGE)
                    (callToEq  1
                               0
                               (shift micro/LIMB NB_PP_ROWS_INVALID_CODE_PREFIX_PO)
                               EIP_3541_MARKER)))

(defconstraint    invalid-code-prefix---setting-the-success-bit (:guard (* MACRO IS_INVALID_CODE_PREFIX))
                  (begin  ;; setting tot nb of mmio inst
                    ;; setting the success bit
                    (eq! macro/SUCCESS_BIT
                         (next prprc/WCP_RES))))

(defconstraint    invalid-code-prefix---micro-instruction-writing (:guard (* MACRO IS_INVALID_CODE_PREFIX))
                  (begin  ;; setting tot nb of mmio inst
                    ;; setting first mmio inst
                    (eq! (shift micro/INST NB_PP_ROWS_INVALID_CODE_PREFIX_PO) MMIO_INST_RAM_TO_LIMB_ONE_SOURCE)
                    (eq! (shift micro/SIZE NB_PP_ROWS_INVALID_CODE_PREFIX_PO) 1)
                    (eq! (shift micro/SLO NB_PP_ROWS_INVALID_CODE_PREFIX_PO) (invalid-code-prefix---slo))
                    (eq! (shift micro/SBO NB_PP_ROWS_INVALID_CODE_PREFIX_PO) (invalid-code-prefix---sbo))
                    (eq! (shift micro/TBO NB_PP_ROWS_INVALID_CODE_PREFIX_PO) LLARGEMO)
                    (vanishes! (shift micro/EXO_SUM NB_PP_ROWS_INVALID_CODE_PREFIX_PO))
                    (eq! (shift micro/CN_S NB_PP_ROWS_INVALID_CODE_PREFIX_PO) macro/SRC_ID)))
