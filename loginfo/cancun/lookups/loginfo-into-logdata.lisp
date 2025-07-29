(defun (sel-loginfo-to-logdata)
  loginfo.TXN_EMITS_LOGS)

(defclookup
  loginfo-into-logdata
  ;; target columns
  (
    logdata.ABS_LOG_NUM_MAX
    logdata.ABS_LOG_NUM
    logdata.SIZE_TOTAL
  )
  ;; source selector
  (sel-loginfo-to-logdata)
  ;; source columns
  (
    loginfo.ABS_LOG_NUM_MAX
    loginfo.ABS_LOG_NUM
    loginfo.DATA_SIZE
  ))


