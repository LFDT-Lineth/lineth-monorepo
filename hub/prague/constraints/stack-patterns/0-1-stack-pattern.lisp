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
;;  5.8 (0,1)-StackPattern  ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (stack-pattern-0-1)
                (begin
                    ;; stack item 1:
                    (empty-stack-item 1)
                    ;; stack item 2:
                    (empty-stack-item 2)
                    ;; stack item 3:
                    (empty-stack-item 3)
                    ;; stack item 4:
                    (inc-frst-row-stack-item-X-height-by-Y       4 1)
                    (push-frst-row-stack-item                    4)
                    (set-frst-row-stack-item-stamp               4 0)
                    ;; height update;
                    (debug (eq!  HEIGHT_NEW  (+  HEIGHT 1)))))

