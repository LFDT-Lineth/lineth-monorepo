(module rom)

;; Constancies
(defun (cfi-constant X)
  (if-not-eq CFI
             (+ (prev CFI) 1)
             (remained-constant! X)))

(defun (cfi-incrementing X)
  (if-not-eq CFI
             (+ (prev CFI) 1)
             (or! (remained-constant! X) (did-inc! X 1))))

(defpurefun (counter-constant X ct ctmax)
  (if-not-eq ct ctmax (will-remain-constant! X)))

(defun (push-constant X)
  (if-not-zero COUNTER_PUSH
               (remained-constant! X)))

(defun (push-incrementing X)
  (if-not-zero COUNTER_PUSH
               (or! (remained-constant! X) (did-inc! X 1))))

(defconstraint cfi-constancies ()
  (cfi-constant CODE_SIZE))

(defconstraint cfi-incrementings ()
  (begin (cfi-incrementing CODESIZE_REACHED)
         (debug (cfi-incrementing PC))))

(defconstraint ct-constancies ()
  (begin (counter-constant LIMB CT COUNTER_MAX)
         (counter-constant nBYTES CT COUNTER_MAX)
         (counter-constant COUNTER_MAX CT COUNTER_MAX)))

(defconstraint push-constancies ()
  (begin (push-constant PUSH_PARAMETER)
         (push-constant PUSH_VALUE_HI)
         (push-constant PUSH_VALUE_LO)))

;; Heartbeat
(defconstraint initialization (:domain {0})
  (vanishes! CODE_FRAGMENT_INDEX))

(defconstraint cfi-evolving-possibility ()
  (or! (will-remain-constant! CFI) (will-inc! CFI 1)))

(defconstraint no-cfi-nothing ()
  (if-zero CFI
           (begin (vanishes! CT)
                  (vanishes! COUNTER_MAX)
                  (vanishes! PBCB)
                  (debug (vanishes! IS_PUSH))
                  (debug (vanishes! IS_PUSH_DATA))
                  (debug (vanishes! COUNTER_PUSH))
                  (debug (vanishes! PUSH_PARAMETER))
                  (debug (vanishes! PROGRAM_COUNTER)))
           (begin (debug (or! (eq! COUNTER_MAX LLARGEMO) (eq! COUNTER_MAX WORD_SIZE_MO)))
                  (if-eq COUNTER_MAX LLARGEMO (will-remain-constant! CFI))
                  (if-not-eq COUNTER COUNTER_MAX (will-remain-constant! CFI))
                  (if-eq CT WORD_SIZE_MO (will-inc! CFI 1)))))

(defconstraint counter-evolution ()
  (if-eq-else CT COUNTER_MAX
              (vanishes! (next CT))
              (will-inc! CT 1)))

(defconstraint finalisation (:domain {-1})
  (if-not-zero CFI
               (begin (eq! CT COUNTER_MAX)
                      (eq! COUNTER_MAX WORD_SIZE_MO)
                      (eq! CFI CODE_FRAGMENT_INDEX_INFTY))))

(defconstraint cfi-infty ()
  (if-zero CFI
           (vanishes! CODE_FRAGMENT_INDEX_INFTY)
           (will-remain-constant! CODE_FRAGMENT_INDEX_INFTY)))

(defconstraint limb-accumulator ()
  (begin (if-zero CT
                  (eq! ACC PBCB)
                  (eq! ACC
                       (+ (* 256 (prev ACC))
                          PBCB)))
         (if-eq CT COUNTER_MAX (eq! ACC LIMB))))

;; CODESIZE_REACHED Constraints
(defconstraint codesizereached-trigger ()
  (if-eq PC (- CODE_SIZE 1)
         (eq! (+ CODESIZE_REACHED (next CODESIZE_REACHED))
              1)))

(defconstraint csr-impose-ctmax (:guard CFI)
  (if-zero CT
           (if-zero CODESIZE_REACHED
                    (eq! COUNTER_MAX LLARGEMO)
                    (eq! COUNTER_MAX WORD_SIZE_MO))))

