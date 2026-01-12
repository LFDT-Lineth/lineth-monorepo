(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                        ;;
;;    X.Y.Z.4 MODEXP common processing    ;;
;;                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    ebs extraction and analysis row    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---ebs-extraction-and-analysis---setting-misc-module-flags
                  (:guard    (precompile-processing---MODEXP---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---ebs-extraction-and-analysis)
                          (+    (*    MISC_WEIGHT_MMU    (precompile-processing---MODEXP---extract-ebs))
                                MISC_WEIGHT_OOB)
                          ))


(defconstraint    precompile-processing---MODEXP---ebs-extraction-and-analysis---setting-MMU-instruction
                  (:guard    (precompile-processing---MODEXP---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (shift    misc/MMU_FLAG    precompile-processing---MODEXP---misc-row-offset---ebs-extraction-and-analysis)
                                  (set-MMU-instruction---right-padded-word-extraction    precompile-processing---MODEXP---misc-row-offset---ebs-extraction-and-analysis                                          ;; offset
                                                                                         CONTEXT_NUMBER                                                                                           ;; source ID
                                                                                         ;; tgt_id                                                                                                   ;; target ID
                                                                                         ;; aux_id                                                                                                   ;; auxiliary ID
                                                                                         ;; src_offset_hi                                                                                            ;; source offset high
                                                                                         32                                                                                                       ;; source offset low
                                                                                         ;; tgt_offset_lo                                                                                            ;; target offset low
                                                                                         ;; size                                                                                                     ;; size
                                                                                         (precompile-processing---dup-cdo)                                                                        ;; reference offset
                                                                                         (precompile-processing---dup-cds)                                                                        ;; reference size
                                                                                         ;; success_bit                                                                                              ;; success bit
                                                                                         (shift    misc/MMU_LIMB_1    precompile-processing---MODEXP---misc-row-offset---ebs-extraction-and-analysis)            ;; limb 1          ;; TODO: remove SELF REFERENCE
                                                                                         (shift    misc/MMU_LIMB_2    precompile-processing---MODEXP---misc-row-offset---ebs-extraction-and-analysis)            ;; limb 2          ;; TODO: remove SELF REFERENCE
                                                                                         ;; exo_sum                                                                                                  ;; weighted exogenous module flag sum
                                                                                         ;; phase                                                                                                    ;; phase
                                                                                         )))


(defun    (precompile-processing---MODEXP---ebs-hi)    (*    (precompile-processing---MODEXP---extract-ebs)    (shift    misc/MMU_LIMB_1    precompile-processing---MODEXP---misc-row-offset---ebs-extraction-and-analysis)))
(defun    (precompile-processing---MODEXP---ebs-lo)    (*    (precompile-processing---MODEXP---extract-ebs)    (shift    misc/MMU_LIMB_2    precompile-processing---MODEXP---misc-row-offset---ebs-extraction-and-analysis)))


(defconstraint    precompile-processing---MODEXP---ebs-extraction-and-analysis---setting-OOB-instruction
                  (:guard    (precompile-processing---MODEXP---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (set-OOB-instruction---modexp-xbs    precompile-processing---MODEXP---misc-row-offset---ebs-extraction-and-analysis         ;; offset
                                                       (precompile-processing---MODEXP---ebs-hi)                               ;; high part of some {b,e,m}bs
                                                       (precompile-processing---MODEXP---ebs-lo)                               ;; low  part of some {b,e,m}bs
                                                       0                                                                       ;; low  part of some {b,e,m}bs
                                                       0                                                                       ;; bit indicating whether to compute max(xbs, ybs) or not
                                                       ))


(defun    (precompile-processing---MODEXP---ebs-within-bounds)    (i1 (shift    [misc/OOB_DATA   9]    precompile-processing---MODEXP---misc-row-offset---ebs-extraction-and-analysis)))
(defun    (precompile-processing---MODEXP---ebs-out-of-bounds)    (i1 (shift    [misc/OOB_DATA  10]    precompile-processing---MODEXP---misc-row-offset---ebs-extraction-and-analysis))) ;; ""
(defun    (precompile-processing---MODEXP---ebs-normalized)       (*   (precompile-processing---MODEXP---ebs-lo)
                                                                       (precompile-processing---MODEXP---ebs-within-bounds)))
