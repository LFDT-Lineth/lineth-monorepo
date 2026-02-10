(module rlptxn)

(defconstraint first-cmp-row-of-cmp (:guard IS_AUTHORIZATION_LIST)
  (if-not-zero (rlptxn---authorization-list---first-CMP-row)
    (begin
      (eq! (rlptxn---authorization-list---tuple-index) 0)
      (eq! (next (rlptxn---authorization-list---tuple-index)) 1)
      (eq! (shift IS_AUTHORIZATION_LIST 1) 1)
      (eq! (shift IS_AUTHORIZATION_LIST 10) 1)
      (eq! (shift IS_AUTHORIZATION_LIST RLP_TXN_NB_ROWS_PER_AUTHORIZATION) 1) )))

(defconstraint index-growth (:guard IS_AUTHORIZATION_LIST)
  (if-not-zero (rlptxn---authorization-list---again-CMP-row)
  (has-0-1-increments (rlptxn---authorization-list---tuple-index))))

(defconstraint index-increment (:guard IS_AUTHORIZATION_LIST)
 (if-not-zero (* CMP (- NUMBER_OF_AUTHORIZATIONS (rlptxn---authorization-list---tuple-index)))
  (begin
  (eq! (shift IS_AUTHORIZATION_LIST 1) 1)
  (eq! (shift IS_AUTHORIZATION_LIST RLP_TXN_NB_ROWS_PER_AUTHORIZATION) 1)
  (eq! (shift (rlptxn---authorization-list---tuple-index) RLP_TXN_NB_ROWS_PER_AUTHORIZATION)
       (+ (rlptxn---authorization-list---tuple-index) 1)
 ))))

(defconstraint end-of-phase (:guard IS_AUTHORIZATION_LIST)
  (if-zero (next IS_AUTHORIZATION_LIST)
  (eq! (rlptxn---authorization-list---tuple-index) NUMBER_OF_AUTHORIZATIONS)
  ))
