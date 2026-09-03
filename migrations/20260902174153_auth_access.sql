-- Create "roles" table
CREATE TABLE "public"."roles" (
  "id" uuid NOT NULL,
  "name" character varying NOT NULL,
  "permissions" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "roles_name_key" to table: "roles"
CREATE UNIQUE INDEX "roles_name_key" ON "public"."roles" ("name");
-- Create "service_accounts" table
CREATE TABLE "public"."service_accounts" (
  "id" uuid NOT NULL,
  "name" character varying NOT NULL,
  "client_id" character varying NOT NULL,
  "client_secret_hash" bytea NOT NULL,
  "active" boolean NOT NULL DEFAULT true,
  "allowed_cidrs" jsonb NOT NULL,
  "role_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "service_accounts_roles_service_accounts" FOREIGN KEY ("role_id") REFERENCES "public"."roles" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "service_account_secret_hash_size" CHECK (octet_length(client_secret_hash) = 32)
);
-- Create index "service_accounts_client_id_key" to table: "service_accounts"
CREATE UNIQUE INDEX "service_accounts_client_id_key" ON "public"."service_accounts" ("client_id");
-- Create index "service_accounts_name_key" to table: "service_accounts"
CREATE UNIQUE INDEX "service_accounts_name_key" ON "public"."service_accounts" ("name");
-- Create "users" table
CREATE TABLE "public"."users" (
  "id" uuid NOT NULL,
  "username" character varying NOT NULL,
  "password_hash" character varying NOT NULL,
  "is_root_admin" boolean NOT NULL DEFAULT false,
  "active" boolean NOT NULL DEFAULT true,
  "allowed_cidrs" jsonb NOT NULL,
  "role_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "users_roles_users" FOREIGN KEY ("role_id") REFERENCES "public"."roles" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "user_root_role_consistency" CHECK (((is_root_admin = true) AND (role_id IS NULL)) OR ((is_root_admin = false) AND (role_id IS NOT NULL)))
);
-- Create index "user_is_root_admin" to table: "users"
CREATE UNIQUE INDEX "user_is_root_admin" ON "public"."users" ("is_root_admin") WHERE (is_root_admin = true);
-- Create index "users_username_key" to table: "users"
CREATE UNIQUE INDEX "users_username_key" ON "public"."users" ("username");
-- Create "sessions" table
CREATE TABLE "public"."sessions" (
  "id" uuid NOT NULL,
  "refresh_token_hash" bytea NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "revoked_at" timestamptz NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "sessions_users_sessions" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "session_refresh_token_hash_size" CHECK (octet_length(refresh_token_hash) = 32)
);
-- Create index "session_refresh_token_hash" to table: "sessions"
CREATE UNIQUE INDEX "session_refresh_token_hash" ON "public"."sessions" ("refresh_token_hash");
-- Create index "session_user_id" to table: "sessions"
CREATE INDEX "session_user_id" ON "public"."sessions" ("user_id");
