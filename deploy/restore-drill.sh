#!/bin/sh
# Run from a postgres client container with /ops mounted read-only.
set -eu
umask 077
work=$(mktemp -d)
scratch="pulse_drill_$(psql -XAt -d pulse -c "select replace(gen_random_uuid()::text,'-','')")"
created=false
cleanup() {
  if [ "$created" = true ]; then dropdb "$scratch"; fi
  rm -f "$work/backup.dump"
  rmdir "$work"
}
trap cleanup EXIT
pg_dump -Fc -d pulse -f "$work/backup.dump"
createdb "$scratch"
created=true
pg_restore --exit-on-error --no-owner -d "$scratch" "$work/backup.dump"
test "$(psql -XAt -d "$scratch" -c 'select version from schema_migrations where not dirty')" = 5
email="$scratch@example.test"
psql -X -v ON_ERROR_STOP=1 -d "$scratch" -c "insert into users(email,display_name,password_hash) values ('$email','Restore drill','disabled')"
for attempt in 1 2; do
  psql -X -d "$scratch" -v owner_email="$email" -f /ops/demo-seed.sql
done
test "$(psql -XAt -d "$scratch" -c "select count(*) from monitor_checks c join monitors m on m.id=c.monitor_id join users u on u.id=m.user_id where u.email='$email'")" = 8
echo "Restore, schema version and repeatable demo seed verified."
