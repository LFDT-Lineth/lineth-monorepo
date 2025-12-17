(module mxp)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;   Heartbeat constraints   ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint  generalities---heartbeat---perspective-sum-is-binary   ()
		(is-binary   (mxp-perspective-sum)))

(defconstraint  generalities---heartbeat---perspective-sum-initially-vanishes   (:domain {0}) ;; ""
		(vanishes!   (mxp-perspective-sum)))

(defconstraint  generalities---heartbeat---perspective-sum-next-value   ()
		(if-zero   (force-bin (mxp-perspective-sum))
			   ;; mxp-perspective-sum ≡ 0
			   (eq!    (next    (mxp-perspective-sum))
				   (next    DECODER))
			   ;; mxp-perspective-sum ≡ 1
			   (eq!    (next    (mxp-perspective-sum))
				   1)))

(defconstraint  generalities---heartbeat---MXP-stamp-initially-vanishes   (:domain {0}) ;; ""
		(vanishes!    MXP_STAMP))

(defun   (mxp-stamp-increment)   (*  (-  1  DECODER)   (next   DECODER)))

(defconstraint  generalities---heartbeat---MXP-stamp-increments   ()
		(will-inc!   MXP_STAMP   (mxp-stamp-increment)))

(defconstraint  generalities---heartbeat---automatic-vanishing-during-padding ()
		(if-zero   (force-bin (mxp-perspective-sum))
			   (begin
			     (vanishes!   CT_MAX)
			     (vanishes!   CT)
			     (vanishes!   (next  CT))
			     )))

(defconstraint  generalities---heartbeat---CT_MAX-vanishes-in-all-phases-except-COMPUTATION ()
		(if-not-zero  (+  DECDR  MACRO  SCNRI)
			      (vanishes!   CT_MAX)))

(defconstraint  generalities---heartbeat---setting-CT_MAX-during-COMPUTATION ()
		(if-not-zero  SCNRI
			      (eq!  (next  CT_MAX)
				    (mxp-ct-max-sum))
			      ))

(defconstraint  generalities---heartbeat---illegal-perspective-transitions ()
		(vanishes!   (+  (*  DECDR  (next  (+  DECDR       SCNRI CMPTN)))
				 (*  MACRO  (next  (+  DECDR MACRO       CMPTN)))
				 (*  SCNRI  (next  (+  DECDR MACRO SCNRI      )))
				 (*  CMPTN  (next  (+        MACRO SCNRI      )))
				 )))

(defconstraint  generalities---heartbeat---legal-perspective-transitions (:guard MXP_STAMP)
		(if-not-zero  (-  CT_MAX  CT)
			      ;; CT ≠ CT_MAX case
			      (eq!  (+  (*  DECDR  (next  DECDR))
					(*  MACRO  (next  MACRO))
					(*  SCNRI  (next  SCNRI))
					(*  CMPTN  (next  CMPTN)))
				    1)
			      ;; CT = CT_MAX case
			      (eq!  (+  (*  DECDR  (next  MACRO))
					(*  MACRO  (next  SCNRI))
					(*  SCNRI  (next  CMPTN))
					(*  CMPTN  (next  DECDR)))
				    1)
			      ))

(defconstraint  generalities---heartbeat---CT-updates ()
		(if-not-zero  (-  CT_MAX  CT)
			      ;; CT ≠ CT_MAX case
			      (will-inc!   CT  1)
			      ;; CT = CT_MAX case
			      (vanishes!   (next   CT))
			      ))

(defconstraint  generalities---heartbeat---finalization-constraints   (:domain {-1}) ;; ""
		(if-not-zero   MXP_STAMP
			       (begin
				 ( eq!   CMPTN   1      )
				 ( eq!   CT      CT_MAX )
				 )))
