(defun (sel-rlptxn-into-rlpauth) (* rlptxn.IS_AUTHORIZATION_LIST (* (prev rlptxn.CMP) rlptxn.CMP )))

(defclookup
  (rlptxn-into-rlpauth :unchecked)
  ;; target selector
  rlpauth.not_padding_selector ;; this selector just filters out padding rows (and it is necessary as rlpauth is a multiline function)
  ;; target columns
  (
    rlpauth.user_txn_number
    rlpauth.tuple_index
    rlpauth.chain_id
    rlpauth.delegation_address
    rlpauth.nonce
    rlpauth.y_parity
    rlpauth.r
    rlpauth.s
  )
  ;; source selector
  (sel-rlptxn-into-rlpauth)
  ;; source columns
  (
    rlptxn.USER_TXN_NUMBER
    rlptxn.cmp/AUX_CCC_1 ;; tuple_index
    (:: rlptxn.cmp/AUX_8 rlptxn.cmp/AUX_3) ;; chain_id
    (:: rlptxn.cmp/AUX_CCC_4 rlptxn.cmp/AUX_CCC_5) ;; delegation_address
    rlptxn.cmp/AUX_CCC_2 ;; nonce
    rlptxn.cmp/AUX_CCC_3 ;; y_parity
    (:: rlptxn.cmp/AUX_4 rlptxn.cmp/AUX_5) ;; r
    (:: rlptxn.cmp/AUX_6 rlptxn.cmp/AUX_7) ;; s
  ))
