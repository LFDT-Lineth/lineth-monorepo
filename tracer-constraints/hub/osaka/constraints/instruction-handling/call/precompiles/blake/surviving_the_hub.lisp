(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;    X.Y.22.5  Surviving the HUB    ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; Miscellaneous-row i + 2 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---BLAKE2f---surviving-the-HUB)    (*    PEEK_AT_SCENARIO
                                                                         scenario/PRC_BLAKE2f
                                                                         (-    1    scenario/PRC_FAILURE_KNOWN_TO_HUB)))


(defconstraint    precompile-processing---BLAKE2f---setting-second-MISC-flags
                  (:guard    (precompile-processing---BLAKE2f---surviving-the-HUB))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!    (weighted-MISC-flag-sum-sans-MMU    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-call-data-extraction)    MISC_WEIGHT_OOB)
                    (eq!    (shift    misc/MMU_FLAG             precompile-processing---BLAKE2f---misc-row-offset---BLAKE-call-data-extraction)    (precompile-processing---BLAKE2f---OOB-ram-success))
                    ))


(defconstraint    precompile-processing---BLAKE2f---setting-the-second-OOB-instruction
                  (:guard    (precompile-processing---BLAKE2f---surviving-the-HUB))
                  (set-OOB-instruction---blake-params    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-call-data-extraction    ;; offset
                                                         (precompile-processing---dup-callee-gas)                                          ;; call gas i.e. gas provided to the precompile
                                                         (precompile-processing---BLAKE2f---r-parameter)                                   ;; rounds parameter of the call data of BLAKE2f
                                                         (precompile-processing---BLAKE2f---f-parameter)                                   ;; f      parameter of the call data of BLAKE2f ("final block indicator")
                                                         ))

(defconstraint    precompile-processing---BLAKE2f---setting-the-second-MMU-instruction
                  (:guard    (precompile-processing---BLAKE2f---surviving-the-HUB))
                  (if-not-zero    (shift    misc/MMU_FLAG                           precompile-processing---BLAKE2f---misc-row-offset---BLAKE-call-data-extraction)
                                  (set-MMU-instruction---ram-to-exo-with-padding    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-call-data-extraction      ;; offset
                                                                                    CONTEXT_NUMBER
                                                                                    (+    1    HUB_STAMP)                                                               ;; target ID
                                                                                    0                                                                                   ;; auxiliary ID
                                                                                    ;; src_offset_hi                                                                       ;; source offset high
                                                                                    (+    (precompile-processing---dup-cdo)    4)                                       ;; source offset low
                                                                                    ;; tgt_offset_lo                                                                       ;; target offset low
                                                                                    208                                                                                 ;; size
                                                                                    ;; ref_offset                                                                          ;; reference offset
                                                                                    208                                                                                 ;; reference size
                                                                                    0                                                                                   ;; success bit
                                                                                    ;; limb_1                                                                              ;; limb 1
                                                                                    ;; limb_2                                                                              ;; limb 2
                                                                                    EXO_SUM_WEIGHT_BLAKEMODEXP                                                          ;; weighted exogenous module flag sum
                                                                                    PHASE_BLAKE_DATA                                                                    ;; phase
                                                                                    )))

(defconstraint    precompile-processing---BLAKE2f---setting-FAILURE_KNOWN_TO_RAM
                  (:guard    (precompile-processing---BLAKE2f---surviving-the-HUB))
                  (begin
                    (eq!    (scenario-shorthand---PRC---success)    (precompile-processing---BLAKE2f---OOB-ram-success))
                    (eq!     scenario/PRC_FAILURE_KNOWN_TO_RAM      (-    1    (precompile-processing---BLAKE2f---OOB-ram-success)))))

(defconstraint    precompile-processing---BLAKE2f---justifying-the-return-gas-prediction
                  (:guard    (precompile-processing---BLAKE2f---surviving-the-HUB))
                  (eq!       (precompile-processing---prd-return-gas)
                             (*    (scenario-shorthand---PRC---success)    (precompile-processing---BLAKE2f---OOB-return-gas))))

(defun    (precompile-processing---BLAKE2f---OOB-ram-success)    (shift    [misc/OOB_DATA   4]    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-call-data-extraction))
(defun    (precompile-processing---BLAKE2f---OOB-return-gas)     (shift    [misc/OOB_DATA   5]    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-call-data-extraction))
