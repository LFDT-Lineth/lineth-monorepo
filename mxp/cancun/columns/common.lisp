(module mxp)
;; mdule will be used from Cancun fork and on

(defcolumns
  ( MXP_STAMP   :i32          )
  ( CN          :i32          )
  ( DECODER     :binary@prove )
  ( MACRO       :binary@prove )
  ( SCENARIO    :binary@prove )
  ( COMPUTATION :binary@prove )
  ( CT          :i4           )
  ( CT_MAX      :i4           )
  )

(defalias
  DECDR  DECODER
  SCNRI  SCENARIO
  CMPTN  COMPUTATION
  )
