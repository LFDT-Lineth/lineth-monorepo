(defun (sel-logdata-to-rlptxnrcpt)
  logdata.LOGS_DATA)

(deflookup
  logdata-into-rlptxnrcpt
  ;reference columns
  (
    rlptxrcpt.ABS_LOG_NUM
    rlptxrcpt.PHASE_ID
    rlptxrcpt.INDEX_LOCAL
    rlptxrcpt.LIMB
    rlptxrcpt.nBYTES
  )
  ;source columns
  (
    (* logdata.ABS_LOG_NUM            (sel-logdata-to-rlptxnrcpt))
    (* RLP_RCPT_SUBPHASE_ID_DATA_LIMB (sel-logdata-to-rlptxnrcpt))
    (* logdata.INDEX                  (sel-logdata-to-rlptxnrcpt))
    (* logdata.LIMB                   (sel-logdata-to-rlptxnrcpt))
    (* logdata.SIZE_LIMB              (sel-logdata-to-rlptxnrcpt))
  ))


