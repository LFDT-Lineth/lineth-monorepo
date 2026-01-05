(defun (hub-into-oob-trigger)
  (* hub.PEEK_AT_MISCELLANEOUS
     hub.misc/OOB_FLAG))

(defclookup hub-into-oob
  ;; target columns
  (
   oob.INST
   oob.DATA_1
   oob.DATA_2
   oob.DATA_3
   oob.DATA_4
   oob.DATA_5
   oob.DATA_6
   oob.DATA_7
   oob.DATA_8
   oob.DATA_9
   oob.DATA_10
  )
  ;; source selector
  (hub-into-oob-trigger)
  ;; source columns
  (
    hub.misc/OOB_INST
   [hub.misc/OOB_DATA  1]
   [hub.misc/OOB_DATA  2]
   [hub.misc/OOB_DATA  3]
   [hub.misc/OOB_DATA  4]
   [hub.misc/OOB_DATA  5]
   [hub.misc/OOB_DATA  6]
   [hub.misc/OOB_DATA  7]
   [hub.misc/OOB_DATA  8]
   [hub.misc/OOB_DATA  9]
   [hub.misc/OOB_DATA 10]
  )
)
