(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                    ;;;;
;;;;  5 Stack patterns  ;;;;
;;;;                    ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;  5.7 (2,0)-StackPattern  ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (stack-pattern-2-0)
                (begin
                    ;; stack item 1:
                    (inc-frst-row-stack-item-X-height-by-Y       1 0)
                    (pop-frst-row-stack-item                     1)
                    (set-frst-row-stack-item-stamp               1 0)
                    ;; stack item 2:
                    (dec-frst-row-stack-item-X-height-by-Y       2 1)
                    (pop-frst-row-stack-item                     2)
                    (set-frst-row-stack-item-stamp               2 1)
                    ;; stack item 3:
                    (empty-stack-item 3)
                    ;; stack item 4:
                    (empty-stack-item 4)
                    ;; height update;
                    (debug (eq!  HEIGHT_NEW  (- HEIGHT 2)))))

