(module blsdata)

(defconstraint id-increment ()
  (if-not-zero (- (next STAMP) STAMP)
               (eq! (next ID)
                    (+ ID
                       1
                       (+ (* 256 256 256 (next BYTE_DELTA))
                          (* 256 256 (shift BYTE_DELTA 2))
                          (* 256 (shift BYTE_DELTA 3))
                          (shift BYTE_DELTA 4))))))
