(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;    X.Y.Z BLK_NUMBER constraints    ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; this should be a defproperty ... but I don't believe there are domains for defproperty
(defconstraint    block-number-constraints---vanishes-initially                     (:domain {0})   (vanishes!             BLK_NUMBER)) ;; ""
(defproperty      block-number-constraints---zero-one-increments                                    (has-0-1-increments    BLK_NUMBER))
;; (defconstraint    block-number-constraints---increments                             ()              (will-inc!             BLK_NUMBER    (*    (- 1  SYSI)  (next SYSI))))
(defcomputedcolumn   (BLK_NUMBER :i16 :fwd)     (+   (prev BLK_NUMBER)
						     (*    (-  1   (prev SYSI))
							   SYSI)))
(defconstraint    block-number-constraints---BLK_NUMBER-is-pegged-to-txn-flag-sum   ()              (if-zero               BLK_NUMBER
															    (eq!    (txn-flag-sum)    0)  ;; BLK = 0
															    (eq!    (txn-flag-sum)    1)  ;; BLK ≠ 0
															    ))
