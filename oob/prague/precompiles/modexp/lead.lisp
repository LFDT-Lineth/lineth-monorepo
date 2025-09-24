(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;   OOB_INST_MODEXP_lead   ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-modexp-lead---standard-precondition)               IS_MODEXP_LEAD)
(defun (prc-modexp-lead---bbs)                                 [DATA 1])
(defun (prc-modexp-lead---ebs)                                 [DATA 3])
(defun (prc-modexp-lead---load-lead)                           [DATA 4])
(defun (prc-modexp-lead---cds-cutoff)                          [DATA 6])
(defun (prc-modexp-lead---ebs-cutoff)                          [DATA 7])
(defun (prc-modexp-lead---sub-ebs_32)                          [DATA 8])
(defun (prc-modexp-lead---ebs-is-zero)                         OUTGOING_RES_LO)
(defun (prc-modexp-lead---ebs-less-than_32)                    (next OUTGOING_RES_LO))
(defun (prc-modexp-lead---call-data-contains-exponent-bytes)   (shift OUTGOING_RES_LO 2))
(defun (prc-modexp-lead---comp)                                (shift OUTGOING_RES_LO 3))

(defconstraint prc-modexp-lead---check-ebs-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
  (call-to-ISZERO 0 0 (prc-modexp-lead---ebs)))

(defconstraint prc-modexp-lead---compare-ebs-against-32 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
  (call-to-LT 1 0 (prc-modexp-lead---ebs) 0 32))

(defconstraint prc-modexp-lead---compare-ebs-against-cds (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
  (call-to-LT 2 0 (+ 96 (prc-modexp-lead---bbs)) 0 (prc---cds)))

(defconstraint prc-modexp-lead---compare-cds-minus-96-plus-bbs-against-32 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
  (if-not-zero (prc-modexp-lead---call-data-contains-exponent-bytes)
               (call-to-LT 3
                           0
                           (- (prc---cds) (+ 96 (prc-modexp-lead---bbs)))
                           0
                           32)))

(defconstraint prc-modexp-lead---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
  (begin (eq! (prc-modexp-lead---load-lead)
              (* (prc-modexp-lead---call-data-contains-exponent-bytes)
                 (- 1 (prc-modexp-lead---ebs-is-zero))))
         (if-zero (prc-modexp-lead---call-data-contains-exponent-bytes)
                  (vanishes! (prc-modexp-lead---cds-cutoff))
                  (if-zero (prc-modexp-lead---comp)
                           (eq! (prc-modexp-lead---cds-cutoff) 32)
                           (eq! (prc-modexp-lead---cds-cutoff)
                                (- (prc---cds) (+ 96 (prc-modexp-lead---bbs))))))
         (if-zero (prc-modexp-lead---ebs-less-than_32)
                  (eq! (prc-modexp-lead---ebs-cutoff) 32)
                  (eq! (prc-modexp-lead---ebs-cutoff) (prc-modexp-lead---ebs)))
         (if-zero (prc-modexp-lead---ebs-less-than_32)
                  (eq! (prc-modexp-lead---sub-ebs_32) (- (prc-modexp-lead---ebs) 32))
                  (vanishes! (prc-modexp-lead---sub-ebs_32)))))
