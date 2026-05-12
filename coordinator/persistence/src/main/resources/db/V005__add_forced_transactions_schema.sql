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
COMMENT ON COLUMN forced_transactions.inclusion_result IS 'Result of forced transaction inclusion attempt (1=Included, 2=BadNonce, 3=BadBalance, 4=BadPrecompile, 5=TooManyLogs, 6=FilteredAddressFrom, 7=FilteredAddressTo, 8=Phylax)';
COMMENT ON COLUMN forced_transactions.simulated_execution_block_number IS 'Block number where FTX was simulated for execution';
COMMENT ON COLUMN forced_transactions.simulated_execution_block_timestamp IS 'Timestamp (epoch millis) of simulated execution block';
COMMENT ON COLUMN forced_transactions.ftx_block_number_deadline IS 'Block number deadline for the forced transaction';
COMMENT ON COLUMN forced_transactions.ftx_rolling_hash IS 'Rolling hash of the forced transaction';
COMMENT ON COLUMN forced_transactions.ftx_rlp IS 'RLP-encoded forced transaction data';
COMMENT ON COLUMN forced_transactions.proof_status IS 'Status of the invalidity proof (1=UNREQUESTED, 2=REQUESTED, 3=PROVEN)';
