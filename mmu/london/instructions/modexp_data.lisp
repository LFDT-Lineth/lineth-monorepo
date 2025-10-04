(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;   MODEXP_DATA   ;;
;;;;;;;;;;;;;;;;;;;;;;;;                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (modexp-data---initial-tbo)                    [OUT 1])
(defun    (modexp-data---initial-slo)                    [OUT 2])
(defun    (modexp-data---initial-sbo)                    [OUT 3])
(defun    (modexp-data---first-limb-bytesize)            [OUT 4])
(defun    (modexp-data---last-limb-bytesize)             [OUT 5])
(defun    (modexp-data---first-limb-single-source)       [BIN 1])
(defun    (modexp-data---aligned)                        [BIN 2])
(defun    (modexp-data---last-limb-single-source)        [BIN 3])
(defun    (modexp-data---src-id)                         macro/SRC_ID)
(defun    (modexp-data---tgt-id)                         macro/TGT_ID)
(defun    (modexp-data---src-offset)                     macro/SRC_OFFSET_LO)
(defun    (modexp-data---size)                           macro/SIZE)
(defun    (modexp-data---cdo)                            macro/REF_OFFSET)
(defun    (modexp-data---cds)                            macro/REF_SIZE)
(defun    (modexp-data---exo-sum)                        macro/EXO_SUM)
(defun    (modexp-data---phase)                          macro/PHASE)
(defun    (modexp-data---param-byte-size)                (modexp-data---size))
(defun    (modexp-data---param-offset)                   (+ (modexp-data---cdo) (modexp-data---src-offset)))
(defun    (modexp-data---leftover-data-size)             (- (modexp-data---cds) (modexp-data---src-offset)))
(defun    (modexp-data---num-left-padding-bytes)         (- 512 (modexp-data---param-byte-size)))
(defun    (modexp-data---data-runs-out)                  (shift prprc/WCP_RES 2))
(defun    (modexp-data---num-right-padding-bytes)        (* (- (modexp-data---param-byte-size) (modexp-data---leftover-data-size)) (modexp-data---data-runs-out)))
(defun    (modexp-data---right-padding-remainder)        (shift prprc/EUC_REM 2))
(defun    (modexp-data---totnt-is-one)                   (shift prprc/WCP_RES 3))
(defun    (modexp-data---middle-sbo)                     (shift prprc/EUC_REM 6)) ;; ""


(defconstraint modexp-data---setting-TOT (:guard (* MACRO IS_MODEXP_DATA))
                 ;; Setting total number of mmio inst
                 (eq! TOT NB_MICRO_ROWS_TOT_MODEXP_DATA))

(defconstraint modexp-data---1st-preprocessing-row (:guard (* MACRO IS_MODEXP_DATA))
               (begin
                 ;; preprocessing row n°1
                 (callToEuc   1
                              (modexp-data---num-left-padding-bytes)
                              LLARGE)
                 (eq! (modexp-data---initial-tbo) (next prprc/EUC_REM))
                 (eq! TOTLZ (next prprc/EUC_QUOT))))

(defconstraint modexp-data---2nd-preprocessing-row (:guard (* MACRO IS_MODEXP_DATA))
               (begin
                 ;; preprocessing row n°2
                 (callToLt    2
                              0
                              (modexp-data---leftover-data-size)
                              (modexp-data---param-byte-size))
                 (callToEuc   2
                              (modexp-data---num-right-padding-bytes)
                              LLARGE)
                 (eq! TOTRZ (shift prprc/EUC_QUOT 2))
                 (debug (eq! TOTNT (- 32 (+ TOTLZ TOTRZ))))))

(defconstraint modexp-data---3rd-preprocessing-row (:guard (* MACRO IS_MODEXP_DATA))
               (begin
                 ;; preprocessing row n°3
                 (callToEq    3
                              0
                              TOTNT
                              1)
                 (callToEuc   3
                              (modexp-data---param-offset)
                              LLARGE)
                 (eq! (modexp-data---initial-slo) (shift prprc/EUC_QUOT 3))
                 (eq! (modexp-data---initial-sbo) (shift prprc/EUC_REM 3))
                 (if-zero (modexp-data---totnt-is-one)
                          (eq! (modexp-data---first-limb-bytesize) (- LLARGE (modexp-data---initial-tbo)))
                          (if-zero (modexp-data---data-runs-out)
                                   (eq! (modexp-data---first-limb-bytesize) (modexp-data---param-byte-size))
                                   (eq! (modexp-data---first-limb-bytesize) (modexp-data---leftover-data-size))))
                 (if-zero (modexp-data---data-runs-out)
                          (eq! (modexp-data---last-limb-bytesize) LLARGE)
                          (eq! (modexp-data---last-limb-bytesize) (- LLARGE (modexp-data---right-padding-remainder))))))

