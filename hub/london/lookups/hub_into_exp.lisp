(defun (hub-into-exp-trigger)
  (* hub.PEEK_AT_MISCELLANEOUS
     hub.misc/EXP_FLAG))

(defclookup hub-into-exp
  ;; target columns
  (
   exp.MACRO
   exp.macro/EXP_INST
   [exp.macro/DATA 1]
   [exp.macro/DATA 2]
   [exp.macro/DATA 3]
   [exp.macro/DATA 4]
   [exp.macro/DATA 5]
  )
  ;; source selector
  (hub-into-exp-trigger)
  ;; source columns
  (
   1
   hub.misc/EXP_INST
   [hub.misc/EXP_DATA 1]
   [hub.misc/EXP_DATA 2]
   [hub.misc/EXP_DATA 3]
   [hub.misc/EXP_DATA 4]
   [hub.misc/EXP_DATA 5]
  )
)