;; nBytes constraints
(defconstraint nbytes-acc (:guard CFI)
  (if-zero CT
           (if-zero CODESIZE_REACHED
                    (eq! nBYTES_ACC 1)
                    (vanishes! nBYTES))
           (if-zero CODESIZE_REACHED
                    (did-inc! nBYTES_ACC 1)
                    (remained-constant! nBYTES_ACC))))

(defconstraint nbytes-collusion ()
  (if-eq CT COUNTER_MAX (eq! nBYTES nBYTES_ACC)))

;; INDEX constraints
(defconstraint no-cfi-no-index ()
  (if-zero CFI
           (vanishes! INDEX)))

(defconstraint new-cfi-reboot-index ()
  (if-not-zero (- CFI (prev CFI))
               (vanishes! INDEX)))

(defconstraint new-ct-increment-index ()
  (if-not-zero (or! (eq! CFI 0)
                    (did-inc! CFI 1)
                    (neq! CT 0))
               (did-inc! INDEX 1)))

(defconstraint index-inc-in-middle-padding ()
  (if-eq CT LLARGE (did-inc! INDEX 1)))

(defconstraint index-quasi-ct-cst ()
  (if-not-zero (* CT (- CT LLARGE))
               (remained-constant! INDEX)))

;; PC constraints
(defconstraint pc-incrementing (:guard CFI)
  (if-not-eq (next CFI) (+ CFI 1) (will-inc! PC 1)))

(defconstraint pc-reboot ()
  (if-not-eq (next CFI)
             CFI
             (vanishes! (next PC))))

;; end of CFI (padding rows)
(defconstraint end-code-no-opcode ()
  (if-eq CODESIZE_REACHED 1 (vanishes! PBCB)))

;; Constraints Related to PUSHX instructions
(defconstraint not-a-push-data ()
  (if-zero IS_PUSH_DATA
           (begin (vanishes! COUNTER_PUSH)
                  (eq! OPCODE PBCB))))

(defconstraint ispush-ispushdata-exclusivity ()
  (or! (eq! IS_PUSH 0) (eq! IS_PUSH_DATA 0)))

(defconstraint ispush-implies-next-pushdata ()
  (if-not-zero IS_PUSH (eq! (next IS_PUSH_DATA) 1)))

(defconstraint ispush-constraint ()
  (if-not-zero IS_PUSH
               (begin (eq! PUSH_PARAMETER
                           (- OPCODE (- EVM_INST_PUSH1 1)))
                      (vanishes! PUSH_VALUE_ACC)
                      (vanishes! (+ PUSH_FUNNEL_BIT (next PUSH_FUNNEL_BIT))))))

(defconstraint ispushdata-constraint ()
  (if-not-zero IS_PUSH_DATA
               (begin (eq! (+ (prev IS_PUSH) (prev IS_PUSH_DATA))
                           1)
                      (eq! OPCODE EVM_INST_INVALID)
                      (did-inc! COUNTER_PUSH 1)
                      (if-zero (- (+ COUNTER_PUSH LLARGE) PUSH_PARAMETER)
                               (begin (will-inc! PUSH_FUNNEL_BIT 1)
                                      (eq! PUSH_VALUE_HI PUSH_VALUE_ACC))
                               (if-eq (next IS_PUSH_DATA) 1 (will-remain-constant! PUSH_FUNNEL_BIT)))
                      (if-zero (- (prev PUSH_FUNNEL_BIT) PUSH_FUNNEL_BIT)
                               (eq! PUSH_VALUE_ACC
                                    (+ (* 256 (prev PUSH_VALUE_ACC))
                                       PBCB))
                               (eq! PUSH_VALUE_ACC PBCB))
                      (if-eq COUNTER_PUSH PUSH_PARAMETER
                             (begin (if-zero PUSH_FUNNEL_BIT
                                             (vanishes! PUSH_VALUE_HI))
                                    (eq! PUSH_VALUE_ACC PUSH_VALUE_LO)
                                    (vanishes! (next IS_PUSH_DATA)))))))


