-- Create forced_transactions table
-- Stores forced transaction execution status and invalidity proof tracking

CREATE TABLE forced_transactions (
    created_epoch_milli BIGINT NOT NULL,
    updated_epoch_milli BIGINT NOT NULL,
    ftx_number BIGINT PRIMARY KEY,
    inclusion_result SMALLINT NOT NULL,
    simulated_execution_block_number BIGINT NOT NULL,
    simulated_execution_block_timestamp BIGINT NOT NULL,
    ftx_block_number_deadline BIGINT NOT NULL,
    ftx_rolling_hash BYTEA NOT NULL,
    ftx_rlp BYTEA NOT NULL,
    proof_status SMALLINT NOT NULL
);

-- Create indexes for efficient querying
CREATE INDEX idx_forced_transactions_simulated_block ON forced_transactions(simulated_execution_block_number);
CREATE INDEX idx_forced_transactions_proof_status ON forced_transactions(proof_status);
CREATE INDEX idx_forced_transactions_inclusion_result ON forced_transactions(inclusion_result);

-- Add comments
COMMENT ON TABLE forced_transactions IS 'Stores forced transaction execution status and invalidity proof tracking';
COMMENT ON COLUMN forced_transactions.created_epoch_milli IS 'Timestamp (epoch millis) when record was created';
COMMENT ON COLUMN forced_transactions.updated_epoch_milli IS 'Timestamp (epoch millis) when record was last updated';
COMMENT ON COLUMN forced_transactions.ftx_number IS 'Forced transaction number (unique identifier)';
COMMENT ON COLUMN forced_transactions.inclusion_result IS 'Result of forced transaction inclusion attempt (1=Included, 2=BadNonce, 3=BadBalance, 4=BadPrecompile, 5=TooManyLogs, 6=FilteredAddressFrom, 7=FilteredAddressTo, 8=ChainSecurityRuleViolation)';
COMMENT ON COLUMN forced_transactions.simulated_execution_block_number IS 'Block number where FTX was simulated for execution';
COMMENT ON COLUMN forced_transactions.simulated_execution_block_timestamp IS 'Timestamp (epoch millis) of simulated execution block';
COMMENT ON COLUMN forced_transactions.ftx_block_number_deadline IS 'Block number deadline for the forced transaction';
COMMENT ON COLUMN forced_transactions.ftx_rolling_hash IS 'Rolling hash of the forced transaction';
COMMENT ON COLUMN forced_transactions.ftx_rlp IS 'RLP-encoded forced transaction data';
COMMENT ON COLUMN forced_transactions.proof_status IS 'Status of the invalidity proof (1=UNREQUESTED, 2=REQUESTED, 3=PROVEN)';

-- Add proof index hash to batches table
ALTER TABLE batches ADD COLUMN IF NOT EXISTS proof_index_hash varchar;

-- RISC-V rollup proofs and their blobs. Adjacent rollups can share a blob: the previous rollup's last blob is the
-- next rollup's first. Each blob is stored once in riscv_blobs, and riscv_rollup_blobs lists each rollup's blobs.
-- "parent" refers to the previous row of the same table: the previous rollup in riscv_rollups, the previous blob in
-- riscv_blobs.
CREATE TABLE riscv_rollups (
    start_block_number BIGINT NOT NULL,
    end_block_number BIGINT NOT NULL,
    created_epoch_milli BIGINT NOT NULL,
    start_block_timestamp BIGINT NOT NULL,
    end_block_timestamp BIGINT NOT NULL,
    batches_count INTEGER NOT NULL,
    proof_hash BYTEA NOT NULL,
    parent_data_rolling_hash BYTEA NOT NULL,
    data_rolling_hash BYTEA NOT NULL,
    parent_shared_blob_carry_over_tail_bytes INTEGER NOT NULL,
    parent_blob_data_rolling_hash BYTEA,
    final_shared_blob_carry_over_tail_bytes INTEGER NOT NULL,
    PRIMARY KEY (start_block_number, end_block_number),
    -- Rollups never overlap: no two share a start block or an end block.
    CONSTRAINT riscv_rollups_start_block_number_key UNIQUE (start_block_number),
    CONSTRAINT riscv_rollups_end_block_number_key UNIQUE (end_block_number)
);

