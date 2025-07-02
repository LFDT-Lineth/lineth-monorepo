(defun (sel-logdata-to-rlptxnrcpt)
  logdata.LOGS_DATA)

(defclookup
  logdata-into-rlptxnrcpt
  ;; target columns
  (
    rlptxrcpt.ABS_LOG_NUM
    rlptxrcpt.PHASE_ID
    rlptxrcpt.INDEX_LOCAL
    rlptxrcpt.LIMB
    rlptxrcpt.nBYTES
  )
  ;; source selector
  (sel-logdata-to-rlptxnrcpt)
  ;; source columns
  (
    logdata.ABS_LOG_NUM
    RLP_RCPT_SUBPHASE_ID_DATA_LIMB
    logdata.INDEX
    logdata.LIMB
    logdata.SIZE_LIMB
  ))


