(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;   X.1.6 Account warmth constraints   ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-same-warmth         kappa)          (shift (eq! account/WARMTH_NEW account/WARMTH) kappa))
(defun (account-turn-on-warmth      kappa)          (shift (eq! account/WARMTH_NEW 1             ) kappa))
(defun (account-undo-warmth-update  undoAt doneAt)  (begin (eq! (shift account/WARMTH_NEW undoAt) (shift account/WARMTH     doneAt))
                                                           (eq! (shift account/WARMTH     undoAt) (shift account/WARMTH_NEW doneAt))))