(defconstraint modexp-data---4th-preprocessing-row (:guard (* MACRO IS_MODEXP_DATA))
               (begin
                 ;; preprocessing row n°4
                 (callToLt    4
                              0
                              (+ (modexp-data---initial-sbo) (- (modexp-data---first-limb-bytesize) 1))
                              LLARGE)
                 (eq! (modexp-data---first-limb-single-source) (shift prprc/WCP_RES 4))))

(defconstraint modexp-data---5th-preprocessing-row (:guard (* MACRO IS_MODEXP_DATA))
               (begin
                 ;; preprocessing row n°5
                 (callToEq    5
                              0
                              (modexp-data---initial-sbo)
                              (modexp-data---initial-tbo))
                 (eq! (modexp-data---aligned) (shift prprc/WCP_RES 5))))

(defconstraint modexp-data---6th-preprocessing-row (:guard (* MACRO IS_MODEXP_DATA))
               (begin
                 ;; preprocessing row n°6
                 (if-eq-else (modexp-data---aligned) 1
                             (eq! (modexp-data---last-limb-single-source) (modexp-data---aligned))
                             (begin (callToEuc    6
                                                  (+ (modexp-data---initial-sbo) (modexp-data---first-limb-bytesize))
                                                  LLARGE)
                                    (callToLt     6
                                                  0
                                                  (+ (modexp-data---middle-sbo) (- (modexp-data---last-limb-bytesize) 1))
                                                  LLARGE)
                                    (eq! (modexp-data---last-limb-single-source) (shift prprc/WCP_RES 6))))))

(defconstraint modexp-data---setting-micro-instruction-constant-values (:guard (* MACRO IS_MODEXP_DATA))
               (begin
                 ;; setting mmio constant values
                 (eq! (shift micro/CN_S NB_PP_ROWS_MODEXP_DATA_PO) (modexp-data---src-id))
                 (eq! (shift micro/EXO_SUM NB_PP_ROWS_MODEXP_DATA_PO) EXO_SUM_WEIGHT_BLAKEMODEXP)
                 (eq! (shift micro/PHASE NB_PP_ROWS_MODEXP_DATA_PO) (modexp-data---phase))
                 (eq! (shift micro/EXO_ID NB_PP_ROWS_MODEXP_DATA_PO) (modexp-data---tgt-id))))

(defconstraint modexp-data---mmio-instruction-writting (:guard IS_MODEXP_DATA)
               (begin (if-eq MICRO 1 (standard-progression micro/TLO))
                      (if-eq (zero-row) 1 (eq! micro/INST MMIO_INST_LIMB_VANISHES))
                      (if-eq (force-bin (+ NT_ONLY NT_FIRST)) 1
                             (begin (if-zero (modexp-data---first-limb-single-source)
                                             (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                                             (eq! micro/INST MMIO_INST_RAM_TO_LIMB_ONE_SOURCE))
                                    (eq! micro/SIZE (modexp-data---first-limb-bytesize))
                                    (eq! micro/SLO (modexp-data---initial-slo))
                                    (eq! micro/SBO (modexp-data---initial-sbo))
                                    (eq! micro/TBO (modexp-data---initial-tbo))))
                      (if-eq NT_FIRST 1
                             (begin (if-eq-else (modexp-data---aligned) 1
                                                (will-inc! micro/SLO 1)
                                                (if-zero (modexp-data---first-limb-single-source)
                                                         (begin (will-inc! micro/SLO 1)
                                                                (will-eq! micro/SBO
                                                                          (- (+ micro/SBO micro/SIZE) LLARGE)))
                                                         (begin (will-remain-constant! micro/SLO)
                                                                (will-eq! micro/SBO (+ micro/SBO micro/SIZE)))))
                                    (vanishes! (next micro/TBO))))
                      (if-eq NT_MDDL 1
                             (begin (if-zero (modexp-data---aligned)
                                             (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                                             (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TRANSPLANT))
                                    (eq! micro/SIZE LLARGE)
                                    (will-inc! micro/SLO 1)
                                    (will-remain-constant! micro/SBO)
                                    (will-remain-constant! micro/TBO)))
                      (if-eq NT_LAST 1
                             (begin (if-zero (modexp-data---last-limb-single-source)
                                             (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                                             (eq! micro/INST MMIO_INST_RAM_TO_LIMB_ONE_SOURCE))
                                    (eq! micro/SIZE (modexp-data---last-limb-bytesize))))))

