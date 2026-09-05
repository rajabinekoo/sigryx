{{- if ne .schema_name "public" }}
-- Move Sigryx application objects out of PostgreSQL's public schema.
--
-- The schema name is supplied by Atlas through atlas.hcl and is validated by
-- the Sigryx container entrypoint before Atlas runs. Existing installations
-- that already applied the previous public-schema migrations are upgraded in
-- place; new installations run the same historical migrations and immediately
-- move the resulting objects into the configured application schema.
CREATE SCHEMA IF NOT EXISTS "{{ .schema_name }}";

ALTER TABLE "public"."key_roots" SET SCHEMA "{{ .schema_name }}";
ALTER TABLE "public"."unseal_key_slots" SET SCHEMA "{{ .schema_name }}";
ALTER TABLE "public"."wallet_counters" SET SCHEMA "{{ .schema_name }}";
ALTER TABLE "public"."wallets" SET SCHEMA "{{ .schema_name }}";
ALTER TABLE "public"."roles" SET SCHEMA "{{ .schema_name }}";
ALTER TABLE "public"."service_accounts" SET SCHEMA "{{ .schema_name }}";
ALTER TABLE "public"."users" SET SCHEMA "{{ .schema_name }}";
ALTER TABLE "public"."sessions" SET SCHEMA "{{ .schema_name }}";
ALTER TABLE "public"."audit_events" SET SCHEMA "{{ .schema_name }}";
ALTER TABLE "public"."signing_records" SET SCHEMA "{{ .schema_name }}";

ALTER FUNCTION "public"."sigryx_reject_append_only_mutation"()
  SET SCHEMA "{{ .schema_name }}";
ALTER FUNCTION "public"."sigryx_purge_audit_events"(character varying, timestamptz, integer)
  SET SCHEMA "{{ .schema_name }}";

-- The original retention function contained explicit public.audit_events
-- references. Recreate it in the target schema so retention keeps working
-- after the tables are moved. The function search_path is pinned as defense in
-- depth and does not rely on the caller's connection search_path.
CREATE OR REPLACE FUNCTION "{{ .schema_name }}"."sigryx_purge_audit_events"(
  p_retention_class character varying,
  p_before timestamptz,
  p_limit integer
)
RETURNS integer
LANGUAGE plpgsql
SET search_path = "{{ .schema_name }}", pg_catalog
AS $$
DECLARE
  v_deleted integer := 0;
BEGIN
  IF p_retention_class IS NULL OR p_retention_class NOT IN ('NORMAL', 'CRITICAL') THEN
    RAISE EXCEPTION 'invalid audit retention class: %', p_retention_class
      USING ERRCODE = '22023';
  END IF;
  IF p_before IS NULL OR p_before > CURRENT_TIMESTAMP THEN
    RAISE EXCEPTION 'audit retention cutoff must not be in the future'
      USING ERRCODE = '22023';
  END IF;
  IF p_limit IS NULL OR p_limit < 1 OR p_limit > 50000 THEN
    RAISE EXCEPTION 'audit retention batch size must be between 1 and 50000'
      USING ERRCODE = '22023';
  END IF;

  PERFORM set_config('sigryx.audit_retention_purge', 'on', true);
  BEGIN
    WITH candidates AS (
      SELECT id
      FROM "{{ .schema_name }}"."audit_events"
      WHERE retention_class = p_retention_class
        AND occurred_at < p_before
      ORDER BY occurred_at ASC, id ASC
      LIMIT p_limit
      FOR UPDATE SKIP LOCKED
    ), deleted AS (
      DELETE FROM "{{ .schema_name }}"."audit_events" AS audit
      USING candidates
      WHERE audit.id = candidates.id
      RETURNING 1
    )
    SELECT count(*)::integer INTO v_deleted FROM deleted;
  EXCEPTION WHEN OTHERS THEN
    PERFORM set_config('sigryx.audit_retention_purge', 'off', true);
    RAISE;
  END;
  PERFORM set_config('sigryx.audit_retention_purge', 'off', true);

  RETURN v_deleted;
END;
$$;
{{- end }}
