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



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    CALL_DATA_SIZE analysis row    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---call-data-size-analysis-row---setting-module-flags
                  (:guard    (precompile-processing---MODEXP---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---cds-analysis)
                          MISC_WEIGHT_OOB))


(defconstraint    precompile-processing---MODEXP---call-data-size-analysis-row---setting-OOB-instruction
                  (:guard    (precompile-processing---MODEXP---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (set-OOB-instruction---modexp-cds    precompile-processing---MODEXP---misc-row-offset---cds-analysis   ;; row offset
                                                       (precompile-processing---dup-cds)))                               ;; call data size


(defun    (precompile-processing---MODEXP---extract-bbs)    (shift    [misc/OOB_DATA  3]    precompile-processing---MODEXP---misc-row-offset---cds-analysis)) ;; ""
(defun    (precompile-processing---MODEXP---extract-ebs)    (shift    [misc/OOB_DATA  4]    precompile-processing---MODEXP---misc-row-offset---cds-analysis)) ;; ""
(defun    (precompile-processing---MODEXP---extract-mbs)    (shift    [misc/OOB_DATA  5]    precompile-processing---MODEXP---misc-row-offset---cds-analysis)) ;; ""
