(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    2.3 instruction decoding   ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint flag-sum-vanishes ()
  (if-zero STAMP
           (vanishes! (flag-sum))))

(defconstraint flag-sum-equal-one ()
  (if-not-zero STAMP
               (eq! (flag-sum) 1)))

(defconstraint decoding ()
  (eq! OOB_INST (wght-sum)))
