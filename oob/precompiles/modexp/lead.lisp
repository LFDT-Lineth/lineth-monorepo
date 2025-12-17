(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;   OOB_INST_MODEXP_lead   ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconst
  ROFF___MODEXP_LEAD___FIRST_ROW                      0
  ROFF___MODEXP_LEAD___EBS_IS_ZERO_CHECK              0
  ROFF___MODEXP_LEAD___EBS_VS_32                      1
  ROFF___MODEXP_LEAD___DOES_CDS_EXTEND_BEYOND_BASE    2
  ROFF___MODEXP_LEAD___COMP_TO_DETERMINE_CDS_CUTOFF   3

  CUMULATIVE_LENGTH_OF_BYTE_SIZES                (*   3   WORD_SIZE)
  )



(defun (prc-modexp-lead---standard-precondition)               IS_MODEXP_LEAD                 )
(defun (prc-modexp-lead---bbs)                                 (shift   [DATA 1]   ROFF___MODEXP_LEAD___FIRST_ROW ) )
(defun (prc-modexp-lead---ebs)                                 (shift   [DATA 3]   ROFF___MODEXP_LEAD___FIRST_ROW ) )
(defun (prc-modexp-lead---extract-leading-word)                (shift   [DATA 4]   ROFF___MODEXP_LEAD___FIRST_ROW ) )
(defun (prc-modexp-lead---cds-cutoff)                          (shift   [DATA 6]   ROFF___MODEXP_LEAD___FIRST_ROW ) )
(defun (prc-modexp-lead---ebs-cutoff)                          (shift   [DATA 7]   ROFF___MODEXP_LEAD___FIRST_ROW ) )
(defun (prc-modexp-lead---sub-ebs_32)                          (shift   [DATA 8]   ROFF___MODEXP_LEAD___FIRST_ROW ) )
;; ""
(defun (prc-modexp-lead---ebs-is-zero)                         (force-bin   (shift   OUTGOING_RES_LO   ROFF___MODEXP_LEAD___EBS_IS_ZERO_CHECK            ) ) )
(defun (prc-modexp-lead---ebs-less-than_32)                    (force-bin   (shift   OUTGOING_RES_LO   ROFF___MODEXP_LEAD___EBS_VS_32                    ) ) )
(defun (prc-modexp-lead---call-data-extends-beyond-the-base)   (force-bin   (shift   OUTGOING_RES_LO   ROFF___MODEXP_LEAD___DOES_CDS_EXTEND_BEYOND_BASE  ) ) )
(defun (prc-modexp-lead---result-of-comparison)                (force-bin   (shift   OUTGOING_RES_LO   ROFF___MODEXP_LEAD___COMP_TO_DETERMINE_CDS_CUTOFF ) ) ) ;; ""



(defconstraint    prc-modexp-lead---check-ebs-is-zero 
                  (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-ISZERO    ROFF___MODEXP_LEAD___EBS_IS_ZERO_CHECK
                                     0
                                     (prc-modexp-lead---ebs)
                                     ))

(defconstraint    prc-modexp-lead---compare-ebs-against-32
                  (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-LT   ROFF___MODEXP_LEAD___EBS_VS_32
                                0
                                (prc-modexp-lead---ebs)
                                0
                                WORD_SIZE
                                ))

(defconstraint    prc-modexp-lead---compare-ebs-against-cds
                  (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-LT   ROFF___MODEXP_LEAD___DOES_CDS_EXTEND_BEYOND_BASE
                                0
                                (+ CUMULATIVE_LENGTH_OF_BYTE_SIZES (prc-modexp-lead---bbs))
                                0
                                (prc---cds)
                                ))

(defconstraint    prc-modexp-lead---compare-cds-minus-96-plus-bbs-against-32
                  (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero (prc-modexp-lead---call-data-extends-beyond-the-base)
                               (call-to-LT   ROFF___MODEXP_LEAD___COMP_TO_DETERMINE_CDS_CUTOFF
                                             0
                                             (- (prc---cds) (+ CUMULATIVE_LENGTH_OF_BYTE_SIZES (prc-modexp-lead---bbs)))
                                             0
                                             WORD_SIZE
                                             )))

(defconstraint    prc-modexp-lead---justify-hub-predictions
                  (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin (eq! (prc-modexp-lead---extract-leading-word)
                              (* (prc-modexp-lead---call-data-extends-beyond-the-base)
                                 (- 1 (prc-modexp-lead---ebs-is-zero))))
                         (if-zero (prc-modexp-lead---call-data-extends-beyond-the-base)
                                  (vanishes! (prc-modexp-lead---cds-cutoff))
                                  (if-zero (prc-modexp-lead---result-of-comparison)
                                           (eq! (prc-modexp-lead---cds-cutoff) WORD_SIZE)
                                           (eq! (prc-modexp-lead---cds-cutoff)
                                                (- (prc---cds) (+ CUMULATIVE_LENGTH_OF_BYTE_SIZES (prc-modexp-lead---bbs))))))
                         (if-zero (prc-modexp-lead---ebs-less-than_32)
                                  (eq! (prc-modexp-lead---ebs-cutoff) WORD_SIZE)
                                  (eq! (prc-modexp-lead---ebs-cutoff) (prc-modexp-lead---ebs)))
                         (if-zero (prc-modexp-lead---ebs-less-than_32)
                                  (eq! (prc-modexp-lead---sub-ebs_32) (- (prc-modexp-lead---ebs) WORD_SIZE))
                                  (vanishes! (prc-modexp-lead---sub-ebs_32)))))
