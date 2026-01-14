(module rlptxn)

(defconstraint ct-constancies-of-access-list-subphase ()
               (begin
                 (counter-constant   IS_PREFIX_OF_ACCESS_LIST_ITEM   CT)
                 (counter-constant   IS_PREFIX_OF_STORAGE_KEY_LIST   CT)
                 (counter-constant   IS_ACCESS_LIST_ADDRESS          CT)
                 (counter-constant   IS_ACCESS_LIST_STORAGE_KEY      CT)))

(defconstraint access-list-flag-exclusivity ()
               (eq! (is-access-list-data)
                    (* (prev CMP) CMP IS_ACCESS_LIST)))

