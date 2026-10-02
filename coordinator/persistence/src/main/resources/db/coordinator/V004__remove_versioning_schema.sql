-- =======================================================
-- batches table
-- =======================================================
ALTER TABLE if exists batches DROP constraint batches_pkey;

ALTER TABLE if exists batches
ADD PRIMARY KEY (start_block_number, end_block_number);

ALTER TABLE if exists batches
  DROP COLUMN conflation_calculator_version,
  DROP COLUMN prover_version;

-- =======================================================
-- blobs table
-- =======================================================
ALTER TABLE if exists blobs DROP constraint blobs_pkey;

ALTER TABLE if exists blobs
ADD PRIMARY KEY (start_block_number, end_block_number);

ALTER TABLE if exists blobs
  DROP COLUMN conflation_calculator_version;

-- =======================================================
-- aggregations table
-- =======================================================
ALTER TABLE if exists aggregations DROP constraint aggregations_pkey;

ALTER TABLE if exists aggregations
ADD PRIMARY KEY (start_block_number, end_block_number, batch_count);

ALTER TABLE if exists aggregations
  DROP COLUMN aggregation_calculator_version;
