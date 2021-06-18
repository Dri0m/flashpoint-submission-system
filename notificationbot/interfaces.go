package notificationbot

type DiscordNotificationSender interface {
	SendMessage(msg string) error
}
