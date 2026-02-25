(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;   X.1.4 Account balance constraints   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-same-balance           relOffset)          (eq!       (shift    account/BALANCE_NEW    relOffset)         (shift account/BALANCE   relOffset)         ))         ;; relOffset rows into the future
(defun (account-increment-balance-by   relOffset value)    (eq!       (shift    account/BALANCE_NEW    relOffset)    (+   (shift account/BALANCE   relOffset)   value)))         ;; relOffset rows into the future
(defun (account-decrement-balance-by   relOffset value)    (eq!       (shift    account/BALANCE_NEW    relOffset)    (-   (shift account/BALANCE   relOffset)   value)))         ;; relOffset rows into the future
(defun (account-undo-balance-update    undoAt doneAt)      (begin
                                                             (eq!     (shift   account/BALANCE_NEW   undoAt)   (shift   account/BALANCE       doneAt))   ;; same as relOffset rows into the past
                                                             (eq!     (shift   account/BALANCE       undoAt)   (shift   account/BALANCE_NEW   doneAt))   ;; same as relOffset rows into the past
                                                             ))

