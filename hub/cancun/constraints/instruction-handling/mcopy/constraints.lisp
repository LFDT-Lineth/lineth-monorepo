(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                               ;;;;
;;;;    X.5 Instruction handling   ;;;;
;;;;                               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    X.Y.Z MCOPY instruction   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   mcopy-instruction---setting-the-stack-pattern
                 (:guard (mcopy-instruction---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (stack-pattern-3-0))

(defconstraint   mcopy-instruction---the-first-row-must-be-misc-and-we-call-MXP
                 (:guard (mcopy-instruction---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq! (shift PEEK_AT_MISCELLANEOUS MCOPY_ROFF___1ST_MISC_ROW) 1)
                   (eq! (shift misc/MXP_FLAG         MCOPY_ROFF___1ST_MISC_ROW) 1)
                   ))

(defconstraint   mcopy-instruction---defining-the-MXP-instruction
                 (:guard (mcopy-instruction---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (set-MXP-instruction---for-MCOPY   MCOPY_ROFF___1ST_MISC_ROW                ;; row offset kappa
                                                    (mcopy-instruction---target-offset-hi)   ;; target offset hi
                                                    (mcopy-instruction---target-offset-lo)   ;; target offset lo
                                                    (mcopy-instruction---source-offset-hi)   ;; source offset hi
                                                    (mcopy-instruction---source-offset-lo)   ;; source offset lo
                                                    (mcopy-instruction---size-hi)            ;; size hi
                                                    (mcopy-instruction---size-lo)            ;; size lo
                                          ))

(defun   (mcopy-instruction---trigger_MXP)   1)
(defun   (mcopy-instruction---trigger_MMU)   (*  (-  1  XAHOY)
                                                 (mcopy-instruction---MXP-s1nznomxpx))) ;; ""

(defconstraint   mcopy-instruction---setting-NSR
                 (:guard (mcopy-instruction---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    XAHOY
                                 ;; XAHOY ≡ true
                                 (eq!   NSR   2)
                                 ;; XAHOY ≡ false
                                 (eq!   NSR   (+  1  (mcopy-instruction---trigger_MMU))))
                 )

(defconstraint   mcopy-instruction---setting-the-peeking-flags
                 (:guard (mcopy-instruction---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    XAHOY
                                 ;; XAHOY ≡ true
                                 (eq!   NSR
                                        (+   (shift  PEEK_AT_MISCELLANEOUS  MCOPY_ROFF___1ST_MISC_ROW )
                                             (shift  PEEK_AT_CONTEXT        MCOPY_ROFF___XCON_ROW     )))
                                 ;; XAHOY ≡ false
                                 (eq!   NSR
                                        (+   (shift  PEEK_AT_MISCELLANEOUS  MCOPY_ROFF___1ST_MISC_ROW )
                                             (*  (mcopy-instruction---trigger_MMU)  (shift  PEEK_AT_MISCELLANEOUS  MCOPY_ROFF___2ND_MISC_ROW ))))
                                 ))

(defconstraint   mcopy-instruction---justifying-the-stacks-MXPX-flag
                 (:guard (mcopy-instruction---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   stack/MXPX
                        (mcopy-instruction---MXP-mxpx)
                 ))

(defconstraint   mcopy-instruction---setting-the-gas-cost
                 (:guard (mcopy-instruction---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero   stack/MXPX
                            ;; MXPX ≡ false
                            (eq!  GAS_COST  (+  stack/STATIC_GAS  (mcopy-instruction---MXP-gas-mxp)))
                            ;; MXPX ≡ true
                            (eq!  GAS_COST  0 )
                 ))

(defconstraint   mcopy-instruction---1st-MISC-row---setting-the-flags
                 (:guard (mcopy-instruction---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!   (weighted-MISC-flag-sum-sans-MMU   MCOPY_ROFF___1ST_MISC_ROW)   (*   MISC_WEIGHT_MXP (mcopy-instruction---trigger_MXP)))
                   (eq!   (shift   misc/MMU_FLAG             MCOPY_ROFF___1ST_MISC_ROW)   (mcopy-instruction---trigger_MMU))
                   ))

(defconstraint   mcopy-instruction---2nd-MISC-row---setting-the-flags
                 (:guard (mcopy-instruction---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (mcopy-instruction---trigger_MMU)
                                (eq! (weighted-MISC-flag-sum   MCOPY_ROFF___2ND_MISC_ROW)
                                     MISC_WEIGHT_MMU)))

(defconstraint   mcopy-instruction---1st-MISC-row---setting-the-MMU-call
                 (:guard (mcopy-instruction---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (mcopy-instruction---trigger_MMU)
                                (set-MMU-instruction---ram-to-ram-sans-padding    MCOPY_ROFF___1ST_MISC_ROW                 ;; offset
                                                                                  CONTEXT_NUMBER                           ;; source ID
                                                                                  (+  1  HUB_STAMP)                        ;; target ID
                                                                                  ;; aux_id                                ;; auxiliary ID
                                                                                  ;; src_offset_hi                         ;; source offset high
                                                                                  (mcopy-instruction---source-offset-lo)   ;; source offset low
                                                                                  ;; tgt_offset_lo                         ;; target offset low
                                                                                  (mcopy-instruction---size-lo)            ;; size
                                                                                  0                                        ;; reference offset
                                                                                  (mcopy-instruction---size-lo)            ;; reference size
                                                                                  ;; success_bit                           ;; success bit
                                                                                  ;; limb_1                                ;; limb 1
                                                                                  ;; limb_2                                ;; limb 2
                                                                                  ;; exo_sum                               ;; weighted exogenous module flag sum
                                                                                  ;; phase                                 ;; phase
                                                                                  )))

(defconstraint   mcopy-instruction---2nd-MISC-row---setting-the-MMU-call
                 (:guard (mcopy-instruction---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (mcopy-instruction---trigger_MMU)
                                (set-MMU-instruction---ram-to-ram-sans-padding    MCOPY_ROFF___2ND_MISC_ROW                ;; offset
                                                                                  (+  1  HUB_STAMP)                        ;; source ID
                                                                                  CONTEXT_NUMBER                           ;; target ID
                                                                                  ;; aux_id                                ;; auxiliary ID
                                                                                  ;; src_offset_hi                         ;; source offset high
                                                                                  0                                        ;; source offset low
                                                                                  ;; tgt_offset_lo                         ;; target offset low
                                                                                  (mcopy-instruction---size-lo)            ;; size
                                                                                  (mcopy-instruction---target-offset-lo)   ;; reference offset
                                                                                  (mcopy-instruction---size-lo)            ;; reference size
                                                                                  ;; success_bit                           ;; success bit
                                                                                  ;; limb_1                                ;; limb 1
                                                                                  ;; limb_2                                ;; limb 2
                                                                                  ;; exo_sum                               ;; weighted exogenous module flag sum
                                                                                  ;; phase                                 ;; phase
                                                                                  )))
