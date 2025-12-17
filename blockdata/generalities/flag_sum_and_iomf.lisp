(module blockdata)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;  2.X flag_sum and IOMF constraints  ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (about-to-start-new-block)    (*  (-  1  IS_CB)
					    (next  IS_CB)))

;; iomf binarity ensured by binary@prove

(defconstraint    iomf-and-flag-sum---IOMF-vanishes-initially (:domain {0}) ;; ""
		  (vanishes!    IOMF))

(defconstraint    iomf-and-flag-sum---setting-next-IOMF-value ()
		  (if-zero    IOMF
			      ;; IOMF ≡ 0
			      (eq!  (next  IOMF)  (about-to-start-new-block))
			      ;; IOMF ≡ 0
			      (eq!  (next  IOMF)  1)
			      ))

(defconstraint    iomf-and-flag-sum---pegging-IOMF-against-flag-sum ()
		  (eq!    IOMF
			  (flag-sum)))

