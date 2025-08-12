(module rlputils)

(defconstraint ct-constancies ()
    (begin 
    (counter-constant COMPT        CT)
    (counter-constant CT_MAX       CT)
    (counter-constant compt/LIMB   CT)))

(defproperty      macro-is-ct-constant
	(counter-constant MACRO        CT))