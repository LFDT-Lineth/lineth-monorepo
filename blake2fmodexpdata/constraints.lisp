(module blake2fmodexpdata)

(defun (flag-sum)
  (+ IS_MODEXP_BASE
     IS_MODEXP_EXPONENT
     IS_MODEXP_MODULUS
     IS_MODEXP_RESULT
     IS_BLAKE_DATA
     IS_BLAKE_PARAMS
     IS_BLAKE_RESULT))

(defun (phase-sum)
  (+
    (* PHASE_MODEXP_BASE     IS_MODEXP_BASE)
    (* PHASE_MODEXP_EXPONENT IS_MODEXP_EXPONENT)
    (* PHASE_MODEXP_MODULUS  IS_MODEXP_MODULUS)
    (* PHASE_MODEXP_RESULT   IS_MODEXP_RESULT)
    (* PHASE_BLAKE_DATA      IS_BLAKE_DATA)
    (* PHASE_BLAKE_PARAMS    IS_BLAKE_PARAMS)
    (* PHASE_BLAKE_RESULT    IS_BLAKE_RESULT)))

(defun (index-max-sum)
  (+ (* INDEX_MAX_MODEXP_BASE IS_MODEXP_BASE)
     (* INDEX_MAX_MODEXP_EXPONENT IS_MODEXP_EXPONENT)
     (* INDEX_MAX_MODEXP_MODULUS IS_MODEXP_MODULUS)
     (* INDEX_MAX_MODEXP_RESULT IS_MODEXP_RESULT)
     (* INDEX_MAX_BLAKE_DATA IS_BLAKE_DATA)
     (* INDEX_MAX_BLAKE_PARAMS IS_BLAKE_PARAMS)
     (* INDEX_MAX_BLAKE_RESULT IS_BLAKE_RESULT)))

(defconstraint no-stamp-no-flag ()
  (if-zero STAMP
           (vanishes! (flag-sum))
           (eq! (flag-sum) 1)))

(defconstraint set-phase-and-index ()
  (begin (eq! PHASE (phase-sum))
         (eq! INDEX_MAX (index-max-sum))))


