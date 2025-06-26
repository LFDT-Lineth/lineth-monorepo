(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                    ;;;;
;;;;  5 Stack patterns  ;;;;
;;;;                    ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;  5.13 dupStackPattern[param]  ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (dup-stack-pattern param)
                (begin
                    ;; stack item 1:
                    (dec-frst-row-stack-item-X-height-by-Y       1 param)
                    (pop-frst-row-stack-item                     1)
                    (set-frst-row-stack-item-stamp               1 0)
                    ;; stack item 2:
                    (dec-frst-row-stack-item-X-height-by-Y       2 param)
                    (push-frst-row-stack-item                    2)
                    (set-frst-row-stack-item-stamp               2 1)
                    (eq!  [ stack/STACK_ITEM_VALUE_HI 2 ]  [ stack/STACK_ITEM_VALUE_HI 1 ])
                    (eq!  [ stack/STACK_ITEM_VALUE_LO 2 ]  [ stack/STACK_ITEM_VALUE_LO 1 ])
                    ;; stack item 3:
                    (empty-stack-item 3)
                    ;; stack item 4:
                    (inc-frst-row-stack-item-X-height-by-Y       4 1)
                    (push-frst-row-stack-item                    4)
                    (set-frst-row-stack-item-stamp               4 2)
                    (eq!  [ stack/STACK_ITEM_VALUE_HI 4 ]  [ stack/STACK_ITEM_VALUE_HI 1 ])
                    (eq!  [ stack/STACK_ITEM_VALUE_LO 4 ]  [ stack/STACK_ITEM_VALUE_LO 1 ])
                    ;; height update;
                    (debug (eq!  HEIGHT_NEW  (+  HEIGHT  1)))))

