(module hub)

(defun (make-empty-stack-item i)
    (begin (vanishes! [ITEM_HEIGHT i])
           (vanishes! [POP i])
           (vanishes! [VAL_HI i])
           (vanishes! [VAL_LO i])
           (vanishes! [ITEM_STACK_STAMP i])))

(defun (make-stack-item i height pop stamp)
    (begin (eq! [ITEM_HEIGHT i] height)
           (eq! [POP i] pop)
           (eq! [ITEM_STACK_STAMP i] stamp)))

(defun (make-stack-item-with-value i height pop stamp val-hi val-lo)
    (begin (make-stack-item i height pop stamp)
           (eq! [VAL_HI i] val-hi)
           (eq! [VAL_LO i] val-lo)))

(defun (standard-regime) (* INSTRUCTION_STAMP (not STACK_EXCEPTION)))

(definrange HEIGHT          1025)
(definrange HEIGHT_NEW      1025)
(definrange HEIGHT_UNDER    1025)
(definrange HEIGHT_OVER     1025)

(defconstraint stack-exception (:guard (* INSTRUCTION_STAMP STACK_EXCEPTION))
  (begin
   (for i [1:4] (make-empty-stack-item i))
   (eq! STACK_STAMP_NEW STACK_STAMP)
   (vanishes! HEIGHT_NEW)))

(defconstraint zero-rows-exp (:guard (is-zero INSTRUCTION_STAMP))
  (begin
   (vanishes! STACK_STAMP)
   (vanishes! HEIGHT)
   (vanishes! HEIGHT_NEW)
   (vanishes! INSTRUCTION)
   (vanishes! INSTRUCTION_ARGUMENT_HI)
   (vanishes! INSTRUCTION_ARGUMENT_LO)
   (vanishes! STATIC_GAS)
   (vanishes! INST_PARAM)
   (vanishes! TWO_LINES_INSTRUCTION)
   (vanishes! STACK_PATTERN)
   (vanishes! FLAG_1)
   (vanishes! FLAG_2)
   (vanishes! FLAG_3)
   (for i [1:4] (begin
                 (vanishes! [ITEM_HEIGHT i])
                 (vanishes! [VAL_HI i])
                 (vanishes! [VAL_LO i])
                 (vanishes! [POP i])
                 (vanishes! [ITEM_STACK_STAMP i])))
   (vanishes! STACK_EXCEPTION)
   (vanishes! STACK_UNDERFLOW_EXCEPTION)
   (vanishes! STACK_OVERFLOW_EXCEPTION)))

(defconstraint stack-exception-constraints (:guard (* INSTRUCTION_STAMP (is-not-zero STACK_EXCEPTION)))
  (begin
   (eq! HEIGHT_UNDER
        (- (* (- (* 2 STACK_UNDERFLOW_EXCEPTION) 1)
              (- DELTA HEIGHT))
           STACK_UNDERFLOW_EXCEPTION))

   (if-zero (eq! STACK_UNDERFLOW_EXCEPTION 1)
            (vanishes! STACK_OVERFLOW_EXCEPTION))

   (if-zero STACK_UNDERFLOW_EXCEPTION
            (eq! HEIGHT_OVER
                 (- (* (- (* 2 STACK_OVERFLOW_EXCEPTION) 1)
                       (- (+ HEIGHT_UNDER ALPHA) 1024))
                    STACK_OVERFLOW_EXCEPTION)))

   (eq! STACK_EXCEPTION (+ STACK_OVERFLOW_EXCEPTION STACK_UNDERFLOW_EXCEPTION))))

(defconstraint heartbeat-init (:domain {0}) (vanishes! INSTRUCTION_STAMP))


(defconstraint heartbeat ()
  (begin ;; INSTRUCTION_STAMP remains constant or increases by 1
   (vanishes!
    (*     (will-remain-constant! INSTRUCTION_STAMP)
           (will-inc! INSTRUCTION_STAMP 1)))

   (if-zero TWO_LINES_INSTRUCTION
            ;; TLI == 0
            (begin (vanishes! COUNTER)
                   (vanishes! (next COUNTER)))
            ;; TLI == 1
            (if-zero COUNTER
                     (begin (will-remain-constant! INSTRUCTION_STAMP)
                            (will-remain-constant! INSTRUCTION)
                            (will-inc! COUNTER 1))
                     (begin (vanishes! (next COUNTER))
                            (will-inc! INSTRUCTION_STAMP 1))))))

(defconstraint counter-constancies (:guard (standard-regime))
  (if-zero (eq! COUNTER 1)
           (begin
            (remained-constant! HEIGHT)
            (remained-constant! HEIGHT_NEW)
            (remained-constant! HEIGHT_UNDER)
            (remained-constant! HEIGHT_OVER))))

