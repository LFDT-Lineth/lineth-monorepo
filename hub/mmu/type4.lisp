(module mmu)

(defun (codecopy-type4)
  (begin
       (vanishes! CN_S)
       (vanishes! EXO_IS_HASH)
       (vanishes! EXO_IS_LOG)
       (vanishes! EXO_IS_TXCD)
       (if-zero-else IS_MICRO
         ;IS_MICRO = 0
         (vanishes! EXO_IS_ROM)
         ;IS_MICRO = 1
         (if-zero-else (shift TOTRD -1)
           ;TOTRD_{i-1} = 0
           (vanishes! EXO_IS_ROM)
           ;TOTRD_{i-1} != 0
           (eq EXO_IS_ROM 1)
         )
       )
     )
)

;4.6.2
(defun (context-type4)
 (begin
   ;1
   (eq CN_T CN)
   ;2.a
   (if-eq INSTRUCTION RETURNDATACOPY
     (begin
       (eq CN_S RETURNER)
       (vanishes! EXO_IS_HASH)
       (vanishes! EXO_IS_LOG)
       (vanishes! EXO_IS_ROM)
       (vanishes! EXO_IS_TXCD)
     )
   )
   ;2.b
   (if-eq INSTRUCTION CALLDATACOPY
     (begin
       (eq CN_S CALLER)
       (vanishes! EXO_IS_HASH)
       (vanishes! EXO_IS_LOG)
       (vanishes! EXO_IS_ROM)
       (if-zero-else IS_MICRO
         ;IS_MICRO = 0
         (vanishes! EXO_IS_TXCD)
         ;IS_MICRO = 1
         (if-zero-else (shift TOTRD -1)
           ;TOTRD_{i-1} = 0
           (vanishes! EXO_IS_TXCD)
           ;TOTRD_{i-1} != 0
           (eq EXO_IS_TXCD INFO)
         )
       )
     )
   )
   ;2c
   (if-eq INSTRUCTION CODECOPY
     (codecopy-type4)
   )
   (if-eq INSTRUCTION EXTCODECOPY
     (codecopy-type4)
   )
 )
)


;4.6.3
(defun (establishing-type4-tern)
  (if-zero-else OFF_2_HI
    ;2
    (begin
      ;2.a
      (if-eq TERNARY tern0
         (begin
           (eq OFFOOB 0)
           (eq ACC_1 (- REFS (+ OFF_2_LO SIZE_IMPORTED)))
         )
      )
      ;2.b
      (if-eq TERNARY tern1
         (begin
           (eq OFFOOB 0)
           (eq ACC_1 (- (+ OFF_2_LO (- SIZE_IMPORTED 1)) REFS))
           (eq ACC_2 (- REFS OFF_2_LO 1))

         )
      )
      ;2.c
      (if-eq TERNARY tern2
         (begin
            (eq OFFOOB 1)
            (eq ACC_1 (- OFF_2_LO REFS))
         )
      )
    )
    ;1
    (begin
      (eq TERNARY tern2)
      (eq OFFOOB 1)
    )
  )
)


(defun (preprocessing-type4)
  (if-zero IS_MICRO
    (if-eq (shift IS_MICRO 1) 1
      (begin
        (establishing-type4-tern)
        (preprocessing-type4-tern1)
        (preprocessing-type4-tern2)
      )
    )
  )
)


(defun (micro-instruction-writing-type4)
  (begin
    (micro-instruction-writing-type4-tern1)
    (micro-instruction-writing-type4-tern2)
  )
)

; Global constraint enforcing that TERNARY takes values 0,1,2
(defconstraint tern-is-ternary ()
                (vanishes!
                    (*	(- TERNARY tern0)
                        (- TERNARY tern1)
                        (- TERNARY tern2))))

;====== type4 ======
(defconstraint type4 ()
  (if-zero (* (- type4CC PRE) (- type4CD PRE) (- type4RD PRE) )
    (begin
      (context-type4)
      (preprocessing-type4)
      (micro-instruction-writing-type4)
    )
  )
)
