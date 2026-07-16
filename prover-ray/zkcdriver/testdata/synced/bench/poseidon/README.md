# Generating data

The Poseidon hash function requires two pieces of data besides a _nonempty_ input byte slice:

- the mds matrix of size t×t
- the round constants a matrix of size r×t

where t ≡ state width t and r ≡ total rounds. This data can be generated using the
[poseidon-tools](https://github.com/khovratovich/poseidon-tools) library.

## Generatring an mds matrix

The following generates the MDS matrix for the koalabear prime with state width 16.

```bash
 python3
>>> from poseidon.mds_matrix import generate_mds_matrix
>>> KOALABEAR_P = 2130706433
>>> STATE_WIDTH = 16
>>> [[f"{k:08x}" for k in l] for l in generate_mds_matrix(STATE_WIDTH, KOALABEAR_P)]
```

## Getting round constants

For the round constants I used `_KB_ROUND_CONSTANTS_16` from the tests of that repo.

## Stuff

```bash
 python3
>>> from poseidon.poseidon import Poseidon
>>> KOALABEAR_P = 2130706433
>>> pos = Poseidon(prime=KOALABEAR_P, alpha=3, t=16, r_f=8, r_p=20)
# t is the state_width, no rate is specified (it defaults to t-1), r_f is the number of full rounds (which has to be
# even), r_p that of partial rounds
>>> pos.sponge_hash(list(range(16)), 1)
[584229223]
>>> pos.sponge_hash(list(range(16)), 15)
[584229223, 1225903167, 435734976, 745693090, 1580884015, 1393870516, 1514786559, 1416327482, 401740899, 305698337, 123847430, 1985271412, 660999169, 1953826170, 1390527262]
# 1 and 15 are the the respectve output_size's, which have to be ≤ rate
```
