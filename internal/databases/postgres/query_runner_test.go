package postgres_test

import (
	"fmt"
	"testing"

	"github.com/wal-g/wal-g/internal/databases/postgres"

	"github.com/stretchr/testify/assert"
)

// Tests building start backup query
func TestBuildStartBackup(t *testing.T) {
	queryBuilder := &postgres.PgQueryRunner{Version: 0}
	_, err := queryBuilder.BuildStartBackup()
	assert.Error(t, err)

	queryBuilder.Version = 81000
	_, err = queryBuilder.BuildStartBackup()
	assert.IsType(t, err, postgres.UnsupportedPostgresVersionError{})

	queryBuilder.Version = 90321
	queryString, err := queryBuilder.BuildStartBackup()
	assert.Equal(t, "SELECT case when pg_is_in_recovery() then '' else (pg_xlogfile_name_offset(lsn)).file_name end, lsn::text, pg_is_in_recovery() FROM pg_start_backup($1, true) lsn", queryString)

	queryBuilder.Version = 90600
	queryString, err = queryBuilder.BuildStartBackup()
	assert.Equal(t, "SELECT case when pg_is_in_recovery() then '' else (pg_xlogfile_name_offset(lsn)).file_name end, lsn::text, pg_is_in_recovery() FROM pg_start_backup($1, true, false) lsn", queryString)

	queryBuilder.Version = 100000
	queryString, err = queryBuilder.BuildStartBackup()
	assert.Equal(t, "SELECT case when pg_is_in_recovery() then '' else (pg_walfile_name_offset(lsn)).file_name end, lsn::text, pg_is_in_recovery() FROM pg_start_backup($1, true, false) lsn", queryString)

	queryBuilder.Version = 150000
	queryString, err = queryBuilder.BuildStartBackup()
	assert.Equal(t, "SELECT case when pg_is_in_recovery() then '' else (pg_walfile_name_offset(lsn)).file_name end, lsn::text, pg_is_in_recovery() FROM pg_backup_start($1, true) lsn", queryString)
}

// Tests building stop backup query
func TestBuildStopBackup(t *testing.T) {
	queryBuilder := &postgres.PgQueryRunner{Version: 0}
	_, err := queryBuilder.BuildStopBackup()
	assert.Errorf(t, err, "BuildStopBackup did not error on version 0")

	queryBuilder.Version = 81000
	_, err = queryBuilder.BuildStopBackup()
	assert.IsType(t, err, postgres.UnsupportedPostgresVersionError{})

	queryBuilder.Version = 90321
	queryString, err := queryBuilder.BuildStopBackup()
	assert.Equal(t, "SELECT (pg_xlogfile_name_offset(lsn)).file_name, lpad((pg_xlogfile_name_offset(lsn)).file_offset::text, 8, '0') AS file_offset, lsn::text FROM pg_stop_backup() lsn", queryString)

	queryBuilder.Version = 90600
	queryString, err = queryBuilder.BuildStopBackup()
	assert.Equal(t, "SELECT labelfile, spcmapfile, lsn FROM pg_stop_backup(false)", queryString)

	queryBuilder.Version = 100000
	queryString, err = queryBuilder.BuildStopBackup()
	assert.Equal(t, "SELECT labelfile, spcmapfile, lsn FROM pg_stop_backup(false)", queryString)

	queryBuilder.Version = 150000
	queryString, err = queryBuilder.BuildStopBackup()
	assert.Equal(t, "SELECT labelfile, spcmapfile, lsn FROM pg_backup_stop(false)", queryString)
}

func TestIsTablespaceMapExists(t *testing.T) {
	testCases := []struct {
		version  int
		expected bool
	}{
		{0, false},
		{90600, true},
		{int((^uint(0)) >> 1), true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("TestIsTablespaceMapExists_PgVersion_%d", tc.version), func(t *testing.T) {
			queryBuilder := &postgres.PgQueryRunner{Version: tc.version}
			isTablespaceMapExists := queryBuilder.IsTablespaceMapExists()
			assert.Equal(t, isTablespaceMapExists, tc.expected)
		})
	}
}
