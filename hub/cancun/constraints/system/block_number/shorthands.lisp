(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;   X.Y The BLK_NUMBER column   ;;
;;   X.Y.Z Shorthands            ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (zero-now-one-next    col)    (*    (-  1  col)    (next    col)))

(defun    (system-block-number---about-to-enter-sysi)    (zero-now-one-next  SYSI))
(defun    (system-block-number---about-to-enter-user)    (zero-now-one-next  USER))
(defun    (system-block-number---about-to-enter-sysf)    (zero-now-one-next  SYSF))
(defun    (system-block-number---about-to-transition)    (+    (system-block-number---about-to-enter-sysi)
							       (system-block-number---about-to-enter-user)
							       (system-block-number---about-to-enter-sysf)))

