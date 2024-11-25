-- =======================================================
-- rejected_transactions table
-- =======================================================
alter table if exists rejected_transactions
  alter column reject_reason type varchar(1024);
create index if not exists tx_hash_idx on rejected_transactions using btree (tx_hash);
