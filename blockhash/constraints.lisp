(module blockhash)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                       ;;;;
;;;;    X. Generalities    ;;;;
;;;;                       ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    X.1 Shorthands   ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (flag-sum)         (+  MACRO  PRPRC))
(defun   (wght-sum)         (+  (*  1  MACRO)
                                (*  2  PRPRC)))
(defun   (transition-bit)   (+  (*  (-  1  MACRO)  (next  MACRO))
                                (*  (-  1  PRPRC)  (next  PRPRC))))
(defun   (ct-max-sum)       (+  (*  (-  nROWS_MACRO  1)  MACRO)
                                (*  (-  nROWS_PRPRC  1)  PRPRC)))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    X.2 Binarities   ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;; ok via :binary@prove

;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                      ;;
;;    X.3 Constancies   ;;
;;                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    constancies   ()
                  (counter-constancy   CT   (wght-sum)))

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    X.4 Heartbeat   ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   heartbeat---IOMF---unconditional-setting        ()
                 (eq!    IOMF    (flag-sum)))

(defconstraint   heartbeat---IOMF---initial-vanishing            (:domain {0}) ;; ""
                 (vanishes!    IOMF))

(defconstraint   heartbeat---IOMF---consequences-of-it-vanishing ()
                 (if-zero    IOMF
                             (begin
                               (vanishes!                CT)
                               (vanishes!                (next    PRPRC))
                               (vanishes!                (next    CT))
                               (debug      (vanishes!    macro/BLOCKHASH_ARG_HI))
                               (debug      (vanishes!    macro/BLOCKHASH_ARG_LO))
                               (debug      (vanishes!    macro/BLOCKHASH_VAL_HI))
                               (debug      (vanishes!    macro/BLOCKHASH_VAL_LO))
                               (debug      (vanishes!    macro/BLOCKHASH_RES_HI))
                               (debug      (vanishes!    macro/BLOCKHASH_RES_LO)))))

(defconstraint   heartbeat---IOMF---nondecreasing                ()
                 (if-not-zero    IOMF
                                 (will-eq!    IOMF    1)))

(defconstraint   heartbeat---CT_MAX---unconditional-setting      ()
                 (eq!    CT_MAX    (ct-max-sum)))

(defconstraint   heartbeat---CT---transitions                    ()
                 (if-not-zero    IOMF
                                 (if-eq-else    CT   CT_MAX
                                                ;; CT == CT_MAX case
                                                (eq!    (transition-bit)    1)
                                                ;; CT != CT_MAX case
                                                (will-inc!    CT    1))))

(defconstraint   heartbeat---CT---reset                          ()
                 (if-not-zero    (transition-bit)
                                 (vanishes!    (next    CT))))

(defconstraint   heartbeat---finalization                        (:domain {-1}) ;; ""
                 (if-not-zero    IOMF
                                 (begin
                                   (eq!  CT     CT_MAX)
                                   (eq!  PRPRC  1))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                     ;;;;
;;;;    Y. Processing    ;;;;
;;;;                     ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    Y.1 Shorthands   ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defun  (not-first)                          (shift  IOMF                    NEGATIVE_OF_BLOCKHASH_DEPTH))
(defun  (prev-BH-arg-hi)  (*   (not-first)   (shift  macro/BLOCKHASH_ARG_HI  NEGATIVE_OF_BLOCKHASH_DEPTH)))
(defun  (prev-BH-arg-lo)  (*   (not-first)   (shift  macro/BLOCKHASH_ARG_LO  NEGATIVE_OF_BLOCKHASH_DEPTH)))
(defun  (curr-BH-arg-hi)                             macro/BLOCKHASH_ARG_HI                       )
(defun  (curr-BH-arg-lo)                             macro/BLOCKHASH_ARG_LO                       ) ;; ""

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    Y.4 Module calls   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (wcp-call-to-LT    relof
                             arg_1_hi
                             arg_1_lo
                             arg_2_hi
                             arg_2_lo)
  (begin
    (eq!   (shift   preprocessing/EXO_ARG_1_HI   relof)   arg_1_hi    )
    (eq!   (shift   preprocessing/EXO_ARG_1_LO   relof)   arg_1_lo    )
    (eq!   (shift   preprocessing/EXO_ARG_2_HI   relof)   arg_2_hi    )
    (eq!   (shift   preprocessing/EXO_ARG_2_LO   relof)   arg_2_lo    )
    (eq!   (shift   preprocessing/EXO_INST       relof)   EVM_INST_LT )))

(defun    (wcp-call-to-LEQ    relof
                              arg_1_hi
                              arg_1_lo
                              arg_2_hi
                              arg_2_lo)
  (begin
    (eq!  (shift  preprocessing/EXO_ARG_1_HI  relof)  arg_1_hi     )
    (eq!  (shift  preprocessing/EXO_ARG_1_LO  relof)  arg_1_lo     )
    (eq!  (shift  preprocessing/EXO_ARG_2_HI  relof)  arg_2_hi     )
    (eq!  (shift  preprocessing/EXO_ARG_2_LO  relof)  arg_2_lo     )
    (eq!  (shift  preprocessing/EXO_INST      relof)  WCP_INST_LEQ )))

