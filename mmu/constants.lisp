(module mmu)

(defconst
  ;;
  ;; MMU NB OF PP ROWS
  ;;
  NB_PP_ROWS_MLOAD                                   1
  NB_PP_ROWS_MSTORE                                  1
  NB_PP_ROWS_MSTORE8                                 1
  NB_PP_ROWS_INVALID_CODE_PREFIX                     1
  NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION            5
  NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING                 4
  NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS                  1
  NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING                 5
  NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING    4
  NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA       10
  NB_PP_ROWS_MODEXP_ZERO                             1
  NB_PP_ROWS_MODEXP_DATA                             6
  NB_PP_ROWS_BLAKE                                   2
  ;;
  ;; MMU NB OF PP ROWS + 1
  ;;
  NB_PP_ROWS_MLOAD_PO                                (+ NB_PP_ROWS_MLOAD 1)
  NB_PP_ROWS_MSTORE_PO                               (+ NB_PP_ROWS_MSTORE 1)
  NB_PP_ROWS_MSTORE8_PO                              (+ NB_PP_ROWS_MSTORE8 1)
  NB_PP_ROWS_INVALID_CODE_PREFIX_PO                  (+ NB_PP_ROWS_INVALID_CODE_PREFIX 1)
  NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO         (+ NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION 1)
  NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO              (+ NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING 1)
  NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS_PO               (+ NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS 1)
  NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO              (+ NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING 1)
  NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO (+ NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING 1)
  NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO    (+ NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA 1)
  NB_PP_ROWS_MODEXP_ZERO_PO                          (+ NB_PP_ROWS_MODEXP_ZERO 1)
  NB_PP_ROWS_MODEXP_DATA_PO                          (+ NB_PP_ROWS_MODEXP_DATA 1)
  NB_PP_ROWS_BLAKE_PO                                (+ NB_PP_ROWS_BLAKE 1)
  ;;
  ;; MMU NB OF PP ROWS + 2
  ;;
  NB_PP_ROWS_MLOAD_PT                                (+ NB_PP_ROWS_MLOAD 2)
  NB_PP_ROWS_MSTORE_PT                               (+ NB_PP_ROWS_MSTORE 2)
  NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT         (+ NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION 2)
  NB_PP_ROWS_BLAKE_PT                                (+ NB_PP_ROWS_BLAKE 2)
  ;;
  ;; MMU NB OF micro-processing rows
  ;;
  NB_MICRO_ROWS_TOT_MLOAD                            2
  NB_MICRO_ROWS_TOT_MSTORE                           2
  NB_MICRO_ROWS_TOT_MSTORE_EIGHT                     1
  NB_MICRO_ROWS_TOT_INVALID_CODE_PREFIX              1
  NB_MICRO_ROWS_TOT_RIGHT_PADDED_WORD_EXTRACTION     2
  ;;NB_MICRO_ROWS_TOT_RAM_TO_EXO_WITH_PADDING              variable
  ;;NB_MICRO_ROWS_TOT_EXO_TO_RAM_TANSPLANTS                variable
  ;;NB_MICRO_ROWS_TOT_RAM_TO_RAM_SANS_PADDING              variable
  ;;NB_MICRO_ROWS_TOT_ANY_TO_RAM_WITH_PADDING_PURE_PADDING variable
  ;;NB_MICRO_ROWS_TOT_ANY_TO_RAM_WITH_PADDING_SOME_DATA    variable
  NB_MICRO_ROWS_TOT_MODEXP_ZERO                      64
  NB_MICRO_ROWS_TOT_MODEXP_DATA                      64
  NB_MICRO_ROWS_TOT_BLAKE                            2)


