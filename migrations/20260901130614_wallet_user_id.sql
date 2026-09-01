-- Modify "wallets" table
ALTER TABLE "public"."wallets" ADD COLUMN "user_id" character varying NOT NULL;
-- Create index "wallet_key_root_id_adapter_user_id" to table: "wallets"
CREATE UNIQUE INDEX "wallet_key_root_id_adapter_user_id" ON "public"."wallets" ("key_root_id", "adapter", "user_id");
