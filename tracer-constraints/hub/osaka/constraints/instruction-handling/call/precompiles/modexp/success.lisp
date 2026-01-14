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
;;    X.Y.Z.4 MODEXP success case    ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---MODEXP---success-case)    (*    PEEK_AT_SCENARIO
                                                                   scenario/PRC_MODEXP
                                                                   (scenario-shorthand---PRC---success)))

(defun    (lets-extract-the-base)            (shift    misc/MMU_FLAG    precompile-processing---MODEXP---misc-row-offset---base-extraction))
(defun    (lets-extract-the-exponent)        (shift    misc/MMU_FLAG    precompile-processing---MODEXP---misc-row-offset---exponent-extraction))
(defun    (lets-extract-the-modulus)         (shift    misc/MMU_FLAG    precompile-processing---MODEXP---misc-row-offset---modulus-extraction))
(defun    (lets-extract-the-full-result)     (shift    misc/MMU_FLAG    precompile-processing---MODEXP---misc-row-offset---full-result-extraction))
(defun    (lets-partially-copy-the-result)   (shift    misc/MMU_FLAG    precompile-processing---MODEXP---misc-row-offset---partial-result-transfer))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    deciding what byte sizes to extract and potentially extracting the base    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---success-case---base-extraction-row---setting-module-flags
                  (:guard    (precompile-processing---MODEXP---success-case))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!    (weighted-MISC-flag-sum-sans-MMU    precompile-processing---MODEXP---misc-row-offset---base-extraction)    MISC_WEIGHT_OOB)
                    (eq!    (shift    misc/MMU_FLAG             precompile-processing---MODEXP---misc-row-offset---base-extraction)    (precompile-processing---MODEXP---extract-modulus))
                    ))

(defconstraint    precompile-processing---MODEXP---success-case---base-extraction-row---setting-the-OOB-instruction-which-decides-which-actual-parameters-to-extract      (:guard    (precompile-processing---MODEXP---success-case))
                  (set-OOB-instruction---modexp-extract    precompile-processing---MODEXP---misc-row-offset---base-extraction    ;; offset
                                                         (precompile-processing---dup-cds)                                     ;; call data size
                                                         (precompile-processing---MODEXP---bbs-normalized)                     ;; low part of bbs (base     byte size)
                                                         (precompile-processing---MODEXP---ebs-normalized)                     ;; low part of ebs (exponent byte size)
                                                         (precompile-processing---MODEXP---mbs-normalized)                     ;; low part of mbs (modulus  byte size)
                                                         ))

;; Note: we deduce some shorthands AT THE END OF THE FILE.

