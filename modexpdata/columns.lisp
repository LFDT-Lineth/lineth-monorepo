(module modexpdata)

(defcolumns 
  (STAMP :byte)
  (CT :byte)
  (RESULT_DATA_CONTEXT :i128)
  (BEMR :byte)
  (PHASE :byte)
  (INDEX :byte :display :dec)
  (LIMB :i16 :display :bytes)
  (BYTES :byte))


