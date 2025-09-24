(module hub)


(definterleaved PEEK_AT_STACK_POW_4   (PEEK_AT_STACK                  PEEK_AT_STACK                  PEEK_AT_STACK                  PEEK_AT_STACK                 ))
(definterleaved CN_POW_4              (CONTEXT_NUMBER                 CONTEXT_NUMBER                 CONTEXT_NUMBER                 CONTEXT_NUMBER                ))
(definterleaved STACK_STAMP_1234      ([stack/STACK_ITEM_STAMP    1]  [stack/STACK_ITEM_STAMP    2]  [stack/STACK_ITEM_STAMP    3]  [stack/STACK_ITEM_STAMP    4] ))  ;; ""
(definterleaved HEIGHT_1234           ([stack/STACK_ITEM_HEIGHT   1]  [stack/STACK_ITEM_HEIGHT   2]  [stack/STACK_ITEM_HEIGHT   3]  [stack/STACK_ITEM_HEIGHT   4] ))  ;; ""
(definterleaved POP_1234              ([stack/STACK_ITEM_POP      1]  [stack/STACK_ITEM_POP      2]  [stack/STACK_ITEM_POP      3]  [stack/STACK_ITEM_POP      4] ))  ;; ""
(definterleaved VALUE_HI_1234         ([stack/STACK_ITEM_VALUE_HI 1]  [stack/STACK_ITEM_VALUE_HI 2]  [stack/STACK_ITEM_VALUE_HI 3]  [stack/STACK_ITEM_VALUE_HI 4] ))  ;; ""
(definterleaved VALUE_LO_1234         ([stack/STACK_ITEM_VALUE_LO 1]  [stack/STACK_ITEM_VALUE_LO 2]  [stack/STACK_ITEM_VALUE_LO 3]  [stack/STACK_ITEM_VALUE_LO 4] ))  ;; ""

;; stkcp_ ⇔ stack consistency permutation
(defpermutation
  ;; row-permuted columns
  (
   stkcp_PEEK_AT_STACK_POW_4
   stkcp_CN_POW_4
   stkcp_HEIGHT_1234
   stkcp_STACK_STAMP_1234
   stkcp_POP_1234
   stkcp_VALUE_HI_1234
   stkcp_VALUE_LO_1234
   )
  ;; underlying columns
  (
   (↓ PEEK_AT_STACK_POW_4) 
   (↓ CN_POW_4) 
   (↓ HEIGHT_1234) 
   (↓ STACK_STAMP_1234) 
   POP_1234
   VALUE_HI_1234
   VALUE_LO_1234
   ) 
  )
