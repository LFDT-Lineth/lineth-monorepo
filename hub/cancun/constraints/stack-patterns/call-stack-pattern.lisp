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
;;  5.17 callStackPattern[b]  ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (call-stack-pattern (b :binary))
                (begin
                    ;; stack item 1:
                    (dec-frst-row-stack-item-X-height-by-Y       1 (+ 2 b))
                    (pop-frst-row-stack-item                     1)
                    (set-frst-row-stack-item-stamp               1 3)
                    ;; stack item 2:
                    (dec-frst-row-stack-item-X-height-by-Y       2 (+ 3 b))
                    (pop-frst-row-stack-item                     2)
                    (set-frst-row-stack-item-stamp               2 4)
                    ;; stack item 3:
                    (dec-frst-row-stack-item-X-height-by-Y       3 (+ 4 b))
                    (pop-frst-row-stack-item                     3)
                    (set-frst-row-stack-item-stamp               3 5)
                    ;; stack item 4:
                    (dec-frst-row-stack-item-X-height-by-Y       4 (+ 5 b))
                    (pop-frst-row-stack-item                     4)
                    (set-frst-row-stack-item-stamp               4 6)
                    ;; height update;
                    (debug (eq!  HEIGHT_NEW  (- HEIGHT  5 b)))
                    ;; stack item 5;
                    (dec-scnd-row-stack-item-X-height-by-Y       1 0)
                    (pop-scnd-row-stack-item                     1)
                    (set-scnd-row-stack-item-stamp               1 0)
                    ;; stack item 6;
                    (dec-scnd-row-stack-item-X-height-by-Y       2 1)
                    (pop-scnd-row-stack-item                     2)
                    (set-scnd-row-stack-item-stamp               2 1)
                    ;; stack item 7;
                    (will-eq! [ stack/STACK_ITEM_HEIGHT 3 ]   (* b (- HEIGHT 2)))
                    (will-eq! [ stack/STACK_ITEM_POP    3 ]      b )
                    (will-eq! [ stack/STACK_ITEM_STAMP  3 ]   (* b (+ (* MULTIPLIER___STACK_STAMP HUB_STAMP) 2)))
                    ;; stack item 8;
                    (dec-scnd-row-stack-item-X-height-by-Y       4 (+ 5 b))
                    (push-scnd-row-stack-item                    4)
                    (set-scnd-row-stack-item-stamp               4 7)
                    (vanishes!   (next  [ stack/STACK_ITEM_VALUE_HI 4 ]))
                    (is-binary   (next  [ stack/STACK_ITEM_VALUE_LO 4 ]))
                    )
                )
