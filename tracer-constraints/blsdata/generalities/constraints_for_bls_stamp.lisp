(module blsdata)

(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint stamp-increment-sanity-check ()
  (begin
    (debug (or! (will-remain-constant! STAMP) (will-inc! STAMP 1))))) ;; implied by the constraint below

(defconstraint stamp-increment ()
  (eq! (next STAMP) (+ STAMP (transition_to_data))))