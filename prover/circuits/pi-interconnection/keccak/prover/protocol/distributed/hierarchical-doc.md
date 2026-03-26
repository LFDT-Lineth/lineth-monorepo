# Hierarchical aggregation

This short document describes the hierarchical aggregation scheme. The scheme
allows aggregating the segment subproofs 2-by-2 in any order to reduce the 
latency as much as possible.

Crucially, the scheme relies on an LtHash, an order-independant hash function. 
The proofs are recursively verified as we do with the all-at-once conglomeration
, but the main change is that the scheme allow recursively aggregating the 
conglomeration proofs. The list of the "acceptable" verification keys of the 
aggregated proofs can no longer be fixed when defining the hierarchical 
conglomeration circuit and must be defined as public input which is ultimately
enforced by the verifier.

A second aspect of the hierarchical conglomeration is how the public inputs are
managed during the aggregation. Crucially, the conglomeration must guarantee the
following properties.

- All the segment proofs are from a list of whitelisted circuits
- At the end of the process, all the segments have been verified
    - This is done by a check on the public inputs by the outer-circuit
- All the segments have the same view of the randomness generation and the
    randomness has been correctly generated.
- The logderivative sum, grand product etc.. are holistically valid
- The "from-prev-to-next-segments" checks (HornerN0Hash and GlobalSendReceive) 
    are verified
- The LPP commitments are consistent between GL and LPP of the same segments and 
    the pair should have verification keys for the same module
- The functional inputs are propagated up through the aggregation steps


## Making it possible to detect that all the segments have been aggregated

For each of the type of proofs (GL/LPP/AGG), we introduce the following public
inputs.

- SegmentCountGL: A list of inputs containing the count of all the proofs that have been aggregated so far by module (only for the LPP)
- SegmentCountLPP: A list of inputs containing the count of all the proofs that have been aggregated so far by module (only for the GL)
- TargetCount: A list of inputs containing the count of all the segments that are to be aggregated by module.

When aggregating two proofs, we sum the "SegmentCountXXX" public input for the two sub-proofs and we also check that the two subproofs have the same "TargetCount" counts and we just propagate the same value upward.

We know that a subproof has aggregated all the segments if the TargetCount is reached for both SegmentCountGL and SegmentCountLPP.

## Proving the unicity of the segments

This is not really needed. Each segment is taking part in a succession of N0Hash
and GlobalSent/ReceiveHash. Having a valid sequence is OK and sufficient for us.

## Proving the segments are from a list of whitelisted circuits

Each segment comes with a pair of field element to represent the verifying key.
The aggregation circuit checks if the segment has the right verifying key by
comparing it with the entries of a Merkle tree.

## Actions
- [] Adding the segment counting public inputs
    - [] GL
    - [] LPP
    - [] Hierarchical
- [] Adding the segment target propagation
    - [] GL
    - [] LPP
    - [] Hierarchicals
