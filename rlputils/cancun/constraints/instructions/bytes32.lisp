(module rlputils)

;; hook
(defun (bytes32--instruction-precondition) (* MACRO IS_BYTE32))

;; setting ct max
(defconstraint byte32--setting-ct-max (:guard (bytes32--instruction-precondition))      (vanishes! CT_MAX))

;; 
(defconstraint byte32--first-wcp-call   (:guard (bytes32--instruction-precondition)) 
    (begin
    (wcp-call-geq        1 macro/DATA_1 macro/DATA_2 0)
    (result-must-be-true 1 )))
