(module hash_info)

(defcolumns
  KEC_NB
  (INST :BOOLEAN)
  KEC_HI
  KEC_LO
  SIZE)


(defconstraint keccak-nb-starts-at-zero (:domain {0})
  (vanishes KEC_NB))

(defconstraint keccak-nb-non-decreasing ()
  (begin
   (vanishes (* (remains-constant KEC_NB) (inc KEC_NB 1)))
   (if-not-zero KEC_NB
                (inc KEC_NB 1))))

(defconstraint keccak-constraints ()
  (if-zero KEC_NB
           (begin (vanishes INST)
                  (vanishes KEC_HI)
                  (vanishes KEC_LO)
                  (vanishes SIZE))))
