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
;;  5.2 POP_k binary conditions  ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; the [stack/STACK_ITEM_POP k] columns are :binary@prove

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
                    (vanishes! [ stack/STACK_ITEM_STAMP    k ])))

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

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;  5.4 emptyStackPattern  ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun  (empty-stack-pattern)  (for k [4] (empty-stack-item k))) ;; ""

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;  5.5 (0,0)-StackPattern  ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (stack-pattern-0-0)
    (begin
     (empty-stack-pattern)
     (debug (eq! HEIGHT_NEW HEIGHT))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;  5.6 (1,0)-StackPattern  ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (stack-pattern-1-0)
                (begin
                    ;; stack item 1:
                    (inc-frst-row-stack-item-X-height-by-Y   1 0)
                    (pop-frst-row-stack-item                 1)
                    (set-frst-row-stack-item-stamp           1 0)
                    ;; stack item 2:
                    (empty-stack-item 2)
                    ;; stack item 3:
                    (empty-stack-item 3)
                    ;; stack item 4:
                    (empty-stack-item 4)
                    ;; height update;
                    (debug (eq!  HEIGHT_NEW  (- HEIGHT 1)))))

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

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;  5.9 (1,1)-StackPattern  ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (stack-pattern-1-1)
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
                    (inc-frst-row-stack-item-X-height-by-Y       4 0)
                    (push-frst-row-stack-item                    4)
                    (set-frst-row-stack-item-stamp               4 1)
                    ;; height update;
                    (debug (eq!  HEIGHT_NEW  HEIGHT))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;  5.10 (2,1)-StackPattern  ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (stack-pattern-2-1)
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
                    (dec-frst-row-stack-item-X-height-by-Y       4 1)
                    (push-frst-row-stack-item                    4)
                    (set-frst-row-stack-item-stamp               4 2)
                    ;; height update;
                    (debug (eq!  HEIGHT_NEW  (- HEIGHT  1)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;  5.11 (3,1)-StackPattern  ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (stack-pattern-3-1)
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
                    (dec-frst-row-stack-item-X-height-by-Y       3 2)
                    (pop-frst-row-stack-item                     3)
                    (set-frst-row-stack-item-stamp               3 2)
                    ;; stack item 4:
                    (dec-frst-row-stack-item-X-height-by-Y       4 2)
                    (push-frst-row-stack-item                    4)
                    (set-frst-row-stack-item-stamp               4 3)
                    ;; height update;
                    (debug (eq!  HEIGHT_NEW  (- HEIGHT  2)))))

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

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;  5.14 swapStackPattern[param]  ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (swap-stack-pattern param)
                (begin
                    ;; stack item 1:
                    (dec-frst-row-stack-item-X-height-by-Y       1 param)
                    (pop-frst-row-stack-item                     1)
                    (set-frst-row-stack-item-stamp               1 0)
                    ;; stack item 2:
                    (inc-frst-row-stack-item-X-height-by-Y       2 0)
                    (pop-frst-row-stack-item                     2)
                    (set-frst-row-stack-item-stamp               2 1)
                    ;; stack item 3:
                    (dec-frst-row-stack-item-X-height-by-Y       3 param)
                    (push-frst-row-stack-item                    3)
                    (set-frst-row-stack-item-stamp               3 2)
                    (eq!  [ stack/STACK_ITEM_VALUE_HI 3 ]  [ stack/STACK_ITEM_VALUE_HI 2 ])
                    (eq!  [ stack/STACK_ITEM_VALUE_LO 3 ]  [ stack/STACK_ITEM_VALUE_LO 2 ])
                    ;; stack item 4:
                    (inc-frst-row-stack-item-X-height-by-Y       4 0)
                    (push-frst-row-stack-item                    4)
                    (set-frst-row-stack-item-stamp               4 3)
                    (eq!  [ stack/STACK_ITEM_VALUE_HI 4 ]  [ stack/STACK_ITEM_VALUE_HI 1 ])
                    (eq!  [ stack/STACK_ITEM_VALUE_LO 4 ]  [ stack/STACK_ITEM_VALUE_LO 1 ])
                    ;; height update;
                    (debug (eq!  HEIGHT_NEW  HEIGHT))))

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

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                        ;;
;;  5. requestHash and maybeRequestHash   ;;
;;                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun   (maybe-request-hash   relative_offset   bit) (eq!   (shift   stack/HASH_INFO_FLAG   relative_offset)
                                                             bit))

(defun   (request-hash   relative_offset)    (maybe-request-hash   relative_offset   1))
