package poseidon2_koalabear

// Contains the poseidon2 implem over koalabear, both out of circuit and in circuit.
// The circuits use frontend.Variable instead of WrappedVariable because a circuit using poseidon2 on koalabear
// will alsways be compiled on koalabear, otherwise we use poseidon2 on bls12377.
