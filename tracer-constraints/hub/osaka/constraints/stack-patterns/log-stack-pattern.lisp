(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                    ;;;;
;;;;  5 Stack patterns  ;;;;
;;;;                    ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                ;;
;;  5.15 logStackPattern[param, b1, b2, b3, b4]   ;;
;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun  (b-sum-1 b1 b2 b3 b4) (+ b1 b2 b3 b4))
(defun  (b-sum-2 b2 b3 b4)    (+    b2 b3 b4))
(defun  (b-sum-3 b3 b4)       (+       b3 b4))
(defun  (b-sum-4 b4)                      b4 )

(defun (log-stack-pattern param (b1 :binary) (b2 :binary) (b3 :binary) (b4 :binary))
                (begin
                    ;; stack item 1:
                    (dec-frst-row-stack-item-X-height-by-Y       1 0)
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
                    (debug (eq!  HEIGHT_NEW  (- HEIGHT  param 2)))
                    ;; stack item 5:
                    (will-eq! [ stack/STACK_ITEM_HEIGHT 1 ]       (* (b-sum-1 b1 b2 b3 b4) (- HEIGHT 2)))
                    (will-eq! [ stack/STACK_ITEM_POP    1 ]          (b-sum-1 b1 b2 b3 b4))
                    (will-eq! [ stack/STACK_ITEM_STAMP  1 ]       (* (b-sum-1 b1 b2 b3 b4) (+ (* MULTIPLIER___STACK_STAMP HUB_STAMP) 2)))
                    ;; stack item 6:
                    (will-eq! [ stack/STACK_ITEM_HEIGHT 2 ]       (* (b-sum-2 b2 b3 b4) (- HEIGHT 3)))
                    (will-eq! [ stack/STACK_ITEM_POP    2 ]          (b-sum-2 b2 b3 b4))
                    (will-eq! [ stack/STACK_ITEM_STAMP  2 ]       (* (b-sum-2 b2 b3 b4) (+ (* MULTIPLIER___STACK_STAMP HUB_STAMP) 3)))
                    ;; stack item 7:
                    (will-eq! [ stack/STACK_ITEM_HEIGHT 3 ]       (* (b-sum-3 b3 b4) (- HEIGHT 4)))
                    (will-eq! [ stack/STACK_ITEM_POP    3 ]          (b-sum-3 b3 b4))
                    (will-eq! [ stack/STACK_ITEM_STAMP  3 ]       (* (b-sum-3 b3 b4) (+ (* MULTIPLIER___STACK_STAMP HUB_STAMP) 4)))
                    ;; stack item 8:
                    (will-eq! [ stack/STACK_ITEM_HEIGHT 4 ]       (* (b-sum-4 b4) (- HEIGHT 5)))
                    (will-eq! [ stack/STACK_ITEM_POP    4 ]          (b-sum-4 b4))
                    (will-eq! [ stack/STACK_ITEM_STAMP  4 ]       (* (b-sum-4 b4) (+ (* MULTIPLIER___STACK_STAMP HUB_STAMP) 5)))))
