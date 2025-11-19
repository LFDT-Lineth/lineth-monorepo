(module rlputils)

(defclookup
  rlp-utils-into-wcp
  ; target columns
  (
    wcp.ARG_1
    wcp.ARG_2
    wcp.RES
    wcp.INST
    )
  ; source selector
  rlputils.COMPT
  ; source columns
  (
    (:: compt/ARG_1_HI compt/ARG_1_LO)
    compt/ARG_2_LO
    compt/RES
    compt/INST
 ))

(defcall
  ;; return(s)
  (compt/WCP_CT_MAX)
  ;; function
  maxlog
  ;; argument(s)
  (compt/INST compt/ARG_1_HI compt/ARG_1_LO compt/ARG_2_LO)
  ;; source selector
  (!= 0 COMPT))
