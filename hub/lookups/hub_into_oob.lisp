(defun (hub-into-oob-trigger)
  (and hub.PEEK_AT_MISCELLANEOUS
       hub.misc/OOB_FLAG))

(deflookup hub-into-oob
           ;; target columns
	   (
	     oob.OOB_INST
	     [oob.DATA 1]
	     [oob.DATA 2]
	     [oob.DATA 3]
	     [oob.DATA 4]
	     [oob.DATA 5]
	     [oob.DATA 6]
	     [oob.DATA 7]
	     [oob.DATA 8]
	     [oob.DATA 9]
           )
           ;; source columns
	   (
	     (* hub.misc/OOB_INST               (hub-into-oob-trigger))
	     (* [hub.misc/OOB_DATA 1]           (hub-into-oob-trigger))
	     (* [hub.misc/OOB_DATA 2]           (hub-into-oob-trigger))
	     (* [hub.misc/OOB_DATA 3]           (hub-into-oob-trigger))
	     (* [hub.misc/OOB_DATA 4]           (hub-into-oob-trigger))
	     (* [hub.misc/OOB_DATA 5]           (hub-into-oob-trigger))
	     (* [hub.misc/OOB_DATA 6]           (hub-into-oob-trigger))
	     (* [hub.misc/OOB_DATA 7]           (hub-into-oob-trigger))
	     (* [hub.misc/OOB_DATA 8]           (hub-into-oob-trigger))
	     (* [hub.misc/OOB_DATA 9]           (hub-into-oob-trigger))
           )
)
