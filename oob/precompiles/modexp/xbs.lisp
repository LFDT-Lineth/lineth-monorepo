(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;   OOB_INST_MODEXP_xbs  ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconst
  EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND_PLUS_ONE     (+ EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND 1)

  ROFF___MODEXP_XBS___FIRST_ROW                   0
  ROFF___MODEXP_XBS___XBS_VS_EIP_7823_UPPER_BOUND 0
  ROFF___MODEXP_XBS___XBS_VS_YBS                  1
  ROFF___MODEXP_XBS___XBS_ISZERO_CHECK            2
  )


(defun (prc-modexp-xbs---standard-precondition)                              IS_MODEXP_XBS)
(defun (prc-modexp-xbs---xbs-hi)                                    (shift  [DATA   1]    ROFF___MODEXP_XBS___FIRST_ROW )   )  ;; ""
(defun (prc-modexp-xbs---xbs-lo)                                    (shift  [DATA   2]    ROFF___MODEXP_XBS___FIRST_ROW )   )  ;; ""
(defun (prc-modexp-xbs---ybs-lo)                                    (shift  [DATA   3]    ROFF___MODEXP_XBS___FIRST_ROW )   )  ;; ""
(defun (prc-modexp-xbs---compute-max)                   (force-bin  (shift  [DATA   4]    ROFF___MODEXP_XBS___FIRST_ROW ) ) )  ;; ""
(defun (prc-modexp-xbs---max-xbs-ybs)                               (shift  [DATA   7]    ROFF___MODEXP_XBS___FIRST_ROW )   )  ;; ""
(defun (prc-modexp-xbs---xbs-nonzero)                   (force-bin  (shift  [DATA   8]    ROFF___MODEXP_XBS___FIRST_ROW ) ) )  ;; ""
(defun (prc-modexp-xbs---xbs-within-bounds)             (force-bin  (shift  [DATA   9]    ROFF___MODEXP_XBS___FIRST_ROW ) ) )  ;; ""
(defun (prc-modexp-xbs---xbs-out-of-bounds)             (force-bin  (shift  [DATA  10]    ROFF___MODEXP_XBS___FIRST_ROW ) ) )  ;; ""
;; ""


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;   row i + 0: comparing xbs against 1024   ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    prc-modexp-xbs---comparing-xbs-against-EIP-7823-upper-bound
                  (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-LT   ROFF___MODEXP_XBS___XBS_VS_EIP_7823_UPPER_BOUND
                                (prc-modexp-xbs---xbs-hi)
                                (prc-modexp-xbs---xbs-lo)
                                0
                                EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND_PLUS_ONE
                                ))

(defun (prc-modexp-xbs---xbs-is-LE-the-EIP-7823-upper-bound)   (shift   OUTGOING_RES_LO   ROFF___MODEXP_XBS___XBS_VS_EIP_7823_UPPER_BOUND ))
(defun (prc-modexp-xbs---xbs-is-GT-the-EIP-7823-upper-bound)   (-  1  (prc-modexp-xbs---xbs-is-LE-the-EIP-7823-upper-bound)))

(defun (prc-modexp-xbs---xbs-normalized)   (*   (prc-modexp-xbs---xbs-lo)   (prc-modexp-xbs---xbs-is-LE-the-EIP-7823-upper-bound)))
(defun (prc-modexp-xbs---ybs-normalized)   (*   (prc-modexp-xbs---ybs-lo)   (prc-modexp-xbs---xbs-is-LE-the-EIP-7823-upper-bound)))



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;   row i + 1: comparing xbs against ybs   ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    prc-modexp-xbs---comparing-xbs-against-ybs
                  (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-LT   ROFF___MODEXP_XBS___XBS_VS_YBS
                                0
                                (prc-modexp-xbs---xbs-normalized)
                                0
                                (prc-modexp-xbs---ybs-normalized)
                                ))

(defun (prc-modexp-xbs---xbs-is-LT-ybs)   (shift   OUTGOING_RES_LO   ROFF___MODEXP_XBS___XBS_VS_YBS ))



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;   row i + 2: zeroness check for xbs   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    prc-modexp-xbs---is-zero-check-for-xbs
                  (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-ISZERO    ROFF___MODEXP_XBS___XBS_ISZERO_CHECK
                                     0
                                     (prc-modexp-xbs---xbs-normalized)
                                     ))

(defun (prc-modexp-xbs---xbs-is-zero)    (shift   OUTGOING_RES_LO   ROFF___MODEXP_XBS___XBS_ISZERO_CHECK ))



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;   justifying HUB predictions   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    prc-modexp-xbs---binarity-sanity-check
                  (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (is-binary   (prc-modexp-xbs---compute-max)))


(defconstraint   prc-modexp-xbs---justifying-hub-predictions---setting-xbs-nonzero
                 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (prc-modexp-xbs---xbs-nonzero)
                        (- 1 (prc-modexp-xbs---xbs-is-zero))
                        ))

(defconstraint   prc-modexp-xbs---justifying-hub-predictions---setting-xbs-within-bounds
                 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (prc-modexp-xbs---xbs-within-bounds)
                        (prc-modexp-xbs---xbs-is-LE-the-EIP-7823-upper-bound)
                        ))

(defconstraint   prc-modexp-xbs---justifying-hub-predictions---setting-xbs-out-of-bounds
                 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (prc-modexp-xbs---xbs-out-of-bounds)
                        (prc-modexp-xbs---xbs-is-GT-the-EIP-7823-upper-bound)
                        ))


(defconstraint   prc-modexp-xbs---justifying-hub-predictions---setting-the-value-of-max-xbs-ybs
                 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero    (prc-modexp-xbs---compute-max)
                             ;; comupte_max = false
                             (vanishes!   (prc-modexp-xbs---max-xbs-ybs))
                             ;; comupte_max = false
                             (if-zero     (prc-modexp-xbs---xbs-is-LE-the-EIP-7823-upper-bound)
                                          ;; false case
                                          (vanishes!   (prc-modexp-xbs---max-xbs-ybs))
                                          ;; true case
                                          (if-zero     (prc-modexp-xbs---xbs-is-LT-ybs)
                                                       ;; false case
                                                       (eq!   (prc-modexp-xbs---max-xbs-ybs)
                                                              (prc-modexp-xbs---xbs-normalized))
                                                       ;; true case
                                                       (eq!   (prc-modexp-xbs---max-xbs-ybs)
                                                              (prc-modexp-xbs---ybs-normalized))
                                                       ))))
