(module mxp)

(defconstraint  computations---state-update---justifying-state-update-scenario-WORD-vs-BYTE
		(:guard   (mxp-guard---state-update))
		(begin
		  (eq!   scenario/STATE_UPDATE_WORD_PRICING   (mxp-shorthand---word-pricing-instruction))
		  (eq!   scenario/STATE_UPDATE_BYTE_PRICING   (mxp-shorthand---byte-pricing-instruction))
		  ))

(defconstraint  computations---state-update---comparing-max-offsets
		(:guard   (mxp-guard---state-update))
		(wcp-call-to-LT  ROW_OFFSET___COMPARISON_OF_MAX_OFFSETS
				 0
				 (*  (mxp-shorthand---double-offset-instruction)  (mxp-shorthand---max-1))
				 0
				 (*  (mxp-shorthand---double-offset-instruction)  (mxp-shorthand---max-2))
				 ))

(defun  ((mxp-shorthand---use-parameter-set-2 :binary :force))  (shift  computation/RES_A  ROW_OFFSET___COMPARISON_OF_MAX_OFFSETS))
(defun  ((mxp-shorthand---use-parameter-set-1 :binary))  (-   1    (mxp-shorthand---use-parameter-set-2)))

(defun  (mxp-shorthand---max-1)          (* (mxp-shorthand---size-1-is-nonzero) (+ (mxp-shorthand---offset-1-lo)   (mxp-shorthand---size-1-lo))))
(defun  (mxp-shorthand---max-2)          (* (mxp-shorthand---size-2-is-nonzero) (+ (mxp-shorthand---offset-2-lo)   (mxp-shorthand---size-2-lo))))
(defun  (mxp-shorthand---max-offset-1)   (- (mxp-shorthand---max-1)         1))
(defun  (mxp-shorthand---max-offset-2)   (- (mxp-shorthand---max-2)         1))
(defun  (mxp-shorthand---max-offset)     (+   (*   (mxp-shorthand---use-parameter-set-1)   (mxp-shorthand---max-offset-1))
					      (*   (mxp-shorthand---use-parameter-set-2)   (mxp-shorthand---max-offset-2))))

(defconstraint  computations---state-update---computing-EYP_A-as-division-of-max-offset-by-32
		(:guard   (mxp-guard---state-update))
		(euc-call    ROW_OFFSET___FLOOR_OF_MAX_OFFSET_OVER_32
			     (mxp-shorthand---max-offset)
			     WORD_SIZE))

(defun  (mxp-shorthand---floor)   (shift   computation/RES_A   ROW_OFFSET___FLOOR_OF_MAX_OFFSET_OVER_32))
(defun  (mxp-shorthand---EYP_a)   (+   (mxp-shorthand---floor)   1))

(defconstraint  computations---state-update---computing-quadratic-part-of-C_MEM-as-division-of-EYP_A-squared-over-512
		(:guard   (mxp-guard---state-update))
		(euc-call    ROW_OFFSET___FLOOR_OF_SQUARE_OVER_512
			     (*   (mxp-shorthand---EYP_a)   (mxp-shorthand---EYP_a))
			     512))

(defun  (mxp-shorthand---C_MEM-quad-part)   (shift   computation/RES_A   ROW_OFFSET___FLOOR_OF_SQUARE_OVER_512))
(defun  (mxp-shorthand---C_MEM-linr-part)   (*   GAS_CONST_G_MEMORY   (mxp-shorthand---EYP_a)))

(defconstraint  computations---state-update---comparing-EYP_A-to-current-WORDS
		(:guard   (mxp-guard---state-update))
		(wcp-call-to-LT  ROW_OFFSET___COMPARISON_OF_WORDS_AND_EYP_A
				 0
				 (mxp-shorthand---words)
				 0
				 (mxp-shorthand---EYP_a)))

(defun  (mxp-shorthand---words)       scenario/WORDS     ) ;; ""
(defun  (mxp-shorthand---words-new)   scenario/WORDS_NEW ) ;; ""
(defun  (mxp-shorthand---c_mem)       scenario/C_MEM     ) ;; ""
(defun  (mxp-shorthand---c_mem-new)   scenario/C_MEM_NEW ) ;; ""
(defun  (mxp-shorthand---update-internal-state)   (shift   computation/RES_A   ROW_OFFSET___COMPARISON_OF_WORDS_AND_EYP_A))

(defconstraint  computations---state-update---updating-the-state---no-state-update
		(:guard   (mxp-guard---state-update))
		(if-zero   (force-bin   (mxp-shorthand---update-internal-state))
			   (begin
			        (eq!   (mxp-shorthand---words-new)   (mxp-shorthand---words))
			        (eq!   (mxp-shorthand---c_mem-new)   (mxp-shorthand---c_mem))
			   )
	    ))

(defconstraint  computations---state-update---updating-the-state---state-update
		(:guard   (mxp-guard---state-update))
		(if-not-zero   (force-bin   (mxp-shorthand---update-internal-state))
			       (begin
			            (eq!   (mxp-shorthand---words-new)   (mxp-shorthand---EYP_a))
			            (eq!   (mxp-shorthand---c_mem-new)   (+  (mxp-shorthand---C_MEM-quad-part)
									(mxp-shorthand---C_MEM-linr-part)))
				   )
	    ))
