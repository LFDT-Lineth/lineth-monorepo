(module mxp)


(defconstraint  computations---state-update---word-pricing---computing-the-number-of-input-words
		(:guard   (mxp-guard---state-update-word-pricing))
		(euc-call    ROW_OFFSET___CEILING_OF_SIZE_OVER_32
			     (mxp-shorthand---size-1-lo)
			     WORD_SIZE))

(defun  (mxp-shorthand---number-of-words)   (shift  computation/RES_B  ROW_OFFSET___CEILING_OF_SIZE_OVER_32))
(defun  (mxp-shorthand---word-price)        (shift  decoder/G_WORD     NEGATIVE_ROW_OFFSET___SCNRI_TO_DECDR))
(defun  (mxp-shorthand---extra-word-cost)  (*  (mxp-shorthand---word-price)
					       (mxp-shorthand---number-of-words)))

(defconstraint  computations---state-update---word-pricing---justifying-the-memory-expansion-gas
		(:guard   (mxp-guard---state-update-word-pricing))
		(eq!      (shift  macro/GAS_MXP  NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO)
			  (+  (-  scenario/C_MEM_NEW  scenario/C_MEM)
			      (mxp-shorthand---extra-word-cost))))
