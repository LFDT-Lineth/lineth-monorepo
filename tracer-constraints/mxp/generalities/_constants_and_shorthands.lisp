(module mxp)

;;;;;;;;;;;;;;;;;;;;;
;;                 ;;
;;   Row offsets   ;;
;;                 ;;
;;;;;;;;;;;;;;;;;;;;;

(defconst
  ROW_OFFSET___DECDR_TO_MACRO   1
  ROW_OFFSET___DECDR_TO_SCNRI   2
  ROW_OFFSET___MACRO_TO_SCNRI   1
  ;;
  ROW_OFFSET___MACRO_TO_DECDR   ROW_OFFSET___DECDR_TO_MACRO
  ROW_OFFSET___SCNRI_TO_DECDR   ROW_OFFSET___DECDR_TO_SCNRI
  ROW_OFFSET___SCNRI_TO_MACRO   ROW_OFFSET___MACRO_TO_SCNRI
  ;;
  NEGATIVE_ROW_OFFSET___SCNRI_TO_DECDR   (- 0 ROW_OFFSET___SCNRI_TO_DECDR)
  NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO   (- 0 ROW_OFFSET___SCNRI_TO_MACRO)
  )

;;;;;;;;;;;;;;;;;;;
;;               ;;
;;   Constants   ;;
;;               ;;
;;;;;;;;;;;;;;;;;;;

(defconst
  nROWS_MSIZE    1
  nROWS_TRIV     2
  nROWS_MXPX     6
  nROWS_UPDT_B  10
  nROWS_UPDT_W  11

  CT_MAX_MSIZE    (-  nROWS_MSIZE   1)
  CT_MAX_TRIV     (-  nROWS_TRIV    1)
  CT_MAX_MXPX     (-  nROWS_MXPX    1)
  CT_MAX_UPDT_W   (-  nROWS_UPDT_W  1)
  CT_MAX_UPDT_B   (-  nROWS_UPDT_B  1)
  )

;;;;;;;;;;;;;;;;;;;;
;;                ;;
;;   Shorthands   ;;
;;                ;;
;;;;;;;;;;;;;;;;;;;;

(defun  (mxp-perspective-sum)
  (+    DECDR
	MACRO
	SCNRI
	CMPTN))

(defun  (mxp-perspective-wght-sum)
  (+    (*   1   DECDR )
	(*   2   MACRO )
	(*   3   SCNRI )
	(*   4   CMPTN )))

(defun  (mxp-ct-max-sum)
  (+    (*   CT_MAX_MSIZE    scenario/MSIZE                     )
	(*   CT_MAX_TRIV     scenario/TRIVIAL                   )
	(*   CT_MAX_MXPX     scenario/MXPX                      )
	(*   CT_MAX_UPDT_W   scenario/STATE_UPDATE_WORD_PRICING )
	(*   CT_MAX_UPDT_B   scenario/STATE_UPDATE_BYTE_PRICING )
	))

