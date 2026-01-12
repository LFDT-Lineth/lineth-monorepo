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


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    pricing analysis row    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defun    (precompile-processing---MODEXP---call-OOB-on-pricing-row)       (shift    misc/OOB_FLAG    precompile-processing---MODEXP---misc-row-offset---pricing))



(defconstraint    precompile-processing---MODEXP---pricing-analysis---setting-misc-module-flags
                  (:guard    (precompile-processing---MODEXP---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---pricing)
                          (*  MISC_WEIGHT_OOB  (precompile-processing---MODEXP---all-byte-sizes-are-in-bounds))
                          ))


(defconstraint    precompile-processing---MODEXP---pricing-analysis---setting-OOB-instruction
                  (:guard    (precompile-processing---MODEXP---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (precompile-processing---MODEXP---call-OOB-on-pricing-row)
                                 (set-OOB-instruction---modexp-pricing    precompile-processing---MODEXP---misc-row-offset---pricing   ;; offset
                                                                          (precompile-processing---dup-callee-gas)                     ;; call gas i.e. gas provided to the precompile
                                                                          (precompile-processing---dup-r@c)                            ;; return at capacity
                                                                          (precompile-processing---MODEXP---modexp-full-log)           ;; leading (≤) word log of exponent
                                                                          (precompile-processing---MODEXP---max-mbs-bbs)               ;; call data size
                                                                          )))

(defun    (precompile-processing---MODEXP---ram-success)    (i1 (shift    [misc/OOB_DATA   4]    precompile-processing---MODEXP---misc-row-offset---pricing)))
(defun    (precompile-processing---MODEXP---return-gas)     (shift    [misc/OOB_DATA   5]    precompile-processing---MODEXP---misc-row-offset---pricing))
(defun    (precompile-processing---MODEXP---r@c-nonzero)    (i1 (shift    [misc/OOB_DATA   8]    precompile-processing---MODEXP---misc-row-offset---pricing)))
