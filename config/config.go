package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type Config struct {
	Port                         int64
	OauthConf                    *oauth2.Config
	DeviceFlowVerificaitonUrl    string
	AuthBotToken                 string
	FlashpointServerID           string
	SecurecookieHashKeyPrevious  string
	SecurecookieBlockKeyPrevious string
	SecurecookieHashKeyCurrent   string
	SecurecookieBlockKeyCurrent  string
	SessionExpirationSeconds     int64
	ValidatorServerURL           string
	DBRootUser                   string
	DBRootPassword               string
	DBUser                       string
	DBPassword                   string
	DBIP                         string
	DBPort                       int64
	DBName                       string
	PostgresUser                 string
	PostgresPassword             string
	PostgresHost                 string
	PostgresPort                 int64
	NotificationBotToken         string
	NotificationChannelID        string
	CurationFeedChannelID        string
	IsDev                        bool
	ResumableUploadDirFullPath   string
	FlashfreezeDirFullPath       string
	ArchiveIndexerServerURL      string
	FlashfreezeIngestDirFullPath string
	FixesDirFullPath             string
	SubmissionsDirFullPath       string
	SubmissionImagesDirFullPath  string
	SystemUid                    int64
	ImagesCdn                    string
	ImagesCdnCompressed          bool
	MinLauncherVersion           string
	DataPacksDir                 string
	ImagesDir                    string
	DeletedDataPacksDir          string
	DeletedImagesDir             string
}

func EnvString(name string) string {
	s := os.Getenv(name)
	if s == "" {
		panic(fmt.Sprintf("env variable '%s' is not set", name))
	}
	return s
}

func EnvInt(name string) int64 {
	s := os.Getenv(name)
	if s == "" {
		panic(fmt.Sprintf("env variable '%s' is not set", name))
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic(err)
	}
	return i
}

func EnvBool(name string) bool {
	s := os.Getenv(name)
	if s == "" {
		panic(fmt.Sprintf("env variable '%s' is not set", name))
	} else if s == "True" {
		return true
	} else if s == "False" {
		return false
	}
	panic(fmt.Sprintf("invalid value of env variable '%s'", name))
}

func GetConfig(l *logrus.Entry) *Config {
	const ScopeIdentify = "identify"

	return &Config{
		Port: EnvInt("PORT"),
		OauthConf: &oauth2.Config{
			RedirectURL:  EnvString("OAUTH_REDIRECT_URL"),
			ClientID:     EnvString("OAUTH_CLIENT_ID"),
			ClientSecret: EnvString("OAUTH_CLIENT_SECRET"),
			Scopes:       []string{ScopeIdentify},
			Endpoint: oauth2.Endpoint{
				AuthURL:   "https://discordapp.com/api/oauth2/authorize",
				TokenURL:  "https://discordapp.com/api/oauth2/token",
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
		DeviceFlowVerificaitonUrl:    EnvString("DEVICE_FLOW_VERIFICATION_URL"),
		AuthBotToken:                 EnvString("AUTH_BOT_TOKEN"),
		FlashpointServerID:           EnvString("FLASHPOINT_SERVER_ID"),
		SecurecookieHashKeyPrevious:  EnvString("SECURECOOKIE_HASH_KEY_PREVIOUS"),
		SecurecookieBlockKeyPrevious: EnvString("SECURECOOKIE_BLOCK_KEY_PREVIOUS"),
		SecurecookieHashKeyCurrent:   EnvString("SECURECOOKIE_HASH_KEY_CURRENT"),
		SecurecookieBlockKeyCurrent:  EnvString("SECURECOOKIE_BLOCK_KEY_CURRENT"),
		SessionExpirationSeconds:     EnvInt("SESSION_EXPIRATION_SECONDS"),
		ValidatorServerURL:           EnvString("VALIDATOR_SERVER_URL"),
		DBUser:                       EnvString("DB_USER"),
		DBPassword:                   EnvString("DB_PASSWORD"),
		DBIP:                         EnvString("DB_IP"),
		DBPort:                       EnvInt("DB_PORT"),
		DBName:                       EnvString("DB_NAME"),
		PostgresUser:                 EnvString("POSTGRES_USER"),
		PostgresPassword:             EnvString("POSTGRES_PASSWORD"),
		PostgresHost:                 EnvString("POSTGRES_HOST"),
		PostgresPort:                 EnvInt("POSTGRES_PORT"),
		NotificationBotToken:         EnvString("NOTIFICATION_BOT_TOKEN"),
		NotificationChannelID:        EnvString("NOTIFICATION_CHANNEL_ID"),
		CurationFeedChannelID:        EnvString("CURATION_FEED_CHANNEL_ID"),
		IsDev:                        EnvBool("IS_DEV"),
		ResumableUploadDirFullPath:   EnvString("RESUMABLE_UPLOAD_DIR_FULL_PATH"),
		FlashfreezeDirFullPath:       EnvString("FLASHFREEZE_DIR_FULL_PATH"),
		ArchiveIndexerServerURL:      EnvString("ARCHIVE_INDEXER_SERVER_URL"),
		FlashfreezeIngestDirFullPath: EnvString("FLASHFREEZE_INGEST_DIR_FULL_PATH"),
		FixesDirFullPath:             EnvString("FIXES_DIR_FULL_PATH"),
		SubmissionsDirFullPath:       EnvString("SUBMISSIONS_DIR_FULL_PATH"),
		SubmissionImagesDirFullPath:  EnvString("SUBMISSION_IMAGES_DIR_FULL_PATH"),
		SystemUid:                    EnvInt("SYSTEM_UID"),
		ImagesCdn:                    EnvString("IMAGES_CDN"),
		ImagesCdnCompressed:          EnvBool("IMAGES_CDN_COMPRESSED"),
		MinLauncherVersion:           EnvString("MIN_LAUNCHER_VERSION"),
		DataPacksDir:                 EnvString("DATA_PACKS_PATH"),
		ImagesDir:                    EnvString("IMAGES_PATH"),
		DeletedDataPacksDir:          EnvString("DELETED_DATA_PACKS_PATH"),
		DeletedImagesDir:             EnvString("DELETED_IMAGES_PATH"),
	}
}
