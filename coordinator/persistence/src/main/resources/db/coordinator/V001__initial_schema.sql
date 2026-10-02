-- =======================================================
-- batches table
-- =======================================================
create table if not exists batches (
    created_epoch_milli          bigint,
    start_block_number           bigint,
    end_block_number             bigint,
    prover_version               varchar(256),
    status                       smallint,
    primary key (start_block_number, end_block_number, prover_version)
);
create index start_block_number_idx ON batches using btree (start_block_number);