(defconstraint pattern-0 (:guard (standard-regime))
  (if-zero (eq! STACK_PATTERN PATTERN_ZERO_ITEMS)
           (begin
            ;; stack items
            (make-empty-stack-item 1)
            (make-empty-stack-item 2)
            (make-empty-stack-item 3)
            (make-empty-stack-item 4)
            ;; stamp update
            (eq! STACK_STAMP STACK_STAMP_NEW)
            ;; height update
            (eq! HEIGHT HEIGHT_NEW))))

(defconstraint pattern-1 (:guard (standard-regime))
  (if-zero (eq! STACK_PATTERN PATTERN_ONE_ITEM)
           (begin
            ;; stack items
            (make-stack-item       1 (* FLAG_1 HEIGHT)             FLAG_1 (* FLAG_1 (+ STACK_STAMP 1)))
            (make-empty-stack-item 2)
            (make-empty-stack-item 3)
            (make-stack-item       4 (* (- 1 FLAG_1) (+ HEIGHT 1)) 0      (* (- 1 FLAG_1) (+ 1 STACK_STAMP)))
            ;; stamp update
            (eq! STACK_STAMP_NEW (+ 1 STACK_STAMP))
            ;; height update
            (eq! HEIGHT_NEW (+ HEIGHT (- 1 (* 2 FLAG_1)))))))

(defconstraint pattern-2 (:guard (standard-regime))
  (if-zero (eq! STACK_PATTERN PATTERN_TWO_ITEMS)
           (begin
            ;; stack items
            (make-stack-item       1 HEIGHT            1      (+ 1 STACK_STAMP))
            (make-empty-stack-item 2)
            (make-empty-stack-item 3)
            (make-stack-item       4 (- HEIGHT FLAG_1) FLAG_1 (+ 2 STACK_STAMP))
            ;; stamp update
            (eq! STACK_STAMP_NEW (+ 2 STACK_STAMP))
            ;; height update
            (eq! HEIGHT_NEW (- HEIGHT (* 2 FLAG_1))))))

(defconstraint pattern-3 (:guard (standard-regime))
  (if-zero (eq! STACK_PATTERN PATTERN_STANDARD)
           (begin
            ;; stack items
            (make-stack-item 1 HEIGHT                  1      (+ STACK_STAMP 1))
            (make-stack-item 2 (* FLAG_1 (- HEIGHT 2)) FLAG_1 (* FLAG_1 (+ STACK_STAMP 2)))
            (make-stack-item 3 (- HEIGHT 1)            1      (+ STACK_STAMP 2 FLAG_1))
            (make-stack-item 4 (- HEIGHT 1 FLAG_1)     0      (+ STACK_STAMP 3 FLAG_1))
            ;; stamp update
            (eq! STACK_STAMP_NEW (+ STACK_STAMP 3 FLAG_1))
            ;; height update
            (eq! HEIGHT_NEW (- HEIGHT 1 FLAG_1)))))

(defconstraint pattern-dup (:guard (standard-regime))
  (if-zero (eq! STACK_PATTERN PATTERN_DUP)
           (begin
            ;; stack items
            (make-stack-item            1 (- HEIGHT INST_PARAM) 1 (+ STACK_STAMP 1))
            (make-empty-stack-item      2)
            (make-stack-item-with-value 3 (- HEIGHT INST_PARAM) 0 (+ STACK_STAMP 2) [VAL_HI 1] [VAL_LO 1])
            (make-stack-item-with-value 4 (+ HEIGHT 1)          0 (+ STACK_STAMP 3) [VAL_HI 1] [VAL_LO 1])
            ;; stamp update
            (eq! STACK_STAMP_NEW (+ STACK_STAMP 3))
            ;; height update
            (eq! HEIGHT_NEW (+ HEIGHT 1)))))

(defconstraint pattern-swap (:guard (standard-regime))
  (if-zero (eq! STACK_PATTERN PATTERN_SWAP)
           (begin
            ;; stack items
            (make-stack-item            1 (- HEIGHT INST_PARAM) 1 (+ STACK_STAMP 1))
            (make-stack-item            2 HEIGHT                1 (+ STACK_STAMP 2))
            (make-stack-item-with-value 3 (- HEIGHT INST_PARAM) 0 (+ STACK_STAMP 3) [VAL_HI 2] [VAL_LO 2])
            (make-stack-item-with-value 4 HEIGHT                0 (+ STACK_STAMP 4) [VAL_HI 1] [VAL_LO 1])
            ;; stamp update
            (eq! STACK_STAMP_NEW (+ STACK_STAMP 4))
            ;; height update
            (eq! HEIGHT_NEW HEIGHT))))

