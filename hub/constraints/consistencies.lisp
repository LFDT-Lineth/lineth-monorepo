(module hub)

;; Transaction constancies
(defconstraint abs_tx_constancies ()
  (begin
    (stamp-constancy    ABSOLUTE_TRANSACTION_NUMBER    RELATIVE_BLOCK_NUMBER)))

;; Stamp Constancies
;; is-stamp-constant should only be applied to stamp columns that grow by 0 or 1 each row.
(defun  (is-stamp-constant    stamp   col)
  (if-not-zero (did-inc! stamp 1)
    (remained-constant! col)))

(defconstraint stamp_hub_constancies ()
  (begin
    (is-stamp-constant HUB_STAMP CMC)))
