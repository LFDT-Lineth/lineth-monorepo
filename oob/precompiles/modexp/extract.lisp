(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;   OOB_INST_MODEXP_extract   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-modexp-extract---standard-precondition)                 IS_MODEXP_EXTRACT)
(defun (prc-modexp-extract---bbs)                                   [DATA 3])
(defun (prc-modexp-extract---ebs)                                   [DATA 4])
(defun (prc-modexp-extract---mbs)                                   [DATA 5])
(defun (prc-modexp-extract---extract-base)                          [DATA 6])
(defun (prc-modexp-extract---extract-exponent)                      [DATA 7])
(defun (prc-modexp-extract---extract-modulus)                       [DATA 8])
(defun (prc-modexp-extract---bbs-is-zero)                           OUTGOING_RES_LO)
(defun (prc-modexp-extract---ebs-is-zero)                           (next OUTGOING_RES_LO))
(defun (prc-modexp-extract---mbs-is-zero)                           (shift OUTGOING_RES_LO 2))
(defun (prc-modexp-extract---call-data-extends-beyond-exponent)     (shift OUTGOING_RES_LO 3))

(defconstraint prc-modexp-extract---check-bbs-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-extract---standard-precondition)))
  (call-to-ISZERO 0 0 (prc-modexp-extract---bbs)))

(defconstraint prc-modexp-extract---check-ebs-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-extract---standard-precondition)))
  (call-to-ISZERO 1 0 (prc-modexp-extract---ebs)))

(defconstraint prc-modexp-extract---check-mbs-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-extract---standard-precondition)))
  (call-to-ISZERO 2 0 (prc-modexp-extract---mbs)))

(defconstraint prc-modexp-extract---compare-96-plus-bbs-plus-ebs-against-cds (:guard (* (assumption---fresh-new-stamp) (prc-modexp-extract---standard-precondition)))
  (call-to-LT 3 0 (+ 96 (prc-modexp-extract---bbs) (prc-modexp-extract---ebs)) 0 (prc---cds)))

(defconstraint prc-modexp-extract---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-modexp-extract---standard-precondition)))
  (begin (eq! (prc-modexp-extract---extract-modulus)
              (* (prc-modexp-extract---call-data-extends-beyond-exponent)
                 (- 1 (prc-modexp-extract---mbs-is-zero))))
         (eq! (prc-modexp-extract---extract-base)
              (* (prc-modexp-extract---extract-modulus) (- 1 (prc-modexp-extract---bbs-is-zero))))
         (eq! (prc-modexp-extract---extract-exponent)
              (* (prc-modexp-extract---extract-modulus) (- 1 (prc-modexp-extract---ebs-is-zero))))))
