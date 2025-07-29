(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;   X.Y The BLK_NUMBER column      ;;
;;   X.Y.Z BLK_NUMBER constraints   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (has-zero-one-increments    col) (or!    (will-inc!    col    0)
						   (will-inc!    col    1)
						   ))

(defconstraint    BLK_NUMBER-constraints---transaction-constancy ()
		  (transaction-constancy    BLK_NUMBER))

(defconstraint    BLK_NUMBER-constraints---initialization (:domain {0}) ;; ""
		  (vanishes!    BLK_NUMBER))

(defconstraint    BLK_NUMBER-constraints---increments ()
		  (will-inc!    BLK_NUMBER    (system-block-number---about-to-enter-sysi)))

(defproperty      BLK_NUMBER-constraints---0-1-increments
		  (has-zero-one-increments    BLK_NUMBER))
