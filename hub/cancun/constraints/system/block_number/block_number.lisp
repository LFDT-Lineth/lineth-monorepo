(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;   X.Y The BLK_NUMBER column      ;;
;;   X.Y.Z BLK_NUMBER constraints   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    BLK_NUMBER-constraints---transaction-constancy ()
		  (transaction-constancy    BLK_NUMBER))

(defconstraint    BLK_NUMBER-constraints---initialization (:domain {0}) ;; ""
		  (vanishes!    BLK_NUMBER))

(defconstraint    BLK_NUMBER-constraints---increments ()
		  (will-inc!    BLK_NUMBER    (system-block-number---about-to-enter-sysi)))

(defproperty      BLK_NUMBER-constraints---0-1-increments
		  (has-0-1-increments    BLK_NUMBER))
