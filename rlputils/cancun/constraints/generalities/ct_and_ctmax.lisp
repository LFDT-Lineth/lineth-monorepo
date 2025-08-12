(module rlputils)

(defproperty      macro-is-ct-constant
	(counter-constant CT_MAX   CT))

(defconstraint ct-vanishes-outside-compt ()
    (if-zero COMPT 
        (begin 
        (vanishes! CT_MAX)
        (vanishes! CT))))

(defconstraint ct-update ()
    (if (== CT CT_MAX) 
        (will-eq! CT 0)
        (will-eq! CT (+ CT 1))))