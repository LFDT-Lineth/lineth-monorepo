(module rlptxn)

(defun (rlptxn---DATA-phase---payload-size)                  (+ (rlptxn---DATA-phase---zeros-in-payload)
                                                                (rlptxn---DATA-phase---nonzs-in-payload)))
(defun (rlptxn---DATA-phase---zeros-in-payload)              (prev txn/NUMBER_OF_ZERO_BYTES))
(defun (rlptxn---DATA-phase---nonzs-in-payload)              (prev txn/NUMBER_OF_NONZERO_BYTES))
(defun (rlptxn---DATA-phase---first-byte-or-zero)            (* (rlptxn---DATA-phase---payload-is-nonempty) (rlptxn---DATA-phase---maybe-first-byte-of-payload)))
(defun (rlptxn---DATA-phase---maybe-first-byte-of-payload)   (next cmp/EXO_DATA_8)) 
(defun (rlptxn---DATA-phase---payload-is-nonempty)           cmp/EXO_DATA_4) ;; ""
(defun (rlptxn---DATA-phase---payload-is-empty)              (force-bin (- 1 (rlptxn---DATA-phase---payload-is-nonempty)))) 

(defconstraint   data-phase---payload-size-analysis-row---calling-RLP_UTILS
                 (:guard   (is-payload-size-analysis-row))
                 ;; RLP_UTILS instruction call
                 (rlp-compound-constraint---BYTE_STRING_PREFIX    0
                                                                  (rlptxn---DATA-phase---payload-size)
                                                                  (rlptxn---DATA-phase---first-byte-or-zero)
                                                                  0
                                                                  0)
                 )

(defconstraint   data-phase---payload-size-analysis-row---initializing-countdowns
                 (:guard   (is-payload-size-analysis-row))
                 (begin
                   (eq!   (zeros-countdown)   (rlptxn---DATA-phase---zeros-in-payload))
                   (eq!   (nonzs-countdown)   (rlptxn---DATA-phase---nonzs-in-payload))
                   ))

(defconstraint   data-phase---payload-size-analysis-row---empty-payload-sanity-check
                 (:guard   (is-payload-size-analysis-row))
                 (if-not-zero (rlptxn---DATA-phase---payload-is-empty)
                              (begin 
                                (vanishes! (rlptxn---DATA-phase---zeros-in-payload))
                                (vanishes! (rlptxn---DATA-phase---nonzs-in-payload))))
                 )

(defconstraint   data-phase---payload-size-analysis-row---empty-payload-sanity-check2
                 (:guard   (is-payload-size-analysis-row))
                 (eq! PHASE_END (rlptxn---DATA-phase---payload-is-empty))
                 )
