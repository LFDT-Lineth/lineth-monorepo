(module hub)

;; tcp_ ⇔ storage consistency permutation
(defpermutation
  ;; permuted columns
  ;; replace scp with storage_consistency_permutation
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (
    tcp_PEEK_AT_TRANSIENT
    tcp_ADDRESS_HI
    tcp_ADDRESS_LO
    tcp_STORAGE_KEY_HI
    tcp_STORAGE_KEY_LO
    tcp_ABS_TX_NUM
    tcp_DOM_STAMP
    tcp_SUB_STAMP
    ;;
    ;;
    tcp_VALUE_CURR_HI
    tcp_VALUE_CURR_LO
    tcp_VALUE_NEXT_HI
    tcp_VALUE_NEXT_LO
  )
  ;; original columns
  ;;;;;;;;;;;;;;;;;;;
  (
    (+ PEEK_AT_TRANSIENT        )
    (+ transient/ADDRESS_HI     )
    (+ transient/ADDRESS_LO     )
    (+ transient/STORAGE_KEY_HI )
    (+ transient/STORAGE_KEY_LO )
    (+ ABS_TX_NUM               )
    (+ DOM_STAMP                )
    (- SUB_STAMP                )
    ;;
    transient/VALUE_CURR_HI
    transient/VALUE_CURR_LO
    transient/VALUE_NEXT_HI
    transient/VALUE_NEXT_LO
    )
  )
