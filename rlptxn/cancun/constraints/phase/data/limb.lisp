(module rlptxn)


(defconstraint    data-phase---limb-content-analysis---RLP_UTILS-call
                  (:guard   (is-limb-content-analysis-row))
                  (rlputils-call---DATA_PRICING   0
                                                  cmp/LIMB
                                                  cmp/LIMB_SIZE)
                  )

(defun (zeros-in-limb)      cmp/EXO_DATA_6)
(defun (nonzs-in-limb)      cmp/EXO_DATA_7) ;; ""

(defconstraint    data-phase---limb-content-analysis---updating-countdowns
                  (:guard   (is-limb-content-analysis-row))
                  (begin
                    (eq! (zeros-countdown) (- (prev (zeros-countdown)) (zeros-in-limb)))
                    (eq! (nonzs-countdown) (- (prev (nonzs-countdown)) (nonzs-in-limb)))
                    ))

(defconstraint    data-phase---limb-content-analysis---PHASE_END-constraints-and-further-relations
                  (:guard   (is-limb-content-analysis-row))
                  (begin
                    (if-zero PHASE_END (eq! cmp/LIMB_SIZE LLARGE))
                    ;; (eq! PHASE_END (* (- 1 (~ (zeros-countdown))) (- 1 (~ (nonzs-countdown))))) is equivalent to the two following constraints
                    (if   (and!   (== (zeros-countdown) 0)
                                  (== (nonzs-countdown) 0))
                      (eq! PHASE_END 1))
                    (if-not-zero   PHASE_END 
                                   (begin
                                     (vanishes! (zeros-countdown))
                                     (vanishes! (nonzs-countdown))))
                    (eq! PHASE_END DONE)
                    ))
