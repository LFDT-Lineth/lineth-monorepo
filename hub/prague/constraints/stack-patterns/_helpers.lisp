(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                    ;;;;
;;;;  5 Stack patterns  ;;;;
;;;;                    ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;  5.3 emptyStackItem_k  ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun  (empty-stack-item k)
                (begin
                    (vanishes! [ stack/STACK_ITEM_HEIGHT   k ])
                    (vanishes! [ stack/STACK_ITEM_POP      k ])
                    (debug (vanishes! [ stack/STACK_ITEM_VALUE_HI k ]))
                    (debug (vanishes! [ stack/STACK_ITEM_VALUE_LO k ]))
                    (vanishes! [ stack/STACK_ITEM_STAMP    k ])
		    ))

;; current row
(defun (set-frst-row-stack-item-stamp                    k offset   ) (eq! [ stack/STACK_ITEM_STAMP  k ] (+ (* MULTIPLIER___STACK_STAMP HUB_STAMP) offset)  ))
(defun (pop-frst-row-stack-item                          k          ) (eq! [ stack/STACK_ITEM_POP    k ] 1                                 ))
(defun (push-frst-row-stack-item                         k          ) (eq! [ stack/STACK_ITEM_POP    k ] 0                                 ))
(defun (inc-frst-row-stack-item-X-height-by-Y            k increment) (eq! [ stack/STACK_ITEM_HEIGHT k ] (+ HEIGHT increment)              ))
(defun (dec-frst-row-stack-item-X-height-by-Y            k decrement) (eq! [ stack/STACK_ITEM_HEIGHT k ] (- HEIGHT decrement)              )) ;; ""

;; next row
(defun (set-scnd-row-stack-item-stamp                    k offset   ) (eq! (next [ stack/STACK_ITEM_STAMP  k ]) (+ (* MULTIPLIER___STACK_STAMP HUB_STAMP) offset)  ))
(defun (pop-scnd-row-stack-item                          k          ) (eq! (next [ stack/STACK_ITEM_POP    k ]) 1                                 ))
(defun (push-scnd-row-stack-item                         k          ) (eq! (next [ stack/STACK_ITEM_POP    k ]) 0                                 ))
(defun (inc-scnd-row-stack-item-X-height-by-Y            k increment) (eq! (next [ stack/STACK_ITEM_HEIGHT k ]) (+ HEIGHT increment)              ))
(defun (dec-scnd-row-stack-item-X-height-by-Y            k decrement) (eq! (next [ stack/STACK_ITEM_HEIGHT k ]) (- HEIGHT decrement)              )) ;; ""

