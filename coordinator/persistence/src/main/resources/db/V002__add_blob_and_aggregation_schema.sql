-- =======================================================
-- batches table
-- =======================================================
alter table if exists batches
ADD COLUMN conflation_calculator_version varchar(256) NOT NULL DEFAULT '1.0.0';

alter table if exists batches DROP constraint batches_pkey;

ALTER TABLE if exists batches
ADD PRIMARY KEY (start_block_number, end_block_number, prover_version, conflation_calculator_version);
-- =======================================================
-- blobs table
-- =======================================================
create table if not exists blobs (
    created_epoch_milli            bigint,
    start_block_number             bigint,
    end_block_number               bigint,
    conflation_calculator_version  varchar(256),
    blob_hash                      varchar(256),
    status                         smallint,
    start_block_timestamp          bigint,
    end_block_timestamp            bigint,
    batches_count                  integer,
    expected_shnarf                varchar(256),
    blob_compression_proof         jsonb,
    primary key (start_block_number, end_block_number, conflation_calculator_version)
);
create index blob_start_block_number_idx ON blobs using btree (start_block_number);
-- =======================================================
-- aggregations table
-- =======================================================
create table if not exists aggregations (
    start_block_number              bigint,
    end_block_number                bigint,
    status                          bigint,
    aggregation_calculator_version  varchar(256),
    start_block_timestamp           bigint, -- useful for submitter to implement submission by deadline
    batch_count                     bigint,
    aggregation_proof               jsonb,
    primary key (start_block_number, end_block_number, batch_count, aggregation_calculator_version)
);
create index aggregations_start_block_number_idx ON aggregations using btree (start_block_number);