(defconstraint    precompile-processing---MODEXP---success-case---base-extraction-row---setting-the-MMU-instruction    (:guard    (precompile-processing---MODEXP---success-case))
                  (if-not-zero    (lets-extract-the-base)
                                  (if-zero    (force-bin    (precompile-processing---MODEXP---extract-base))
                                              ;; extract_base == 0 case:
                                              (set-MMU-instruction---modexp-zero    precompile-processing---MODEXP---misc-row-offset---base-extraction     ;; offset
                                                                                    ;; src_id                                                                 ;; source ID
                                                                                    (+    1    HUB_STAMP)                                                  ;; target ID
                                                                                    ;; aux_id                                                                 ;; auxiliary ID
                                                                                    ;; src_offset_hi                                                          ;; source offset high
                                                                                    ;; src_offset_lo                                                          ;; source offset low
                                                                                    ;; tgt_offset_lo                                                          ;; target offset low
                                                                                    ;; size                                                                   ;; size
                                                                                    ;; ref_offset                                                             ;; reference offset
                                                                                    ;; ref_size                                                               ;; reference size
                                                                                    ;; success_bit                                                            ;; success bit
                                                                                    ;; limb_1                                                                 ;; limb 1
                                                                                    ;; limb_2                                                                 ;; limb 2
                                                                                    ;; exo_sum                                                                ;; weighted exogenous module flag sum
                                                                                    PHASE_MODEXP_BASE                                                      ;; phase
                                                                                    )
                                              ;; extract_base == 1 case:
                                              (set-MMU-instruction---modexp-data    precompile-processing---MODEXP---misc-row-offset---base-extraction     ;; offset
                                                                                  CONTEXT_NUMBER                                                         ;; source ID
                                                                                  (+    1    HUB_STAMP)                                                  ;; target ID
                                                                                  ;; aux_id                                                                 ;; auxiliary ID
                                                                                  ;; src_offset_hi                                                          ;; source offset high
                                                                                  96                                                                     ;; source offset low
                                                                                  ;; tgt_offset_lo                                                          ;; target offset low
                                                                                  (precompile-processing---MODEXP---bbs-normalized)                      ;; size
                                                                                  (precompile-processing---dup-cdo)                                      ;; reference offset
                                                                                  (precompile-processing---dup-cds)                                      ;; reference size
                                                                                  ;; success_bit                                                            ;; success bit
                                                                                  ;; limb_1                                                                 ;; limb 1
                                                                                  ;; limb_2                                                                 ;; limb 2
                                                                                  ;; exo_sum                                                                ;; weighted exogenous module flag sum
                                                                                  PHASE_MODEXP_BASE                                                      ;; phase
                                                                                  ))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    potentially extracting the exponent    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---success-case---exponent-extraction-row---setting-module-flags    (:guard    (precompile-processing---MODEXP---success-case))
                  (eq!   (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---exponent-extraction)
                         (*    MISC_WEIGHT_MMU     (precompile-processing---MODEXP---extract-modulus))
                         ))

(defconstraint    precompile-processing---MODEXP---success-case---exponent-extraction-row---setting-the-MMU-instruction    (:guard    (precompile-processing---MODEXP---success-case))
                  (if-not-zero    (lets-extract-the-exponent)
                                  (if-zero    (force-bin    (precompile-processing---MODEXP---extract-exponent))
                                              ;; extract_exponent == 0 case:
                                              (set-MMU-instruction---modexp-zero    precompile-processing---MODEXP---misc-row-offset---exponent-extraction     ;; offset
                                                                                    ;; src_id                                                                     ;; source ID
                                                                                    (+    1    HUB_STAMP)                                                      ;; target ID
                                                                                    ;; aux_id                                                                     ;; auxiliary ID
                                                                                    ;; src_offset_hi                                                              ;; source offset high
                                                                                    ;; src_offset_lo                                                              ;; source offset low
                                                                                    ;; tgt_offset_lo                                                              ;; target offset low
                                                                                    ;; size                                                                       ;; size
                                                                                    ;; ref_offset                                                                 ;; reference offset
                                                                                    ;; ref_size                                                                   ;; reference size
                                                                                    ;; success_bit                                                                ;; success bit
                                                                                    ;; limb_1                                                                     ;; limb 1
                                                                                    ;; limb_2                                                                     ;; limb 2
                                                                                    ;; exo_sum                                                                    ;; weighted exogenous module flag sum
                                                                                    PHASE_MODEXP_EXPONENT                                                      ;; phase
                                                                                    )
                                              ;; extract_exponent == 1 case:
                                              (set-MMU-instruction---modexp-data    precompile-processing---MODEXP---misc-row-offset---exponent-extraction     ;; offset
                                                                                  CONTEXT_NUMBER                                                             ;; source ID
                                                                                  (+    1    HUB_STAMP)                                                      ;; target ID
                                                                                  ;; aux_id                                                                     ;; auxiliary ID
                                                                                  ;; src_offset_hi                                                              ;; source offset high
                                                                                  (+    96    (precompile-processing---MODEXP---bbs-normalized))             ;; source offset low
                                                                                  ;; tgt_offset_lo                                                              ;; target offset low
                                                                                  (precompile-processing---MODEXP---ebs-normalized)                          ;; size
                                                                                  (precompile-processing---dup-cdo)                                          ;; reference offset
                                                                                  (precompile-processing---dup-cds)                                          ;; reference size
                                                                                  ;; success_bit                                                                ;; success bit
                                                                                  ;; limb_1                                                                     ;; limb 1
                                                                                  ;; limb_2                                                                     ;; limb 2
                                                                                  ;; exo_sum                                                                    ;; weighted exogenous module flag sum
                                                                                  PHASE_MODEXP_EXPONENT                                                      ;; phase
                                                                                  ))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    potentially extracting the modulus    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---success-case---modulus-extraction-row---setting-module-flags    (:guard    (precompile-processing---MODEXP---success-case))
                  (eq!   (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---modulus-extraction)
                         (*    MISC_WEIGHT_MMU     (precompile-processing---MODEXP---extract-modulus))
                         ))

(defconstraint    precompile-processing---MODEXP---success-case---modulus-extraction-row---setting-the-MMU-instruction    (:guard    (precompile-processing---MODEXP---success-case))
                  (if-not-zero    (lets-extract-the-modulus)
                                  ;; extract_modulus == 1 case:
                                  (set-MMU-instruction---modexp-data    precompile-processing---MODEXP---misc-row-offset---modulus-extraction                               ;; offset
                                                                        CONTEXT_NUMBER                                                                                      ;; source ID
                                                                        (+   1   HUB_STAMP)                                                                                 ;; target ID
                                                                        ;; aux_id                                                                                              ;; auxiliary ID
                                                                        ;; src_offset_hi                                                                                       ;; source offset high
                                                                        (+   96
                                                                             (precompile-processing---MODEXP---bbs-normalized)
                                                                             (precompile-processing---MODEXP---ebs-normalized))                                             ;; source offset low
                                                                        ;; tgt_offset_lo                                                                                       ;; target offset low
                                                                        (precompile-processing---MODEXP---mbs-normalized)                                                   ;; size
                                                                        (precompile-processing---dup-cdo)                                                                   ;; reference offset
                                                                        (precompile-processing---dup-cds)                                                                   ;; reference size
                                                                        ;; success_bit                                                                                         ;; success bit
                                                                        ;; limb_1                                                                                              ;; limb 1
                                                                        ;; limb_2                                                                                              ;; limb 2
                                                                        ;; exo_sum                                                                                             ;; weighted exogenous module flag sum
                                                                        PHASE_MODEXP_MODULUS                                                                                ;; phase
                                                                        )))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    full copy of result to dedicated RAM segment    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---success-case---full-result-extraction-row---set-module-flags    (:guard    (precompile-processing---MODEXP---success-case))
                  (eq!   (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---full-result-extraction)
                         (*    MISC_WEIGHT_MMU     (precompile-processing---MODEXP---extract-modulus))
                         ))

(defconstraint    precompile-processing---MODEXP---success-case---full-result-extraction-row---setting-the-MMU-instruction    (:guard    (precompile-processing---MODEXP---success-case))
                  (if-not-zero    (lets-extract-the-full-result)
                                  (set-MMU-instruction---exo-to-ram-transplants    precompile-processing---MODEXP---misc-row-offset---full-result-extraction    ;; offset
                                                                                   (+    1    HUB_STAMP)                                                        ;; source ID
                                                                                   (+    1    HUB_STAMP)                                                        ;; target ID
                                                                                   ;; aux_id                                                                       ;; auxiliary ID
                                                                                   ;; src_offset_hi                                                                ;; source offset high
                                                                                   ;; src_offset_lo                                                                ;; source offset low
                                                                                   ;; tgt_offset_lo                                                                ;; target offset low
                                                                                   EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND                                        ;; size
                                                                                   ;; ref_offset                                                                   ;; reference offset
                                                                                   ;; ref_size                                                                     ;; reference size
                                                                                   ;; success_bit                                                                  ;; success bit
                                                                                   ;; limb_1                                                                       ;; limb 1
                                                                                   ;; limb_2                                                                       ;; limb 2
                                                                                   EXO_SUM_WEIGHT_BLAKEMODEXP                                                   ;; weighted exogenous module flag sum
                                                                                   PHASE_MODEXP_RESULT                                                          ;; phase
                                                                                   )))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    partial copy of results to current RAM segment    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---success-case---partial-result-copy-row---set-module-flags    (:guard    (precompile-processing---MODEXP---success-case))
                  (eq!   (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---partial-result-transfer)
                         (*    MISC_WEIGHT_MMU
                               (precompile-processing---MODEXP---mbs-nonzero)
                               (precompile-processing---MODEXP---r@c-nonzero)
                               )))

