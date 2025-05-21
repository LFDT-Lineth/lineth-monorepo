(module oob)


;;;;;;;;;;;;;;;;;;;;;
;;                 ;;
;;   BLAKE2F_cds   ;;
;;                 ;;
;;;;;;;;;;;;;;;;;;;;;

(defun (prc-blake-cds---standard-precondition) IS_BLAKE2F_CDS)
(defun (prc-blake-cds---valid-cds)             OUTGOING_RES_LO)
(defun (prc-blake-cds---r@c-is-zero)           (next OUTGOING_RES_LO))

(defconstraint prc-blake-cds---compare-cds-against-PRC_BLAKE2F_SIZE (:guard (* (assumption---fresh-new-stamp) (prc-blake-cds---standard-precondition)))
  (call-to-EQ 0 0 (prc---cds) 0 PRC_BLAKE2F_SIZE))

(defconstraint prc-blake-cds---check--is-zero (:guard (* (assumption---fresh-new-stamp) (prc-blake-cds---standard-precondition)))
  (call-to-ISZERO 1 0 (prc---r@c)))

(defconstraint blake2f-a---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-blake-cds---standard-precondition)))
  (begin (eq! (prc---hub-success) (prc-blake-cds---valid-cds))
         (eq! (prc---r@c-nonzero) (- 1 (prc-blake-cds---r@c-is-zero)))))
