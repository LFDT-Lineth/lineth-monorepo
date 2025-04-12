(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.2 binary constraints   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint wcp-add-mod-are-exclusive ()
  (is-binary (lookup-sum 0)))

;; others are done with binary@prove in columns.lisp
