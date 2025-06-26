(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                    ;;;;
;;;;  5 Stack patterns  ;;;;
;;;;                    ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;  5.18 createStackPattern[b]  ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (create-stack-pattern (b :binary))
                (begin
                    ;; stack item 1:
                    (dec-frst-row-stack-item-X-height-by-Y       1 1)
                    (pop-frst-row-stack-item                     1)
                    (set-frst-row-stack-item-stamp               1 1)
                    ;; stack item 2:
                    (dec-frst-row-stack-item-X-height-by-Y       2 2)
                    (pop-frst-row-stack-item                     2)
                    (set-frst-row-stack-item-stamp               2 2)
                    ;; stack item 3:
                    (empty-stack-item 3)
                    ;; stack item 4:
                    (empty-stack-item 4)
                    ;; height update;
                    (debug (eq! HEIGHT_NEW (- HEIGHT 2 b)))
                    ;; stack item 5;
                    (next (empty-stack-item 1))
                    ;; stack item 6;
                    (will-eq! [ stack/STACK_ITEM_HEIGHT 2 ]  (*  b (- HEIGHT 3)))
                    (will-eq! [ stack/STACK_ITEM_POP    2 ]      b)
                    (will-eq! [ stack/STACK_ITEM_STAMP  2 ]  (*  b (+ (* MULTIPLIER___STACK_STAMP HUB_STAMP) 3)))
                    ;; stack item 7;
                    (dec-scnd-row-stack-item-X-height-by-Y          3 0)
                    (pop-scnd-row-stack-item                        3)
                    (set-scnd-row-stack-item-stamp                  3 0)
                    ;; stack item 8;
                    (dec-scnd-row-stack-item-X-height-by-Y          4 (+ 2 b))
                    (push-scnd-row-stack-item                       4)
                    (set-scnd-row-stack-item-stamp                  4 4)
                    ))

