(module mxp)

;;;;;;;;;;;;;;;;;;;;;
;;                 ;;
;;   Row offsets   ;;
;;                 ;;
;;;;;;;;;;;;;;;;;;;;;

(defconst
  ROW_OFFSET___1ST_SIZE___ZERONESS_TEST         1
  ROW_OFFSET___2ND_SIZE___ZERONESS_TEST         2
  ROW_OFFSET___1ST_SIZE___SMALLNESS_TEST        3
  ROW_OFFSET___2ND_SIZE___SMALLNESS_TEST        4
  ROW_OFFSET___1ST_OFFSET___SMALLNESS_TEST      5
  ROW_OFFSET___2ND_OFFSET___SMALLNESS_TEST      6
  ROW_OFFSET___COMPARISON_OF_MAX_OFFSETS        7
  ROW_OFFSET___FLOOR_OF_MAX_OFFSET_OVER_32      8
  ROW_OFFSET___FLOOR_OF_SQUARE_OVER_512         9
  ROW_OFFSET___COMPARISON_OF_WORDS_AND_EYP_A   10
  ROW_OFFSET___CEILING_OF_SIZE_OVER_32         11
  )

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;   Scenario guards   ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;


;; scenario guards
(defun   (mxp-guard---msize)                       (* SCENARIO   scenario/MSIZE                     ))
(defun   (mxp-guard---trivial)                     (* SCENARIO   scenario/TRIVIAL                   ))
(defun   (mxp-guard---mxpx)                        (* SCENARIO   scenario/MXPX                      ))
(defun   (mxp-guard---state-update-word-pricing)   (* SCENARIO   scenario/STATE_UPDATE_WORD_PRICING ))
(defun   (mxp-guard---state-update-byte-pricing)   (* SCENARIO   scenario/STATE_UPDATE_BYTE_PRICING ))

;; umbrella scenario shorthands
(defun   (mxp-guard---not-msize)                   (* SCENARIO   (mxp-scenario-shorthand---not-msize)))
(defun   (mxp-guard---not-msize-not-trivial)       (* SCENARIO   (mxp-scenario-shorthand---not-msize-nor-trivial)))
(defun   (mxp-guard---state-update)                (* SCENARIO   (mxp-scenario-shorthand---state-update)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;   Macro-row parameter shorthands   ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun   (mxp-shorthand---offset-1-hi)   (shift   macro/OFFSET_1_HI   NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO))
(defun   (mxp-shorthand---offset-1-lo)   (shift   macro/OFFSET_1_LO   NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO))
(defun   (mxp-shorthand---size-1-hi)     (shift   macro/SIZE_1_HI     NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO))
(defun   (mxp-shorthand---size-1-lo)     (shift   macro/SIZE_1_LO     NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO))
(defun   (mxp-shorthand---offset-2-hi)   (shift   macro/OFFSET_2_HI   NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO))
(defun   (mxp-shorthand---offset-2-lo)   (shift   macro/OFFSET_2_LO   NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO))
(defun   (mxp-shorthand---size-2-hi)     (shift   macro/SIZE_2_HI     NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO))
(defun   (mxp-shorthand---size-2-lo)     (shift   macro/SIZE_2_LO     NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                  ;;
;;   Instruction-decoder-row parameter shorthands   ;;
;;                                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun   (mxp-shorthand---single-offset-instruction)   (shift   decoder/IS_SINGLE_MAX_OFFSET   NEGATIVE_ROW_OFFSET___SCNRI_TO_DECDR))
(defun   (mxp-shorthand---double-offset-instruction)   (shift   decoder/IS_DOUBLE_MAX_OFFSET   NEGATIVE_ROW_OFFSET___SCNRI_TO_DECDR))
(defun   (mxp-shorthand---word-pricing-instruction)    (shift   decoder/IS_WORD_PRICING        NEGATIVE_ROW_OFFSET___SCNRI_TO_DECDR))
(defun   (mxp-shorthand---byte-pricing-instruction)    (shift   decoder/IS_BYTE_PRICING        NEGATIVE_ROW_OFFSET___SCNRI_TO_DECDR))
