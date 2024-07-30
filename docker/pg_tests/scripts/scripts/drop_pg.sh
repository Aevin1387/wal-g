#!/bin/bash
/usr/lib/postgresql/15/bin/pg_ctl -D ${PGDATA} -w stop
pkill -9 postgres || true
pkill -9 wal-g || true
rm -rf $PGDATA /tmp/basebackups_005 /tmp/wal_005 /tmp/spaces /tmp/spaces_backup