COMMENT ON TABLE riscv_rollups IS 'RISC-V rollup proofs, one row per proven rollup';
COMMENT ON COLUMN riscv_rollups.start_block_number IS 'First block of the rollup';
COMMENT ON COLUMN riscv_rollups.end_block_number IS 'Last block of the rollup';
COMMENT ON COLUMN riscv_rollups.created_epoch_milli IS 'Timestamp (epoch millis) when the row was created';
COMMENT ON COLUMN riscv_rollups.start_block_timestamp IS 'Timestamp (epoch millis) of the first block';
COMMENT ON COLUMN riscv_rollups.end_block_timestamp IS 'Timestamp (epoch millis) of the last block';
COMMENT ON COLUMN riscv_rollups.batches_count IS 'Number of conflations in the rollup';
COMMENT ON COLUMN riscv_rollups.proof_hash IS 'Hash identifying the rollup proof request and response';
COMMENT ON COLUMN riscv_rollups.parent_data_rolling_hash IS 'Data rolling hash at the end of the previous rollup';
COMMENT ON COLUMN riscv_rollups.data_rolling_hash IS 'Data rolling hash after the rollup''s last blob';
COMMENT ON COLUMN riscv_rollups.parent_shared_blob_carry_over_tail_bytes IS
    'Payload bytes at the end of the first (shared) blob that the previous rollup carried over to this one; 0 when the rollup starts on a fresh blob';
COMMENT ON COLUMN riscv_rollups.parent_blob_data_rolling_hash IS
    'Data rolling hash before the shared first blob; set exactly when parent_shared_blob_carry_over_tail_bytes > 0';
COMMENT ON COLUMN riscv_rollups.final_shared_blob_carry_over_tail_bytes IS
    'Payload bytes at the end of the last blob that this rollup carries over to the next one';

CREATE TABLE riscv_blobs (
    data_rolling_hash BYTEA PRIMARY KEY,
    parent_data_rolling_hash BYTEA NOT NULL,
    chunk_hash BYTEA NOT NULL,
    blob_bytes BYTEA NOT NULL,
    created_epoch_milli BIGINT NOT NULL
);

COMMENT ON TABLE riscv_blobs IS 'RISC-V blobs, one row per blob, shared by the rollups that contain it';
COMMENT ON COLUMN riscv_blobs.data_rolling_hash IS 'Data rolling hash after this blob; unique per blob in the stream';
COMMENT ON COLUMN riscv_blobs.parent_data_rolling_hash IS 'Data rolling hash after the previous blob, i.e. before this one';
COMMENT ON COLUMN riscv_blobs.chunk_hash IS 'KZG versioned hash of the blob';
COMMENT ON COLUMN riscv_blobs.blob_bytes IS 'Packed blob bytes';
COMMENT ON COLUMN riscv_blobs.created_epoch_milli IS 'Timestamp (epoch millis) when the blob was first stored';

CREATE TABLE riscv_rollup_blobs (
    start_block_number BIGINT NOT NULL,
    end_block_number BIGINT NOT NULL,
    blob_index INTEGER NOT NULL,
    data_rolling_hash BYTEA NOT NULL REFERENCES riscv_blobs (data_rolling_hash),
    PRIMARY KEY (start_block_number, end_block_number, blob_index),
    FOREIGN KEY (start_block_number, end_block_number)
        REFERENCES riscv_rollups (start_block_number, end_block_number) ON DELETE CASCADE
);

CREATE INDEX riscv_rollup_blobs_data_rolling_hash_idx ON riscv_rollup_blobs (data_rolling_hash);

COMMENT ON TABLE riscv_rollup_blobs IS 'Blobs of each RISC-V rollup, in stream order';
COMMENT ON COLUMN riscv_rollup_blobs.start_block_number IS 'First block of the rollup in riscv_rollups';
COMMENT ON COLUMN riscv_rollup_blobs.end_block_number IS 'Last block of the rollup in riscv_rollups';
COMMENT ON COLUMN riscv_rollup_blobs.blob_index IS 'Position of the blob in the rollup, from 0';
COMMENT ON COLUMN riscv_rollup_blobs.data_rolling_hash IS 'The blob in riscv_blobs';
