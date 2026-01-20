(defun (blakemodexp-to-blake2f-selector)
(* (- 1 (prev blake2fmodexpdata.IS_BLAKE_DATA)) blake2fmodexpdata.IS_BLAKE_DATA))

(defclookup
(blakemodexp-into-blake2f :unchecked)
;; unchecked because of r and f
;; target columns
(
 blake2f.h0h1_be_input
 blake2f.h2h3_be_input
 blake2f.h4h5_be_input
 blake2f.h6h7_be_input
 blake2f.m0m1_be
 blake2f.m2m3_be
 blake2f.m4m5_be
 blake2f.m6m7_be
 blake2f.m8m9_be
 blake2f.m10m11_be
 blake2f.m12m13_be
 blake2f.m14m15_be
 blake2f.t0t1_be
 blake2f.r
 blake2f.f
 blake2f.h0h1_be
 blake2f.h2h3_be
 blake2f.h4h5_be
 blake2f.h6h7_be
)
;; source selector
(blakemodexp-to-blake2f-selector)
;; source columns
(
 (shift blake2fmodexpdata.LIMB 0 )
 (shift blake2fmodexpdata.LIMB 1 )
 (shift blake2fmodexpdata.LIMB 2 )
 (shift blake2fmodexpdata.LIMB 3 )
 (shift blake2fmodexpdata.LIMB 4 )
 (shift blake2fmodexpdata.LIMB 5 )
 (shift blake2fmodexpdata.LIMB 6 )
 (shift blake2fmodexpdata.LIMB 7 )
 (shift blake2fmodexpdata.LIMB 8 )
 (shift blake2fmodexpdata.LIMB 9 )
 (shift blake2fmodexpdata.LIMB 10)
 (shift blake2fmodexpdata.LIMB 11)
 (shift blake2fmodexpdata.LIMB 12)
 (shift blake2fmodexpdata.LIMB 13)
 (shift blake2fmodexpdata.LIMB 14)
 (shift blake2fmodexpdata.LIMB 15)
 (shift blake2fmodexpdata.LIMB 16)
 (shift blake2fmodexpdata.LIMB 17)
 (shift blake2fmodexpdata.LIMB 18)
))
