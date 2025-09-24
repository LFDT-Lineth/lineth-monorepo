(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;   Constancy constraints   ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint counter-constancy ()
  (begin (counter-constancy CT STAMP)
         (debug (counter-constancy CT CT_MAX))
         (for i [10] (counter-constancy CT [DATA i]))
         (counter-constancy CT OOB_INST)))
