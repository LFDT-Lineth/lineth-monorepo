(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;    X.Y.Z perspective_sum constraints    ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (perspective-sum)    (force-bin   (+    RLP
                                                  HUB
                                                  CMPTN)))

(defproperty      perspective-sum-constraints---it-coincides-with-txn-flag-sum                  (eq!         (perspective-sum)   (txn-flag-sum)))
(defconstraint    perspective-sum-constraints---it-vanishes-initially          (:domain {0})    (eq!         (perspective-sum)   0))  ;; ""
(defproperty      perspective-sum-constraints---it-is-binary                                    (is-binary   (perspective-sum)))
(defconstraint    perspective-sum-constraints---it-starts-with-a-HUB-row       ()               (if-zero     (perspective-sum)
                                                                                                             (eq!   (next   (perspective-sum))
                                                                                                                    (next   HUB))))
