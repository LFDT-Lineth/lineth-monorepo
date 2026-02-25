(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;   X.1.5 Account byte code constraints   ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-same-code-size         relOffset)     (shift (eq! account/CODE_SIZE_NEW account/CODE_SIZE)              relOffset))
(defun (account-same-code-hash         relOffset)     (begin (shift (eq! account/CODE_HASH_HI_NEW account/CODE_HASH_HI) relOffset)
                                                             (shift (eq! account/CODE_HASH_LO_NEW account/CODE_HASH_LO) relOffset)))
(defun (account-same-code              relOffset)     (begin (account-same-code-size relOffset)
                                                             (account-same-code-hash relOffset)))

(defun (account-undo-code-size-update  undoAt doneAt)     (begin (eq! (shift account/CODE_SIZE_NEW      undoAt) (shift account/CODE_SIZE          doneAt))
                                                                 (eq! (shift account/CODE_SIZE          undoAt) (shift account/CODE_SIZE_NEW      doneAt))))
(defun (account-undo-code-hash-update  undoAt doneAt)     (begin (eq! (shift account/CODE_HASH_HI_NEW   undoAt) (shift account/CODE_HASH_HI       doneAt))
                                                                 (eq! (shift account/CODE_HASH_HI       undoAt) (shift account/CODE_HASH_HI_NEW   doneAt))
                                                                 (eq! (shift account/CODE_HASH_LO_NEW   undoAt) (shift account/CODE_HASH_LO       doneAt))
                                                                 (eq! (shift account/CODE_HASH_LO       undoAt) (shift account/CODE_HASH_LO_NEW   doneAt))))
(defun (account-undo-code-update       undoAt doneAt)     (begin (account-undo-code-size-update undoAt doneAt)
                                                                 (account-undo-code-hash-update undoAt doneAt)))

