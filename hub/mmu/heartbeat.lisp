(module mmu)

;; 1.
(defconstraint ram-stamp-non-decreasing ()
  (if-not-zero (will-remain-constant! RAM_STAMP)
               (will-inc! RAM_STAMP 1)))

;; 2.
(defconstraint ramp-stamp-starts-at-0 (:domain {0}) RAM_STAMP)

;; 3.
(defconstraint zero-rows ()
  (if-zero RAM_STAMP
           (begin
            (vanishes! RAM_STAMP)
            (vanishes! MICRO_STAMP)
            (vanishes! MICRO_INSTRUCTION)
            (vanishes! TOT)
            (vanishes! TOTRD)
            (vanishes! IS_MICRO)
            (vanishes! OFF_1_LO)
            (vanishes! OFF_2_HI)
            (vanishes! OFF_2_LO)
            (vanishes! SIZE_IMPORTED)
            (vanishes! SIZE)
            (vanishes! VAL_HI)
            (vanishes! VAL_LO)
            (vanishes! CONTEXT_NUMBER)
            (vanishes! CALLER)
            (vanishes! RETURNER)
            (vanishes! CONTEXT_SOURCE)
            (vanishes! CONTEXT_TARGET)
            (vanishes! CT)
            (vanishes! OFFOOB)
            (vanishes! PRE)
            (vanishes! SLO)
            (vanishes! SBO)
            (vanishes! TLO)
            (vanishes! TBO)
            (vanishes! ALIGNED)
            (vanishes! MIN)
            (vanishes! TERNARY)
            (vanishes! RETURN_OFFSET)
            (vanishes! RETURN_CAPACITY)
            (vanishes! EXO_IS_HASH)
            (vanishes! EXO_IS_LOG)
            (vanishes! EXO_IS_ROM)
            (vanishes! REFS)
            (vanishes! INFO)

            (vanishes! NIB_1)
            (vanishes! NIB_2)
            (vanishes! NIB_3)
            (vanishes! NIB_4)
            (vanishes! NIB_5)
            (vanishes! NIB_6)
            (vanishes! NIB_7)
            (vanishes! NIB_8)
            (vanishes! NIB_9)

            (vanishes! BIT_1)
            (vanishes! BIT_2)
            (vanishes! BIT_3)
            (vanishes! BIT_4)
            (vanishes! BIT_5)
            (vanishes! BIT_6)
            (vanishes! BIT_7)
            (vanishes! BIT_8)

            (vanishes! ACC_1)
            (vanishes! ACC_2)
            (vanishes! ACC_3)
            (vanishes! ACC_4)
            (vanishes! ACC_5)
            (vanishes! ACC_6)
            (vanishes! ACC_7)
            (vanishes! ACC_8)

            (vanishes! BYTE_1)
            (vanishes! BYTE_2)
            (vanishes! BYTE_3)
            (vanishes! BYTE_4)
            (vanishes! BYTE_5)
            (vanishes! BYTE_6)
            (vanishes! BYTE_7)
            (vanishes! BYTE_8))))

;; 4.
(defconstraint ram-stamp-changes ()
  (if-not-zero (will-remain-constant! RAM_STAMP)
               (begin (vanishes! (shift IS_MICRO 1))
                      (vanishes! (shift CT 1))
                      (is-not-zero (shift TOT 1)))))

;; 5.
(defconstraint ram-stamp-not-zero (:guard RAM_STAMP)
  (if-zero IS_MICRO
           ;; OFFOOB == 0
           (if-zero OFFOOB
                    (if-eq-else CT (- SSMALL 1)
                                ;; CT == 2
                                (will-eq! IS_MICRO 1)
                                ;; CT != 2
                                (begin (will-inc! CT 1)
                                       (shift IS_MICRO 1)))
                    ;; OFFOOB == 1
                    (if-eq-else CT (- LLARGE 1)
                                ;; CT == 15
                                (will-eq! IS_MICRO 1)
                                ;; CT != 15
                                (begin (will-inc! CT 1)
                                       (shift IS_MICRO 1))))))

;; 6.
(defconstraint counter-eq-0-in-micro-writing ()
  (if-eq IS_MICRO 1 CT))

;; 7.
(defconstraint ram-stamp-remains-const ()
  (if-zero (will-remain-constant! RAM_STAMP)
           (eq (shift TOT 1) (- TOT (shift IS_MICRO 1)))))

;; 8.
(defconstraint micro-writing-and-tot-non-zero  ()
  (if-eq IS_MICRO 1
         (if-not-zero TOT (will-eq! IS_MICRO 1))))

;; 9.
(defconstraint ram-stamp-non-zero-and-tot-zero  ()
  (if-not-zero RAM_STAMP
               (if-zero TOT
                        (will-inc! RAM_STAMP 1))))

;; 10.
(defconstraint micro-stamp ()
  (eq (shift MICRO_STAMP 1)
      (+ MICRO_STAMP (shift IS_MICRO 1))))


;; _IS_DATA
(defconstraint is_data ()
  (if-zero RAM_STAMP
           (vanishes! IS_DATA)
           (eq IS_DATA 1)))
