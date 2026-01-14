(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                    ;;;;
;;;;  5 Stack patterns  ;;;;
;;;;                    ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;  5.16 copyStackPattern[b]  ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (copy-stack-pattern (b :binary))
                (begin
                    ;; stack item 1:
                    (dec-frst-row-stack-item-X-height-by-Y       1 b)
                    (pop-frst-row-stack-item                     1)
                    (set-frst-row-stack-item-stamp               1 1)
                    ;; stack item 2:
                    (dec-frst-row-stack-item-X-height-by-Y       2 (+ 2 b))
                    (pop-frst-row-stack-item                     2)
                    (set-frst-row-stack-item-stamp               2 2)
                    ;; stack item 3:
                    (dec-frst-row-stack-item-X-height-by-Y       3 (+ 1 b))
                    (pop-frst-row-stack-item                     3)
                    (set-frst-row-stack-item-stamp               3 3)
                    ;; stack item 4:
                    (eq!  [ stack/STACK_ITEM_HEIGHT   4 ]  (* b HEIGHT))
                    (eq!  [ stack/STACK_ITEM_POP      4 ]     b)
                    (eq!  [ stack/STACK_ITEM_STAMP    4 ]  (* b MULTIPLIER___STACK_STAMP HUB_STAMP))
                    ;; height update;
                    (debug (eq!  HEIGHT_NEW  (- HEIGHT  3 b)))
                    ))

