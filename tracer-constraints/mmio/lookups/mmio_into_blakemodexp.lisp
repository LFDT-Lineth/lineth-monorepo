(defclookup
  (mmio-into-blake2fmodexpdata :unchecked)
  ;; target columns
  (
    blake2fmodexpdata.ID
    blake2fmodexpdata.PHASE
    blake2fmodexpdata.INDEX
    blake2fmodexpdata.LIMB
  )
  ;; source selector
  mmio.EXO_IS_BLAKEMODEXP
  ;; source columns
  (
   mmio.EXO_ID
   mmio.PHASE
   mmio.INDEX_X
   mmio.LIMB
 ))