(defconstraint pattern-return-revert (:guard (standard-regime))
  (if-zero (eq! STACK_PATTERN PATTERN_RETURN_REVERT)
           (begin
            ;; stack items
            (make-stack-item            1 HEIGHT       1 (+ STACK_STAMP 1))
            (make-empty-stack-item      2)
            (make-stack-item            3 (- HEIGHT 1) 1 (+ STACK_STAMP 2))
            (make-stack-item-with-value 4 0            0 0                (* BYTECODE_ADDRESS_HI FLAG_1 CONTEXT_TYPE) (* BYTECODE_ADDRESS_LO FLAG_1 CONTEXT_TYPE))
            ;; stamp update
            (eq! STACK_STAMP_NEW (+ STACK_STAMP 2))
            ;; height update
            (eq! HEIGHT_NEW (- HEIGHT 2)))))

(defconstraint pattern-copy (:guard (standard-regime))
  (if-zero (eq! STACK_PATTERN PATTERN_COPY)
           (let ((is-EXTCODECOPY (* FLAG_1 FLAG_2)))
             (begin
              ;; stack items
              (make-stack-item 1 (- HEIGHT is-EXTCODECOPY)   1              (+ STACK_STAMP 1))
              (make-stack-item 2 (- HEIGHT is-EXTCODECOPY 1) 1              (+ STACK_STAMP 2))
              (make-stack-item 3 (- HEIGHT is-EXTCODECOPY 2) 1              (+ STACK_STAMP 3))
              (make-stack-item 4 (* HEIGHT is-EXTCODECOPY)   is-EXTCODECOPY (* (+ STACK_STAMP 4) is-EXTCODECOPY))
              (if-zero (eq! FLAG_1 1)
                       (if-zero FLAG_2
                                (begin (eq! [VAL_HI 4] BYTECODE_ADDRESS_HI)
                                       (eq! [VAL_LO 4] BYTECODE_ADDRESS_LO))))
              ;; stamp update
              (eq! STACK_STAMP_NEW (+ STACK_STAMP 3 is-EXTCODECOPY))
              ;; height update
              (eq! HEIGHT_NEW (- HEIGHT 3 is-EXTCODECOPY))))))

(defconstraint pattern-log (:guard (standard-regime))
  (if-zero (eq! STACK_PATTERN PATTERN_LOG)
           (let ((log-gt-2 (+ FLAG_2 (* FLAG_3 (- 1 FLAG_2))))
                 (is-log4 (* FLAG_2 FLAG_3)))
             (if-zero COUNTER
                      ;; counter = 0
                      (begin
                       ;; stack items - first line
                       (make-stack-item       1 HEIGHT       1 (+ STACK_STAMP 1))
                       (make-empty-stack-item 2)
                       (make-stack-item       3 (- HEIGHT 1) 1 (+ STACK_STAMP 2))
                       (make-empty-stack-item 4)
                       ;; stamp update
                       (eq! STACK_STAMP_NEW (+ STACK_STAMP 2 INST_PARAM))
                       ;; height update
                       (eq! HEIGHT_NEW (- HEIGHT 2 INST_PARAM)))
                      ;; counter = 1
                      (begin
                       ;; stack items - second lines
                       (make-stack-item 1 (* FLAG_1 (- HEIGHT 2))   FLAG_1   (* FLAG_1 (+ STACK_STAMP 3)))
                       (make-stack-item 2 (* log-gt-2 (- HEIGHT 3)) log-gt-2 (* log-gt-2 (+ STACK_STAMP 4)))
                       (make-stack-item 3 (* FLAG_2 (- HEIGHT 4))   FLAG_2   (* FLAG_2 (+ STACK_STAMP 5)))
                       (make-stack-item 4 (* is-log4 (- HEIGHT 5))  is-log4  (* is-log4 (+ STACK_STAMP 6))))))))

(defconstraint pattern-call (:guard (standard-regime))
  (if-zero (eq! STACK_PATTERN PATTERN_CALL)
           (if-zero COUNTER
                    (begin
                     ;; stack items - first line
                     (make-stack-item 1 (- HEIGHT 2 FLAG_1) 1 (+ STACK_STAMP 1))
                     (make-stack-item 2 (- HEIGHT 4 FLAG_1) 1 (+ STACK_STAMP 2))
                     (make-stack-item 3 (- HEIGHT 3 FLAG_1) 1 (+ STACK_STAMP 3))
                     (make-stack-item 4 (- HEIGHT 5 FLAG_1) 1 (+ STACK_STAMP 4))
                     ;; stamp update
                     (eq! STACK_STAMP_NEW (+ STACK_STAMP 7 FLAG_1))
                     ;; height update
                     (eq! HEIGHT_NEW (- HEIGHT 5 FLAG_1)))
                    (begin
                     ;; stack items - second line
                     (make-stack-item 1 HEIGHT                  1      (+ STACK_STAMP 5))
                     (make-stack-item 2 (- HEIGHT 1)            1      (+ STACK_STAMP 6))
                     (make-stack-item 3 (* (- HEIGHT 2) FLAG_1) FLAG_1 (* FLAG_1 (+ STACK_STAMP 7)))
                     (make-stack-item 4 (- HEIGHT 5 FLAG_1)     0      (+ STACK_STAMP 7 FLAG_1))))))

