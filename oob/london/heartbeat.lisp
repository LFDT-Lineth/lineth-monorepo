(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.5 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint padding-vanishing ()
  (if-zero STAMP
           (begin (vanishes! CT)
                  (vanishes! (+ (lookup-sum 0) (flag-sum))))))

(defconstraint stamp-increments ()
  (or! (remained-constant! STAMP) (did-inc! STAMP 1)))

(defconstraint counter-reset ()
  (if-not (remained-constant! STAMP)
          (vanishes! CT)))

(defconstraint ct-max ()
  (eq! CT_MAX (maxct-sum)))

(defconstraint non-trivial-instruction-counter-cycle ()
  (if-not-zero STAMP
               (if-eq-else CT CT_MAX (will-inc! STAMP 1) (will-inc! CT 1))))

(defconstraint final-row (:domain {-1})
  (if-not-zero STAMP
               (eq! CT CT_MAX)))
