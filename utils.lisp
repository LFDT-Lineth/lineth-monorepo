;;;;;;;;;;;;;;;;;;;;
;;                ;;
;;      Data      ;;
;;                ;;
;;;;;;;;;;;;;;;;;;;;

(defun (num-zero-implies-zero NUM COL) (if-zero NUM (vanishes! COL)))

(defun (num-non-decreasing NUM)
                (or! (will-remain-constant! NUM) (inc NUM 1)))

(defun (index-grows-or-resets NUM INDEX)
                (if-zero (will-remain-constant! NUM)
                    (if-not-zero NUM (inc INDEX 1))
                    (will-eq! INDEX 0)))


;;;;;;;;;;;;;;;;;;;;
;;                ;;
;;      Info      ;;
;;                ;;
;;;;;;;;;;;;;;;;;;;;

;is anything here besides this ?

;;           /
;;          /
;; ________/
(defun (call-option-func NUM)
                (if-not-zero NUM
                    (inc NUM 1)))
