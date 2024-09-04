(defpurefun (sel-logdata-to-rlprcpt)
  logdata.LOGS_DATA)

(deflookup 
  logdata-into-rlptxrcpt
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
    (* logdata.ABS_LOG_NUM (sel-logdata-to-rlprcpt))
    (* RLP_RCPT_SUBPHASE_ID_DATA_LIMB (sel-logdata-to-rlprcpt))
    (* logdata.INDEX (sel-logdata-to-rlprcpt))
    (* logdata.LIMB (sel-logdata-to-rlprcpt))
    (* logdata.SIZE_LIMB (sel-logdata-to-rlprcpt))
  ))


