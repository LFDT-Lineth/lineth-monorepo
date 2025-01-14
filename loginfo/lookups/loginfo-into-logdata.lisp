(defun (sel-loginfo-to-logdata)
  loginfo.TXN_EMITS_LOGS)

(deflookup
  loginfo-into-logdata
  ;; target columns
  (
    logdata.ABS_LOG_NUM_MAX
    logdata.ABS_LOG_NUM
    logdata.SIZE_TOTAL
  )
  ;; source columns
  (
    (* loginfo.ABS_LOG_NUM_MAX (sel-loginfo-to-logdata))
    (* loginfo.ABS_LOG_NUM (sel-loginfo-to-logdata))
    (* loginfo.DATA_SIZE (sel-loginfo-to-logdata))
  ))


