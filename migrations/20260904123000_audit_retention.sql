-- Add explicit retention classes so security-sensitive audit events can be kept
-- longer than routine operational events.
ALTER TABLE "public"."audit_events"
  ADD COLUMN "retention_class" character varying NOT NULL DEFAULT 'NORMAL',
  ADD CONSTRAINT "audit_retention_class_valid"
    CHECK (retention_class IN ('NORMAL', 'CRITICAL'));

-- Preserve the intended classification for audit rows created before this
-- migration. The table is otherwise append-only, so temporarily disabling the
-- mutation trigger is limited to this migration-only backfill.
ALTER TABLE "public"."audit_events" DISABLE TRIGGER "audit_events_append_only";
UPDATE "public"."audit_events"
SET "retention_class" = 'CRITICAL'
WHERE action = 'audit.retention_cleanup'
   OR action LIKE 'security.%'
   OR action LIKE 'sign.%'
   OR action LIKE 'recovery.%'
   OR (action LIKE 'vault.%' AND action <> 'vault.status')
   OR (action LIKE 'keyroot.%' AND action <> 'keyroot.list')
   OR (action LIKE 'access.%' AND action NOT LIKE '%.list')
   OR action IN ('auth.setup', 'auth.service_token', 'auth.update_me', 'wallet.create');
ALTER TABLE "public"."audit_events" ENABLE TRIGGER "audit_events_append_only";

-- Supports small ordered retention batches without scanning the whole table.
CREATE INDEX "audit_events_retention_cleanup"
  ON "public"."audit_events" ("retention_class", "occurred_at", "id");

-- Ordinary UPDATE/DELETE/TRUNCATE remain rejected. DELETE is permitted only
-- while the dedicated retention function is executing.
CREATE OR REPLACE FUNCTION "public"."sigryx_reject_append_only_mutation"()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_TABLE_NAME = 'audit_events'
     AND TG_OP = 'DELETE'
     AND current_setting('sigryx.audit_retention_purge', true) = 'on' THEN
    RETURN OLD;
  END IF;

  RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = '55000';
END;
$$;

-- Sanctioned, batch-bounded purge path used by the Sigryx retention worker.
-- The local guard is turned off again before returning, even when the delete
-- fails, so later statements in the same transaction cannot silently mutate
-- the append-only audit table.
CREATE OR REPLACE FUNCTION "public"."sigryx_purge_audit_events"(
  p_retention_class character varying,
  p_before timestamptz,
  p_limit integer
)
RETURNS integer
LANGUAGE plpgsql
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
      FROM "public"."audit_events"
      WHERE retention_class = p_retention_class
        AND occurred_at < p_before
      ORDER BY occurred_at ASC, id ASC
      LIMIT p_limit
      FOR UPDATE SKIP LOCKED
    ), deleted AS (
      DELETE FROM "public"."audit_events" AS audit
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
