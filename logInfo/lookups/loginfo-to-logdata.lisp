(defpurefun (sel-logInfo-to-logData)
  logInfo.TXN_EMITS_LOGS)

(deflookup 
  logInfo-into-logdata
  ;; target columns
  (
    logData.ABS_LOG_NUM_MAX
    logData.ABS_LOG_NUM
    logData.SIZE_TOTAL
  )
  ;; source columns
  (
    (* logInfo.ABS_LOG_NUM_MAX (sel-logInfo-to-logData))
    (* logInfo.ABS_LOG_NUM (sel-logInfo-to-logData))
    (* logInfo.DATA_SIZE (sel-logInfo-to-logData))
  ))


