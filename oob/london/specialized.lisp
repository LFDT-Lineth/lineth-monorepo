(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.6 Constraint systems   ;;
;;    for populating lookups   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; support function to improve to reduce code duplication in the functions below
(defun (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (shift [OUTGOING_DATA 1] k) arg_1_hi)
         (eq! (shift [OUTGOING_DATA 2] k) arg_1_lo)
         (eq! (shift [OUTGOING_DATA 3] k) arg_2_hi)
         (eq! (shift [OUTGOING_DATA 4] k) arg_2_lo)))

(defun (call-to-ADD k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 1)
         (eq! (shift OUTGOING_INST k) EVM_INST_ADD)
         (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)))

(defun (call-to-DIV k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 2)
         (eq! (shift OUTGOING_INST k) EVM_INST_DIV)
         (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)))

(defun (call-to-MOD k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 2)
         (eq! (shift OUTGOING_INST k) EVM_INST_MOD)
         (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)))

(defun (call-to-LT k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 3)
         (eq! (shift OUTGOING_INST k) EVM_INST_LT)
         (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)))

(defun (call-to-GT k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 3)
         (eq! (shift OUTGOING_INST k) EVM_INST_GT)
         (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)))

(defun (call-to-EQ k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 3)
         (eq! (shift OUTGOING_INST k) EVM_INST_EQ)
         (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)))

(defun (call-to-ISZERO k arg_1_hi arg_1_lo)
  (begin (eq! (wght-lookup-sum k) 3)
         (eq! (shift OUTGOING_INST k) EVM_INST_ISZERO)
         (eq! (shift [OUTGOING_DATA 1] k) arg_1_hi)
         (eq! (shift [OUTGOING_DATA 2] k) arg_1_lo)
         (debug (vanishes! (shift [OUTGOING_DATA 3] k)))
         (debug (vanishes! (shift [OUTGOING_DATA 4] k)))))

(defun (noCall k)
  (begin (eq! (wght-lookup-sum k) 0)))
