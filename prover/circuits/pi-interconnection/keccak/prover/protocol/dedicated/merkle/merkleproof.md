## Design of the Merkle-tree hashing

This section describes our technique to verify binary Merkle-trees

### Structure

| IsInactive    | NewProof      | IsEndOfProof      | Root     | Curr        | Proof         | PosBit          | PosAcc                               | *Zero* | Left          | IntermState        | Right         | NodeHash          |
| ---           | ---           | ---               | -------- | ----------- | ------------- | --------------- | ------------------------------------ | ------ | ------------- | -----------        | ------------- | -----------       |
|  $0$          | $0$           | 1                 | $R_0$    | $H_{d-1,0}$ | $\pi_{d-1,0}$ | $b_{d-1,0} = 0$ | $p_{d-1,0} = b_{d-1,0}$              | $0$    | $H_{d-1,0}$   | $I_{d-1,0}$        | $\pi_{d-1,0}$ | $R_0$             |
|  $0$          | $0$           | $0$               | $R_0$    | $H_{d-2,0}$ | $\pi_{d-2,0}$ | $b_{d-2,0} = 1$ | $p_{d-2,0} = b_{d-2,0} + 2p_{d-1,0}$ | $0$    | $\pi_{d-2,0}$ | $I_{d-2,0}$        | $H_{d-2,0}$   | $H_{d-1,0}$       |
|  $\cdots$     | $\cdots$      | $\cdots$          | $\cdots$ | $\cdots$    | $\cdots$      | $\cdots$        | $\cdots$                             | $0$    | $\cdots$      | $\cdots$           | $\cdots$      | $\cdots$          |
|  $0$          | $1$           | $0$               | $R_0$    | $L_0$       | $\pi_{0,0}$   | $b_{0,0} = 1$   | $p_{d-2,0} = b_{0,0} + 2p_{1,0}$     | $0$    | $\pi_{0,0}$   | $I_{0,0}$          | $L_0$         | $H_{1,0}$         |
|  $0$          | $0$           | 1                 | $R_1$    | $H_{d-1,1}$ | $\pi_{d-1,1}$ | $b_{d-1,1} = 1$ | $p_{d-1,1} = b_{d-1,1}$              | $0$    | $\pi_{d-1,1}$ | $I_{d-1,1}$        | $H_{d-1,1}$   | $R_0$             |
|  $0$          | $0$           | $0$               | $R_1$    | $H_{d-2,1}$ | $\pi_{d-2,1}$ | $b_{d-2,1} = 1$ | $p_{d-2,1} = b_{d-2,1} + 2p_{d-1,1}$ | $0$    | $\pi_{d-2,1}$ | $I_{d-2,1}$        | $H_{d-2,1}$   | $H_{d-1,1}$       |
|  $\cdots$     | $\cdots$      | $\cdots$          | $\cdots$ | $\cdots$    | $\cdots$      | $\cdots$        | $\cdots$                             | $0$    | $\cdots$      | $\cdots$           | $\cdots$      | $\cdots$          |
|  $0$          | $1$           | $0$               | $R_1$    | $L_1$       | $\pi_{0,1}$   | $b_{0,1} = 0$   | $p_{d-2,1} = b_{0,1} + 2p_{1,1}$     | $0$    | $L_1$         | $I_{0,1}$          | $\pi_{0,1}$   | $H_{1,1}$         |
|  $\cdots$     | $\cdots$      | $\cdots$          | $\cdots$ | $\cdots$    | $\cdots$      | $\cdots$        | $\cdots$                             | $0$    | $\cdots$      | $\cdots$           | $\cdots$      | $\cdots$          |
|  $0$          | $1$           | $0$               | $R_n$    | $L_n$       | $\pi_{0,n}$   | $b_{0,n} = 0$   | $p_{d-2,n} = b_{0,n} + 2p_{1,n}$     | $0$    | $L_n$         | $I_{0,n}$          | $\pi_{0,n}$   | $H_{n,n}$         |
|  $1$          | $0$           | $0$               | $0$      | $0$         | $0$           | $0$             | $0$                                  | $0$    | $0$           | $I_\text{dead}$    | $0$           | $H_\text{dead}$   |
|  $\cdots$     | $\cdots$      | $\cdots$          | $\cdots$ | $\cdots$    | $\cdots$      | $\cdots$        | $\cdots$                             | $0$    | $\cdots$      | $\cdots$           | $\cdots$      | $\cdots$          |
|  $1$          | $0$           | $0$               | $0$      | $0$         | $0$           | $0$             | $0$                                  | $0$    | $0$           | $I_\text{dead}$    | $0$           | $H_\text{dead}$   |

The table below represents how we model Merkle-tree verification, each proof uses a segment of $d$ rows. In each segment, the Merkle root is recomputed from bottom-up. The zero column is implicitly not really committed to (it is a constant). This can be confusing, so we say "bottom" to mean the last row of a segment. Which corresponds to the first hash of the Merkle-proof

For convenience, we use the following expressions to constructs the constraints

- $\text{NotNewProof}[i] = 1 - \text{NewProof}[i]$
- $\text{IsActive}[i] = 1 - \text{IsInactive}[i]$
- $\text{NotEndOfProof}[i] = 1 - \text{EndOfProof}[i]$

This are not materialized by columns but are used as subexpressions in the constraints for clarity.

#### Root is constant over a segment

Root is constant within a segment and it must be inactive when the "IsInactive" flag is set.

Global : $(\text{Root}[i] - \text{IsActive}[i]\text{Root}[i+1])\text{NotNewProof[i]} == 0$

#### For each segment the root is result of the topmost hash

