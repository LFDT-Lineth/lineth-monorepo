(module mxp)

(defconstraint  computations---generalities---precisely-one-scenario-is-active
		(:guard    SCENARIO)
		(eq!   (mxp-scenario-shorthand---scenario-sum)
		       1))

(defconstraint  computations---generalities---setting-MSIZE-scenario-flag
		(:guard    SCENARIO)
		(eq!   scenario/MSIZE
		       (shift   decoder/IS_MSIZE    NEGATIVE_ROW_OFFSET___SCNRI_TO_DECDR)))


(defconstraint  computations---generalities---setting-MXPX-scenario-flag
		(:guard    SCENARIO)
		(eq!   scenario/MXPX
		       (shift   macro/MXPX    NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO)))

(defconstraint  computations---generalities---no-state-update-scenario-enforces-no-state-update
		(:guard    SCENARIO)
		(if-not-zero    (mxp-scenario-shorthand---no-state-update)
				(begin
				  (eq!   scenario/WORDS_NEW   scenario/WORDS)
				  (eq!   scenario/C_MEM_NEW   scenario/C_MEM)
				  )))

(defconstraint  computations---generalities---no-state-update-scenario-enforces-zero-GAS_MXP
		(:guard    SCENARIO)
		(if-not-zero    (mxp-scenario-shorthand---no-state-update)
				(vanishes!   (shift   macro/GAS_MXP   NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO))
				))
