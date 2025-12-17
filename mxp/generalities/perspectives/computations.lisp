(module mxp)

(defconstraint  generalities---perspectives---computation---lookup-flag-exclusivity   (:guard COMPUTATION)
		(vanishes!   (*   computation/WCP_FLAG   computation/EUC_FLAG)))
