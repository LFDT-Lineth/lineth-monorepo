(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                    ;;;;
;;;;  5 Stack patterns  ;;;;
;;;;                    ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;  5.12 loadStoreStackPattern[b]  ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (load-store-stack-pattern (b :binary))
                (begin
                    ;; stack item 1:
                    (inc-frst-row-stack-item-X-height-by-Y       1 0)
                    (pop-frst-row-stack-item                     1)
                    (set-frst-row-stack-item-stamp               1 0)
                    ;; stack item 2:
                    (empty-stack-item 2)
                    ;; stack item 3:
                    (empty-stack-item 3)
                    ;; stack item 4:
                    (dec-frst-row-stack-item-X-height-by-Y       4 b)
                    (eq! [ stack/STACK_ITEM_POP                    4 ]  b)
                    (set-frst-row-stack-item-stamp               4 1)
                    ;; height update;
                    (debug (eq!  HEIGHT_NEW  (- HEIGHT b b)))))