Global : $\text{EndOfProof[i]}(\text{Root}[i] - \text{NodeHash}[i]) == 0$

#### PosBit is boolean

This enforces $\text{PosBit}$ to be boolean and zero if the inactive flag is set.

Global : $\text{PosBit}[i] = \text{IsActive}[i]\text{PosBit}[i]^2$

#### PosAcc should compute the final position

PosAcc progressively computes the position of the opened leaf from the bits.
It must be zero when the inactive flag is set.

Global : $\text{PosAcc}[i] = \text{IsActive}[i](\text{PosBit}[i] + 2 \text{NotEndOfProof}[i]\text{PosAcc}[i+1])$

#### Left and Right should be correctly passed

The flag $\text{PosBit}$ decides which one of $\text{Proof}$ or $\text{Curr}$ is mapped to $\text{Left}$ or $\text{Right}$.
Since, we enforce both proofs and curr to be zero when the inactive flag is set the constraints do not have to account for that : $\text{Left}$ and $\text{Right}$ are already enforced to be $0$.

- Global : $\text{Left}[i] - \text{PosBit}[i] \text{Proof}[i] - (1 - \text{PosBit}[i])\text{Curr}[i]$
- Global : $\text{Right}[i] - \text{PosBit}[i] \text{Curr}[i] - (1 - \text{PosBit}[i])\text{Proof}[i]$

#### Within a chunk, use the previous node hash as current node

When the inactive flag is set, this enforces that $\text{Curr}$ is zero. This works because the 

Global : $\text{NotNewProof}[i](\text{Curr}[i] - \text{IsActive}[i]\text{NodeHash}[i+1])$

#### The MiMC are well-computed

MiMC: $(\text{Left}, \text{Zero}, \text{Interm})$
MiMC: $(\text{Right}, \text{Interm}, \text{NodeHash})$

#### Proof is canceled when inactive

Global : $\text{Proof}[i] = \text{IsActive}[i]\text{Proof}[i]$

#### Constraint on Leaf
Leaf is constant within a segment and at the bottom of it, it should equal Curr. This leaf must be zero when the inactive flag is up. We could have set the below global constraint,

Global: $\text{Leaf}[i] = \text{IsActive}[i]\text{NotNewProof}[i]\text{Leaf}[i+1]+\text{NewProof}[i]\text{Curr}[i]$.

But, instead we register the $\text{Leaf}$ column in the result module and 
verify that all values in $\text{Leaf}$ are included in the $\text{Curr}$ column of the Compute module via a lookup query. This saves us 1 global constraint. 
### Optional queries to check reuse of Merkle proofs
Suppose there are two Merkle proofs, one before and one after a leaf update of the tree. In this case, we are reusing the same merkle proof/tree and the `siblings` will be the same (ofcourse the roots are different). This kind of operation are the basic building block of the state manager (in particular, the accumulator module). To verify the reuse of merkle proofs, we resort to the below idea.

We introduce a new column $\text{UseNextMerkleProof}$ and a boolean variable $\text{withOptProofReuseCheck}$. When the bool variable is false, the column $\text{UseNextMerkleProof}$ is essentially a zero column. When it is true, it is constructed as 1s of length `Depth` followed by 0s of length `Depth` and so on upto `numProof`. To justify this structure, we refer to the below table. Here we have `Depth` = 2 and 1 update (`numProof` = 2). If it is a reuse of Merkle proofs, then the sibling hashes and the position bits are the same for the two consecutive proofs. For example, say we have $\text{proof}_1 = (\text{pos}, (h_1, h_2))$ and $\text{proof}_2=(\text{pos}, (h_1, h_2))$. Let $\text{pos} = (b_0, b_1)$. 

| UseNextMerkleProof | Proof   | Proof(i+depth) | SegmentCounter | PosBit | PosBit[i+depth] |
|--------------------|---------|------------------|----------------|-------|----------------|
|          1         | $h_1$   | $h_1$            |  0             | $b_0$ |  $b_0$         | 
|          1         | $h_2$   | $h_2$            |  0             | $b_1$ |  $b_1$         |
|          0         | $h_1$   |  -               |  1             | $b_0$ |   -            |
|          0         | $h_2$   |  -               |  1             | $b_1$ |   -            |
---------------------------------------------------------------------------------------------

We put the proofs (`Siblings`) consecutively in the $\text{Proof}$ column. We also have `SegmentCounter` column to verify the sequentiality of the Merkle proof assignments in the `Accumulator` module. This is done to ensure that both the `Merkle` and the `Accumulator` module verifies the Merkle proofs in the same order (via a lookup constraint). Then reuse of Merkle proofs is verified by the below constraints,
$$
\begin{aligned}
\text{UseNextMerkleProof}[i]* \text{IsActive[i]} * (\text{Proof}[i] * (\text{SegmentCounter}[i] + 1)-\text{Proof}[i+\text{depth}] * \text{SegmentCounter}[i+\text{depth}]) &= 0 \\
\text{UseNextMerkleProof}[i]* \text{IsActive[i]} * (\text{PosBit}[i] * (\text{SegmentCounter}[i] + 1)-\text{PosBit}[i+\text{depth}] * \text{SegmentCounter}[i+\text{depth}]) &= 0 
\end{aligned}
$$
 Note that similar technique can be used if we pack multiple update operations consecutively. Since an Insert and a Delete operation can be thought of 3 update operations in three different positions, this trick is useful for wizard verification of the state manager operations.

We also need constraints to show that the columns $\text{UseNextMerkleProof}$ and $\text{SegmentCounter}$ are constant throughout a particular proof segment and the value of $\text{SegmentCounter}$ is incremented by 1 in the next segment.