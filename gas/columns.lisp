(module gas)

(defcolumns
  (INPUTS_AND_OUTPUTS_ARE_MEANINGFUL    :binary@prove)
  (FIRST                                :binary@prove)
  (CT                                   :i3)
  (CT_MAX                               :i3)
  (GAS_ACTUAL                           :i64)
  (GAS_COST                             :i64)
  (EXCEPTIONS_AHOY                      :binary@prove)
  (OUT_OF_GAS_EXCEPTION                 :binary@prove)
  (WCP_ARG1_LO                          :i128)
  (WCP_ARG2_LO                          :i128)
  (WCP_INST                             :byte@prove :display :opcode)
  (WCP_RES                              :binary@prove))

(defalias
  IOMF  INPUTS_AND_OUTPUTS_ARE_MEANINGFUL
  XAHOY EXCEPTIONS_AHOY
  OOGX  OUT_OF_GAS_EXCEPTION)


