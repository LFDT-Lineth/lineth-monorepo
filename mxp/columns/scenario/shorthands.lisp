(module mxp)


(defun  (mxp-scenario-shorthand---scenario-sum)
  (+    (mxp-scenario-shorthand---no-state-update)
	(mxp-scenario-shorthand---state-update)
	))

(defun  (mxp-scenario-shorthand---no-state-update)
  (+    scenario/MSIZE
	scenario/TRIVIAL
	scenario/MXPX
	;; scenario/STATE_UPDATE_WORD_PRICING
	;; scenario/STATE_UPDATE_BYTE_PRICING
	))

(defun  (mxp-scenario-shorthand---state-update)
  (+    ;; scenario/MSIZE
        ;; scenario/TRIVIAL
        ;; scenario/MXPX
        scenario/STATE_UPDATE_WORD_PRICING
        scenario/STATE_UPDATE_BYTE_PRICING
        ))

(defun  (mxp-scenario-shorthand---not-msize-nor-trivial)
  (+    (mxp-scenario-shorthand---state-update)
	scenario/MXPX
	))

(defun  (mxp-scenario-shorthand---not-msize)
  (+    (mxp-scenario-shorthand---not-msize-nor-trivial)
	scenario/TRIVIAL
	))
