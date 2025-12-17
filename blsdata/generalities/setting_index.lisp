(module blsdata)

(defconstraint vanishing-values-index ()
  (if-zero (flag_sum)
           (begin (vanishes! INDEX_MAX)                 
                  (vanishes! INDEX)
                  (vanishes! ID))))

(defconstraint index-reset ()
  (if-not-zero (transition_bit)
               (vanishes! (next INDEX))))

(defconstraint index-increment ()
  (if-not-zero (flag_sum)
               (if-eq-else INDEX INDEX_MAX
                           (eq! (transition_bit) 1)
                           (eq! (next INDEX) (+ 1 INDEX)))))

(defconstraint final-row (:domain {-1})
  (if-not-zero (flag_sum)
               (begin (eq! (is_result) 1)
                      (eq! INDEX INDEX_MAX))))
