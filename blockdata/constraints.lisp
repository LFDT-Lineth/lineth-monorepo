(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    2.2 Binary constraints      ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; done with binary@prove in columns.lisp

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;  2.3 Unconditional constraints  ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   unconditional-constraints ()
                 (begin
                   (eq! IOMF   (flag-sum))
                   (eq! CT_MAX (ct-max-sum))
                   (eq! INST   (inst-sum))))

;;;;;;;;;;;;;;;;;;;;;;
;;                  ;;
;;  2.4 Contancies  ;;
;;                  ;;
;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   counter-constancies ()
                 (begin (counter-constancy CT DATA_HI)
                        (counter-constancy CT DATA_LO)
                        (counter-constancy CT COINBASE_HI)
                        (counter-constancy CT COINBASE_LO)
                        (counter-constancy CT REL_TX_NUM_MAX)
                        (counter-constancy CT BLOCK_GAS_LIMIT)
                        (counter-constancy CT (wght-sum))))

(defconstraint   first-block-number-is-conflation-constant ()
                 (if-not-zero IOMF
                              (will-remain-constant! FIRST_BLOCK_NUMBER)))

;;;;;;;;;;;;;;;;;;;;;
;;                 ;;
;;  2.5 Heartbeat  ;;
;;                 ;;
;;;;;;;;;;;;;;;;;;;;;

(defconstraint   heartbeat---iomf-initially-vanishes (:domain {0}) ;; ""
                 (vanishes! IOMF))

(defconstraint   heartbeat---iomf-is-non-decreasing ()
                 (if-not-zero    IOMF
                                 (eq!    (next    IOMF)    1)))

(defconstraint   heartbeat---padding-vanishing ()
                 (if-zero IOMF
                          (begin (vanishes! CT)
                                 (vanishes! (next CT)))))

(defconstraint   heartbeat---first-instruction-is-coinbase ()
                 (if-not    (will-remain-constant!    IOMF)
                            (will-eq!                 IS_CB 1)))

(defconstraint   heartbeat---counter-reset-at-phase-entry ()
                 (if-not-zero (phase-entry)
                              (vanishes! (next CT))))

(defconstraint   heartbeat---counter-increase-or-instruction-transition ()
                 (if-not-zero IOMF
                              (if-not-zero (- CT CT_MAX)
                                           (will-inc!  CT  1)
                                           (eq!        (allowable-transitions) 1))))

(defconstraint   heartbeat---first-row-rel-block (:domain {0}) ;; ""
                 (vanishes! REL_BLOCK))

(defconstraint   heartbeat---rel-block-increments-by-0-or-1 ()
                 (or!  (will-inc!  REL_BLOCK  0)
                       (will-inc!  REL_BLOCK  1)))

(defconstraint   heartbeat---rel-block-exact-increments ()
                 (eq!    (next REL_BLOCK)
                         (+ REL_BLOCK (* (-  1  IS_CB) (next IS_CB)))))

(defconstraint   heartbeat---finalization-constraints (:domain {-1}) ;; ""
                 (begin
                   (eq!  IS_BF  1)
                   (eq!  CT     CT_MAX)))
