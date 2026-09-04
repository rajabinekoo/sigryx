-- Create "audit_events" table.
CREATE TABLE "public"."audit_events" (
  "id" uuid NOT NULL,
  "occurred_at" timestamptz NOT NULL,
  "actor_type" character varying NULL,
  "actor_id" character varying NULL,
  "session_id" character varying NULL,
  "action" character varying NOT NULL,
  "outcome" character varying NOT NULL,
  "source_ip" character varying NULL,
  "request_id" character varying NULL,
  "method" character varying NULL,
  "path" character varying NULL,
  "status_code" bigint NOT NULL DEFAULT 0,
  "details" jsonb NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "audit_action_not_empty" CHECK (length(action) > 0),
  CONSTRAINT "audit_outcome_not_empty" CHECK (length(outcome) > 0)
);
CREATE INDEX "audit_events_occurred_at" ON "public"."audit_events" ("occurred_at");

-- Create "signing_records" table. Only INTEGRITY signing writes this table.
CREATE TABLE "public"."signing_records" (
  "id" uuid NOT NULL,
  "context" character varying NOT NULL,
  "object_id" character varying NOT NULL,
  "encrypted_record" bytea NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "signing_record_context_not_empty" CHECK (length(context) > 0),
  CONSTRAINT "signing_record_object_id_not_empty" CHECK (length(object_id) > 0),
  CONSTRAINT "signing_record_encrypted_not_empty" CHECK (octet_length(encrypted_record) > 0)
);
CREATE UNIQUE INDEX "signing_records_context_object_id_key" ON "public"."signing_records" ("context", "object_id");

-- Both tables are append-only from ordinary application/database roles.
CREATE OR REPLACE FUNCTION "public"."sigryx_reject_append_only_mutation"()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER "audit_events_append_only"
BEFORE UPDATE OR DELETE ON "public"."audit_events"
FOR EACH ROW EXECUTE FUNCTION "public"."sigryx_reject_append_only_mutation"();

CREATE TRIGGER "signing_records_append_only"
BEFORE UPDATE OR DELETE ON "public"."signing_records"
FOR EACH ROW EXECUTE FUNCTION "public"."sigryx_reject_append_only_mutation"();

CREATE TRIGGER "audit_events_no_truncate"
BEFORE TRUNCATE ON "public"."audit_events"
FOR EACH STATEMENT EXECUTE FUNCTION "public"."sigryx_reject_append_only_mutation"();

CREATE TRIGGER "signing_records_no_truncate"
BEFORE TRUNCATE ON "public"."signing_records"
FOR EACH STATEMENT EXECUTE FUNCTION "public"."sigryx_reject_append_only_mutation"();