(defun    (wcp-call-to-EQ    relof
                             arg_1_hi
                             arg_1_lo
                             arg_2_hi
                             arg_2_lo)
  (begin
    (eq!  (shift  preprocessing/EXO_ARG_1_HI  relof)  arg_1_hi    )
    (eq!  (shift  preprocessing/EXO_ARG_1_LO  relof)  arg_1_lo    )
    (eq!  (shift  preprocessing/EXO_ARG_2_HI  relof)  arg_2_hi    )
    (eq!  (shift  preprocessing/EXO_ARG_2_LO  relof)  arg_2_lo    )
    (eq!  (shift  preprocessing/EXO_INST      relof)  EVM_INST_EQ )))



(defun    (result-must-be-true    relof)    (eq!  (shift  preprocessing/EXO_RES  relof)  1))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;    Y.5 Processing constraints   ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    processing---row-1---blockhash-argument-monontony
                  (:guard    MACRO)
                  ;;;;;;;;;;;;;;;;;
                  (begin
                    (wcp-call-to-LEQ        ROFF___BLOCKHASH_arguments___monotony
                                            (prev-BH-arg-hi)
                                            (prev-BH-arg-lo)
                                            (curr-BH-arg-hi)
                                            (curr-BH-arg-lo))
                    (result-must-be-true    ROFF___BLOCKHASH_arguments___monotony)))

(defconstraint    processing---row-2---blockhash-argument-equality-test
                  (:guard    MACRO)
                  ;;;;;;;;;;;;;;;;;
                  (wcp-call-to-EQ   ROFF___BLOCKHASH_arguments___equality_test
                                    (prev-BH-arg-hi)
                                    (prev-BH-arg-lo)
                                    (curr-BH-arg-hi)
                                    (curr-BH-arg-lo)))

(defun    (same-argument)    (shift    preprocessing/EXO_RES    ROFF___BLOCKHASH_arguments___equality_test))

(defconstraint    processing---row-3---ABS-vs-256
                  (:guard    MACRO)
                  ;;;;;;;;;;;;;;;;;
                  (wcp-call-to-LEQ   ROFF___ABS___comparison_to_256
                                     0
                                     256
                                     0
                                     macro/ABS_BLOCK))

(defun    (minimal-reachable)   (*   (shift   preprocessing/EXO_RES   ROFF___ABS___comparison_to_256)
                                     (-   macro/ABS_BLOCK   256)))

(defconstraint    processing---row-4---blockhash-argument-vs-max
                  (:guard    MACRO)
                  ;;;;;;;;;;;;;;;;;
                  (wcp-call-to-LT    ROFF___curr_BLOCKHASH_argument___comparison_to_max
                                     (curr-BH-arg-hi)
                                     (curr-BH-arg-lo)
                                     0
                                     macro/ABS_BLOCK))

(defun    (upper-bound-ok)    (shift    preprocessing/EXO_RES   ROFF___curr_BLOCKHASH_argument___comparison_to_max))

(defconstraint    processing---row-5---blockhash-argument-vs-min
                  (:guard    MACRO)
                  ;;;;;;;;;;;;;;;;;
                  (wcp-call-to-LEQ   ROFF___curr_BLOCKHASH_argument___comparison_to_min
                                     0
                                     (minimal-reachable)
                                     (curr-BH-arg-hi)
                                     (curr-BH-arg-lo)
                                     ))

(defun    (lower-bound-ok)    (shift    preprocessing/EXO_RES   ROFF___curr_BLOCKHASH_argument___comparison_to_min))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    Y.6 Result constraints   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    setting-the-result
                  (:guard    MACRO)
                  ;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!    macro/BLOCKHASH_RES_HI    (*    (arg-in-bounds)   macro/BLOCKHASH_VAL_HI))
                    (eq!    macro/BLOCKHASH_RES_LO    (*    (arg-in-bounds)   macro/BLOCKHASH_VAL_LO))))

(defun    (arg-in-bounds)    (*    (lower-bound-ok)
                                   (upper-bound-ok)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                      ;;;;
;;;;    Z. Consistency    ;;;;
;;;;                      ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    consistency ()
                  (if-not-zero    MACRO
                                  (if-not-zero    (not-first)
                                                  (if-not-zero    (same-argument)
                                                                  (begin
                                                                    (eq!   macro/BLOCKHASH_VAL_HI    (shift    macro/BLOCKHASH_VAL_HI    NEGATIVE_OF_BLOCKHASH_DEPTH))
                                                                    (eq!   macro/BLOCKHASH_VAL_LO    (shift    macro/BLOCKHASH_VAL_LO    NEGATIVE_OF_BLOCKHASH_DEPTH)))))))