(defconstraint    precompile-processing---MODEXP---success-case---partial-result-copy-row---setting-the-MMU-instruction    (:guard    (precompile-processing---MODEXP---success-case))
                  (if-not-zero    (lets-partially-copy-the-result)
                                  (set-MMU-instruction---ram-to-ram-sans-padding    precompile-processing---MODEXP---misc-row-offset---partial-result-transfer    ;; offset
                                                                                    (+    1    HUB_STAMP)                                                         ;; source ID
                                                                                    CONTEXT_NUMBER                                                                ;; target ID
                                                                                    ;; aux_id                                                                        ;; auxiliary ID
                                                                                    ;; src_offset_hi                                                                 ;; source offset high
                                                                                    (-    EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND
                                                                                          (precompile-processing---MODEXP---mbs-normalized))               ;; source offset low
                                                                                    ;; tgt_offset_lo                                                                 ;; target offset low
                                                                                    (precompile-processing---MODEXP---mbs-normalized)                             ;; size
                                                                                    (precompile-processing---dup-r@o)                                             ;; reference offset
                                                                                    (precompile-processing---dup-r@c)                                             ;; reference size
                                                                                    ;; success_bit                                                                   ;; success bit
                                                                                    ;; limb_1                                                                        ;; limb 1
                                                                                    ;; limb_2                                                                        ;; limb 2
                                                                                    ;; exo_sum                                                                       ;; weighted exogenous module flag sum
                                                                                    ;; phase                                                                         ;; phase
                                                                                    )))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    updating current context's return data    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---success-case---return-data-update-row    (:guard    (precompile-processing---MODEXP---success-case))
                  (provide-return-data     precompile-processing---MODEXP---context-row-offset---success   ;; row offset
                                           CONTEXT_NUMBER                                                  ;; receiver context
                                           (+    1    HUB_STAMP)                                           ;; provider context
                                           (-    EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND
                                                 (precompile-processing---MODEXP---mbs-normalized))        ;; rdo
                                           (precompile-processing---MODEXP---mbs-normalized)               ;; rds
                                           ))


;; setting some shorthands
(defun    (precompile-processing---MODEXP---extract-base)        (shift    [ misc/OOB_DATA   6 ]    precompile-processing---MODEXP---misc-row-offset---base-extraction))
(defun    (precompile-processing---MODEXP---extract-exponent)    (shift    [ misc/OOB_DATA   7 ]    precompile-processing---MODEXP---misc-row-offset---base-extraction))
(defun    (precompile-processing---MODEXP---extract-modulus)     (shift    [ misc/OOB_DATA   8 ]    precompile-processing---MODEXP---misc-row-offset---base-extraction))
