(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                                     ;;
;;   For POINT_EVALUATION, BLS_G1_ADD, BLS_G2_ADD, BLS_MAP_FP_TO_G1, BLS_MAP_FP2_TO_G2 ;;
;;                                                                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---standard-precondition)
                                                                    (+ IS_POINT_EVALUATION
                                                                       IS_BLS_G1_ADD
                                                                       IS_BLS_G2_ADD
                                                                       IS_BLS_MAP_FP_TO_G1
                                                                       IS_BLS_MAP_FP2_TO_G2))
(defun (fixed-cds)
                                                                    (+  (* PRECOMPILE_CALL_DATA_SIZE___POINT_EVALUATION  IS_POINT_EVALUATION)
                                                                        (* PRECOMPILE_CALL_DATA_SIZE___G1_ADD         IS_BLS_G1_ADD)
                                                                        (* PRECOMPILE_CALL_DATA_SIZE___G2_ADD         IS_BLS_G2_ADD)
                                                                        (* PRECOMPILE_CALL_DATA_SIZE___FP_TO_G1  IS_BLS_MAP_FP_TO_G1)
                                                                        (* PRECOMPILE_CALL_DATA_SIZE___FP2_TO_G2 IS_BLS_MAP_FP2_TO_G2)))
(defun (fixed-gast-cost)
                                                                    (+  (* GAS_CONST_POINT_EVALUATION  IS_POINT_EVALUATION)
                                                                        (* GAS_CONST_BLS_G1_ADD         IS_BLS_G1_ADD)
                                                                        (* GAS_CONST_BLS_G2_ADD         IS_BLS_G2_ADD)
                                                                        (* GAS_CONST_BLS_MAP_FP_TO_G1  IS_BLS_MAP_FP_TO_G1)
                                                                        (* GAS_CONST_BLS_MAP_FP2_TO_G2 IS_BLS_MAP_FP2_TO_G2)))
(defun (prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---precompile-cost)          (fixed-gast-cost))
(defun (prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---valid-cds)                (shift OUTGOING_RES_LO 2))
(defun (prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---sufficient-gas)           (- 1 (shift OUTGOING_RES_LO 3)))

(defconstraint prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---check-cds-validity (:guard (* (assumption---fresh-new-stamp) (prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---standard-precondition)))
  (call-to-EQ 2 0 (prc---cds) 0 (fixed-cds)))

(defconstraint prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---compare-call-gas-against-precompile-cost (:guard (* (assumption---fresh-new-stamp) (prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---standard-precondition)))
  (call-to-LT 3 0 (prc---callee-gas) 0 (prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---precompile-cost)))

(defconstraint prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---standard-precondition)))
  (begin (eq! (prc---hub-success) (* (prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---valid-cds) (prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---sufficient-gas)))
         (if-zero (prc---hub-success)
                  (vanishes! (prc---return-gas))
                  (eq! (prc---return-gas)
                       (- (prc---callee-gas) (prc-pointevaluation-prc-blsg1add-prc-blsg2add-prc-blsmapfptog1-prc-blsmapfp2tog2---precompile-cost))))))
