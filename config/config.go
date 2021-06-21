package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"os"
	"strconv"
)

type Config struct {
	Port                         int64
	OauthConf                    *oauth2.Config
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
	NotificationBotToken         string
	NotificationChannelID        string
	CurationFeedChannelID        string
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

func GetConfig(l *logrus.Logger) *Config {
	l.Infoln("loading config...")
	err := godotenv.Load()
	if err != nil {
		l.Fatal(err)
	}

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
		NotificationBotToken:         EnvString("NOTIFICATION_BOT_TOKEN"),
		NotificationChannelID:        EnvString("NOTIFICATION_CHANNEL_ID"),
		CurationFeedChannelID:        EnvString("CURATION_FEED_CHANNEL_ID"),
	}
}
