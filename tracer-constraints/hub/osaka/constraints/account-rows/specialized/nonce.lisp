(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;   X.1.1 Introduction                ;;
;;   X.1.2 Conventions                 ;;
;;   X.1.3 Account nonce constraints   ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-same-nonce         relOffset)     (shift (eq! account/NONCE_NEW account/NONCE)       relOffset))
(defun (account-increment-nonce    relOffset)     (shift (eq! account/NONCE_NEW (+ account/NONCE 1)) relOffset))
(defun (account-undo-nonce-update  undoAt doneAt) (begin (eq! (shift account/NONCE_NEW undoAt) (shift account/NONCE      doneAt))
                                                         (eq! (shift account/NONCE     undoAt) (shift account/NONCE_NEW  doneAt))))

