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


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    leading_word extraction and analysis row    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---MODEXP---call-EXP-to-analyze-leading-word)   (shift    misc/EXP_FLAG    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis))
(defun    (precompile-processing---MODEXP---call-MMU-to-extract-leading-word)   (shift    misc/MMU_FLAG    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis))
(defun    (precompile-processing---MODEXP---call-OOB-on-leading-word-row)       (shift    misc/OOB_FLAG    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis))


(defconstraint    precompile-processing---MODEXP---lead-log-analysis---setting-misc-module-flags
                  (:guard    (precompile-processing---MODEXP---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!    (precompile-processing---MODEXP---call-OOB-on-leading-word-row)         (precompile-processing---MODEXP---all-byte-sizes-are-in-bounds) )
                    (eq!    (precompile-processing---MODEXP---call-EXP-to-analyze-leading-word)     (precompile-processing---MODEXP---extract-leading-word)         )
                    (eq!    (precompile-processing---MODEXP---call-MMU-to-extract-leading-word)     (precompile-processing---MODEXP---extract-leading-word)         )
                    (eq!    (+    (shift    misc/MXP_FLAG    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)
                                  (shift    misc/STP_FLAG    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis))
                            0)
                    ))


(defconstraint    precompile-processing---MODEXP---lead-log-analysis---setting-OOB-instruction
                  (:guard    (precompile-processing---MODEXP---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (precompile-processing---MODEXP---call-OOB-on-leading-word-row)
                                 (set-OOB-instruction---modexp-lead    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis   ;; offset
                                                                       (precompile-processing---MODEXP---bbs-normalized)                          ;; low part of bbs (base     byte size)
                                                                       (precompile-processing---dup-cds)                                          ;; call data size
                                                                       (precompile-processing---MODEXP---ebs-normalized)                          ;; low part of ebs (exponent byte size)
                                                                       )))

(defun    (precompile-processing---MODEXP---extract-leading-word)     (shift    [misc/OOB_DATA  4]    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)) ;; ""
(defun    (precompile-processing---MODEXP---cds-cutoff)               (shift    [misc/OOB_DATA  6]    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)) ;; ""
(defun    (precompile-processing---MODEXP---ebs-cutoff)               (shift    [misc/OOB_DATA  7]    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)) ;; ""
(defun    (precompile-processing---MODEXP---sub-ebs-32)               (shift    [misc/OOB_DATA  8]    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)) ;; ""


(defconstraint    precompile-processing---MODEXP---lead-word-analysis---setting-MMU-instruction
                  (:guard    (precompile-processing---MODEXP---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (precompile-processing---MODEXP---call-MMU-to-extract-leading-word)
                                  (set-MMU-instruction---mload    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis                                   ;; offset
                                                                  CONTEXT_NUMBER                                                                                             ;; source ID
                                                                  ;; tgt_id                                                                                                     ;; target ID
                                                                  ;; aux_id                                                                                                     ;; auxiliary ID
                                                                  ;; src_offset_hi                                                                                              ;; source offset high
                                                                  (+    (precompile-processing---dup-cdo)
                                                                        96
                                                                        (precompile-processing---MODEXP---bbs-normalized))                                                      ;; source offset low
                                                                  ;; tgt_offset_lo                                                                                              ;; target offset low
                                                                  ;; size                                                                                                       ;; size
                                                                  ;; ref_offset                                                                                                 ;; reference offset
                                                                  ;; ref_size                                                                                                   ;; reference size
                                                                  ;; success_bit                                                                                                ;; success bit
                                                                  (shift    misc/MMU_LIMB_1    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)     ;; limb 1    ;; TODO: remove SELF REFERENCE
                                                                  (shift    misc/MMU_LIMB_2    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)     ;; limb 2    ;; TODO: remove SELF REFERENCE
                                                                  ;; exo_sum                                                                                                    ;; weighted exogenous module flag sum
                                                                  ;; phase                                                                                                      ;; phase
                                                                  )))

(defun    (precompile-processing---MODEXP---raw-lead-hi)    (*    (precompile-processing---MODEXP---call-MMU-to-extract-leading-word)    (shift    misc/MMU_LIMB_1    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)))
(defun    (precompile-processing---MODEXP---raw-lead-lo)    (*    (precompile-processing---MODEXP---call-MMU-to-extract-leading-word)    (shift    misc/MMU_LIMB_2    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)))


(defconstraint    precompile-processing---MODEXP---lead-word-analysis---setting-EXP-instruction
                  (:guard    (precompile-processing---MODEXP---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (precompile-processing---MODEXP---call-EXP-to-analyze-leading-word)
                                  (set-EXP-instruction-MODEXP-lead-log    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis          ;; row offset
                                                                          (precompile-processing---MODEXP---raw-lead-hi)                                    ;; raw leading word where exponent starts, high part
                                                                          (precompile-processing---MODEXP---raw-lead-lo)                                    ;; raw leading word where exponent starts, low  part
                                                                          (precompile-processing---MODEXP---cds-cutoff)                                     ;; min{max{cds - 96 - bbs, 0}, 32}
                                                                          (precompile-processing---MODEXP---ebs-cutoff)                                     ;; min{ebs, 32}
                                                                          )))

(defun    (precompile-processing---MODEXP---lead-log)           (shift    [misc/EXP_DATA   5]    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)) ;; ""
(defun    (precompile-processing---MODEXP---modexp-full-log)    (+    (precompile-processing---MODEXP---lead-log)    (*   16    (precompile-processing---MODEXP---sub-ebs-32))))

;; @OLIVIER: on reprend ici; pas sûr que modexp-full-log soit bien défini (filtres différents)
