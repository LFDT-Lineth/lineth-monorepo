(module blsdata)

(defconstraint stamp-constancy ()
  (begin (stamp-constancy STAMP ID)
         (stamp-constancy STAMP SUCCESS_BIT)
         (stamp-constancy STAMP MINT)
         (stamp-constancy STAMP MEXT)
         (stamp-constancy STAMP WTRV)
         (stamp-constancy STAMP WNON)))

(defconstraint counter-constancy ()
  (begin (counter-constancy INDEX PHASE) ;; NOTE: PHASE and INDEX_MAX are said to be index-constant
         (counter-constancy INDEX INDEX_MAX)
         (counter-constancy CT CT_MAX)
         (counter-constancy CT IS_INFINITY)
         (counter-constancy CT ACC_INPUTS)
         (counter-constancy CT NONTRIVIAL_POP_ACC)
         (counter-constancy CT MEXT_BIT)
         (counter-constancy CT MEXT_ACC)))

(defconstraint pair-of-inputs-constancy ()
  (if-not-zero ACC_INPUTS
               (if-zero (- (next ACC_INPUTS) ACC_INPUTS)
                   (will-remain-constant! NONTRIVIAL_POP_BIT))))