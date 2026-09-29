-- =======================================================
-- feehistories table
-- =======================================================
create table if not exists feehistories (
    created_epoch_milli            bigint,
    block_number                   bigint,
    base_fee_per_gas               bigint,
    base_fee_per_blob_gas          bigint,
    gas_used_ratio                 float,
    blob_gas_used_ratio            float,
    rewards                        bigint[],
    reward_percentiles             float[],
    primary key (block_number)
);
create index fee_history_block_number_idx ON feehistories using btree (block_number);
