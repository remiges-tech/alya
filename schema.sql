CREATE TABLE voucher (
  id   BIGSERIAL PRIMARY KEY,
  date date NOT NULL,
  debit_account_id BIGINT,
  credit_account_id BIGINT,
  cost_centre_id BIGINT,
  amount Decimal,
  narration TEXT
);