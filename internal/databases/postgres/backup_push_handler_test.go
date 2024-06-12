package postgres_test

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/wal-g/tracelog"

	"github.com/wal-g/wal-g/internal"
	conf "github.com/wal-g/wal-g/internal/config"
	"github.com/wal-g/wal-g/internal/databases/postgres"
	"github.com/wal-g/wal-g/testtools"
	"github.com/wal-g/wal-g/utility"
)

const (
	deltaFromUserDataFlag    = "delta-from-user-data"
	deltaFromNameFlag        = "delta-from-name"
	withoutFilesMetadataFlag = "without-files-metadata"
)

var (
	permanent             = false
	fullBackup            = false
	verifyPageChecksums   = false
	storeAllCorruptBlocks = false
	useRatingComposer     = false
	useCopyComposer       = false
	deltaFromName         = ""
	deltaFromUserData     = ""
	userDataRaw           = ""
	withoutFilesMetadata  = false
)

func TestBackupHandler_HandleBackupPush(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bh := initCommand()
			bh.HandleBackupPush(context.TODO())
		})
	}
}

func chooseTarBallComposer() postgres.TarBallComposerType {
	tarBallComposerType := postgres.RegularComposer

	useRatingComposer = useRatingComposer || viper.GetBool(conf.UseRatingComposerSetting)
	if useRatingComposer {
		tarBallComposerType = postgres.RatingComposer
	}
	useCopyComposer = useCopyComposer || viper.GetBool(conf.UseCopyComposerSetting)
	if useCopyComposer {
		fullBackup = true
		tarBallComposerType = postgres.CopyComposer
	}

	return tarBallComposerType
}

func initCommand() *postgres.BackupHandler {
	internal.ConfigureLimiters()

	var dataDirectory string

	verifyPageChecksums = verifyPageChecksums || viper.GetBool(conf.VerifyPageChecksumsSetting)
	storeAllCorruptBlocks = storeAllCorruptBlocks || viper.GetBool(conf.StoreAllCorruptBlocksSetting)

	tarBallComposerType := chooseTarBallComposer()

	if deltaFromName == "" {
		deltaFromName = viper.GetString(conf.DeltaFromNameSetting)
	}
	if deltaFromUserData == "" {
		deltaFromUserData = viper.GetString(conf.DeltaFromUserDataSetting)
	}
	if userDataRaw == "" {
		userDataRaw = viper.GetString(conf.SentinelUserDataSetting)
	}
	withoutFilesMetadata = withoutFilesMetadata || viper.GetBool(conf.WithoutFilesMetadataSetting)
	if withoutFilesMetadata {
		// files metadata tracking is required for delta backups and copy/rating composers
		if tarBallComposerType != postgres.RegularComposer {
			tracelog.ErrorLogger.Fatalf(
				"%s option cannot be used with non-regular tar ball composer",
				withoutFilesMetadataFlag)
		}
		if deltaFromName != "" || deltaFromUserData != "" {
			tracelog.ErrorLogger.Fatalf(
				"%s option cannot be used with %s, %s options",
				withoutFilesMetadataFlag, deltaFromNameFlag, deltaFromUserDataFlag)
		}
		tracelog.InfoLogger.Print("Files metadata tracking is disabled")
		fullBackup = true
	}

	deltaBaseSelector, err := internal.NewDeltaBaseSelector(
		deltaFromName, deltaFromUserData, postgres.NewGenericMetaFetcher())
	tracelog.ErrorLogger.FatalOnError(err)

	userData, err := internal.UnmarshalSentinelUserData(userDataRaw)
	tracelog.ErrorLogger.FatalfOnError("Failed to unmarshal the provided UserData: %s", err)

	// storage, err := internal.ConfigureStorage()
	// tracelog.ErrorLogger.FatalOnError(err)

	folder := testtools.MakeDefaultInMemoryStorageFolder()
	// baseBackupFolder := folder.GetSubFolder(utility.BaseBackupPath)

	uploader, err := internal.ConfigureUploaderToFolder(folder)
	tracelog.ErrorLogger.FatalOnError(err)

	arguments := postgres.NewBackupArguments(uploader, dataDirectory, utility.BaseBackupPath,
		permanent, verifyPageChecksums || viper.GetBool(conf.VerifyPageChecksumsSetting),
		fullBackup, storeAllCorruptBlocks || viper.GetBool(conf.StoreAllCorruptBlocksSetting),
		tarBallComposerType, postgres.NewRegularDeltaBackupConfigurator(deltaBaseSelector),
		userData, withoutFilesMetadata)

	backupHandler, err := postgres.NewBackupHandler(arguments)
	tracelog.ErrorLogger.FatalOnError(err)
	return backupHandler
}
