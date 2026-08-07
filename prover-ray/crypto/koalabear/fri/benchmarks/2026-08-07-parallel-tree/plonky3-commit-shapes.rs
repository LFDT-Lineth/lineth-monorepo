use std::env;
use std::hint::black_box;
use std::time::Instant;

use p3_commit::Mmcs;
use p3_dft::{Radix2DitParallel, TwoAdicSubgroupDft};
use p3_field::{Field, PrimeCharacteristicRing};
use p3_koala_bear::{KoalaBear, Poseidon2KoalaBear};
use p3_matrix::bitrev::BitReversibleMatrix;
use p3_matrix::dense::RowMajorMatrix;
use p3_matrix::Matrix;
use p3_maybe_rayon::prelude::current_num_threads;
use p3_merkle_tree::MerkleTreeMmcs;
use p3_symmetric::{PaddingFreeSponge, TruncatedPermutation};
use rand::rngs::SmallRng;
use rand::SeedableRng;

type F = KoalaBear;
type Perm = Poseidon2KoalaBear<16>;
type H = PaddingFreeSponge<Perm, 16, 8, 8>;
type C = TruncatedPermutation<Perm, 2, 8, 16>;
type ValMmcs = MerkleTreeMmcs<<F as Field>::Packing, <F as Field>::Packing, H, C, 2, 8>;

fn main() {
    let rows_log2 = argument("rows-log2").parse::<usize>().unwrap();
    let columns = argument("columns").parse::<usize>().unwrap();
    let log_blowup = argument("log-blowup").parse::<usize>().unwrap();
    let phase = argument("phase");
    assert!(matches!(
        phase.as_str(),
        "input" | "lde" | "merkle" | "commit"
    ));

    let rows = 1usize << rows_log2;
    let cells = rows.checked_mul(columns).unwrap();
    let input_bytes = cells.checked_mul(size_of::<F>()).unwrap();
    let values = deterministic_values(cells);
    let matrix = RowMajorMatrix::new(values, columns);

    if phase == "input" {
        print_result(
            &phase,
            rows,
            columns,
            input_bytes,
            log_blowup,
            0,
            "input-only",
        );
        black_box(matrix);
        return;
    }

    let dft = Radix2DitParallel::default();
    let mut rng = SmallRng::seed_from_u64(1);
    let permutation = Perm::new_from_rng_128(&mut rng);
    let hash = H::new(permutation.clone());
    let compress = C::new(permutation);
    let mmcs = ValMmcs::new(hash, compress, 0);

    if phase == "lde" {
        let start = Instant::now();
        let lde = make_lde(&dft, matrix, log_blowup);
        let elapsed = start.elapsed().as_nanos();
        print_result(
            &phase,
            rows,
            columns,
            input_bytes,
            log_blowup,
            elapsed,
            "lde-ready",
        );
        black_box(lde);
        return;
    }

    if phase == "merkle" {
        let lde = make_lde(&dft, matrix, log_blowup);
        let start = Instant::now();
        let (root, prover_data) = mmcs.commit(vec![lde]);
        let elapsed = start.elapsed().as_nanos();
        print_result(
            &phase,
            rows,
            columns,
            input_bytes,
            log_blowup,
            elapsed,
            &format!("{root:?}"),
        );
        black_box(prover_data);
        return;
    }

    let start = Instant::now();
    let lde = make_lde(&dft, matrix, log_blowup);
    let (root, prover_data) = mmcs.commit(vec![lde]);
    let elapsed = start.elapsed().as_nanos();
    print_result(
        &phase,
        rows,
        columns,
        input_bytes,
        log_blowup,
        elapsed,
        &format!("{root:?}"),
    );
    black_box(prover_data);
}

fn make_lde(
    dft: &Radix2DitParallel<F>,
    matrix: RowMajorMatrix<F>,
    log_blowup: usize,
) -> RowMajorMatrix<F> {
    dft.coset_lde_batch(matrix, log_blowup, F::GENERATOR)
        .bit_reverse_rows()
        .to_row_major_matrix()
}

fn deterministic_values(count: usize) -> Vec<F> {
    let mut values = Vec::with_capacity(count);
    let mut state = 1u64;
    for _ in 0..count {
        state ^= state >> 12;
        state ^= state << 25;
        state ^= state >> 27;
        state = state.wrapping_mul(2_685_821_657_736_338_717);
        values.push(F::from_u64(state));
    }
    values
}

fn argument(name: &str) -> String {
    let prefix = format!("--{name}=");
    env::args()
        .find_map(|arg| arg.strip_prefix(&prefix).map(str::to_owned))
        .unwrap_or_else(|| panic!("missing {prefix}<value>"))
}

fn print_result(
    phase: &str,
    rows: usize,
    columns: usize,
    input_bytes: usize,
    log_blowup: usize,
    elapsed_ns: u128,
    marker: &str,
) {
    println!(
        "{{\"implementation\":\"plonky3\",\"phase\":\"{phase}\",\
         \"rows\":{rows},\"columns\":{columns},\"input_bytes\":{input_bytes},\
         \"expanded_bytes\":{},\"elapsed_ns\":{elapsed_ns},\"threads\":{},\
         \"marker\":\"{marker}\"}}",
        input_bytes * (1 << log_blowup),
        current_num_threads(),
    );
}
