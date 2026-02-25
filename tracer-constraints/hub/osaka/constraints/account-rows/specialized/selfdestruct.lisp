(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                       ;;
;;   X.1.8 Account MARKED_FOR_SELFDESTRUCT constraints   ;;
;;                                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-same-marked-for-deletion  kappa) (shift (eq! account/MARKED_FOR_DELETION_NEW account/MARKED_FOR_DELETION) kappa))
(defun (account-mark-account-for-deletion kappa) (shift (eq! account/MARKED_FOR_DELETION_NEW 1                              ) kappa))