(defconstraint pattern-create (:guard (standard-regime))
  (if-zero (eq! STACK_PATTERN PATTERN_CREATE)
           (if-zero COUNTER
                    (begin
                     ;; stack items - first line
                     (make-stack-item 1 (- HEIGHT 1)            1      (+ STACK_STAMP 1))
                     (make-stack-item 2 (* FLAG_1 (- HEIGHT 3)) FLAG_1 (* FLAG_1 (+ STACK_STAMP 2)))
                     (make-stack-item 3 (- HEIGHT 2)            1      (+ STACK_STAMP 2 FLAG_1))
                     (make-stack-item 4 (- HEIGHT 2 FLAG_1)     0      (+ STACK_STAMP 3 FLAG_1))
                     ;; stamp update
                     (eq! STACK_STAMP_NEW (+ STACK_STAMP 4 FLAG_1))
                     ;; height update
                     (eq! HEIGHT_NEW (- HEIGHT 2 FLAG_1)))
                    (begin
                     ;; stack items - second line
                     (make-empty-stack-item 1)
                     (make-empty-stack-item 2)
                     (make-stack-item       3 HEIGHT 1 (+ STACK_STAMP 4 FLAG_1))
                     (make-empty-stack-item 4)))))

(defun (pattern-exception)
    (if-zero (eq! STACK_PATTERN PATTERN_CREATE)
             (if-zero COUNTER
                      (begin 0)))) ;; TODO

(defconstraint consistency ()
  (if-not-zero (next SRT_CN_POW_4)
               (if-not-zero (next SRT_HEIGHT_1234)
                            (begin
                             ;; context and height remain unchanged
                             (if-zero (will-remain-constant! SRT_CN_POW_4)
                                      (if-zero (will-remain-constant! SRT_HEIGHT_1234)
                                               (begin (if-not-zero (next SRT_POP_1234)
                                                                   (begin
                                                                    (will-remain-constant! SRT_VAL_HI_1234)
                                                                    (will-remain-constant! SRT_VAL_LO_1234)))
                                                      ;; context changes
                                                      (if-not-zero (will-remain-constant! SRT_CN_POW_4)
                                                                   (vanishes! (next SRT_POP_1234)))
                                                      ;; height changes
                                                      (if-not-zero (will-remain-constant! SRT_HEIGHT_1234)
                                                                   (vanishes! (next SRT_POP_1234)))
                                                      (eq! (+ (next SRT_POP_1234) SRT_POP_1234) 1))))))))



;; TODO @Olivier, cf issue #235
;; (defconstraint pc-constraints ()
;;   (
;;      // unusual program counter update
;;      upcu := CE[UNUSUAL_PC_UPDATE.Name()].Equals(
;;          CE[JUMP_FLAG.Name()].Add(CE[PUSH_FLAG.Name()]),
;;      )

;;      // should be BinIfZeroElse ...
;;      sortedPcUpdate := CE[SORTED_CONTEXT_NUMBER.Name()].RemainsConstant().IfZeroElse(
;;          // SORTED_CONTEXT_NUMBER[i+1] == SORTED_CONTEXT_NUMBER[i]
;;          CE[SORTED_UNUSUAL_PC_UPDATE.Name()].BinIfZero(
;;              CE[SORTED_PC.Name()].Inc(1),
;;          ),
;;          // SORTED_CONTEXT_NUMBER[i+1] != SORTED_CONTEXT_NUMBER[i]
;;          CE[SORTED_PC.Name()].Shift(1),
;;      )

;;      pc = append(pc, upcu)
;;      pc = append(pc, sortedPcUpdate)

;;      pc = CE[SORTED_CONTEXT_NUMBER.Name()].BranchIfNotZero(pc...)
;;    ))
;;
;; (defconstraint stamp-update ()
;;   (if-not-zero CONTEXT_NUMBER
;;                (if-not-zero (will-remain-constant! INSTRUCTION_STAMP)
;;                             (ex STACK_STAMP_NEW (next STACK_STAMP)))))
