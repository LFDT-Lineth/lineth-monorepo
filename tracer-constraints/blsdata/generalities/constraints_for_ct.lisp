(module blsdata)

(defconstraint vanishing-values-ct ()
  (if-zero (flag_sum)
           (begin (vanishes! CT_MAX)
                  (vanishes! CT)
                  (debug (vanishes! (next CT))))))

(defconstraint ct-increment ()
  (if-eq-else CT CT_MAX
              (vanishes! (next CT))
              (eq! (next CT) (+ 1 CT))))
