package notificationbot

type DiscordNotificationSender interface {
	SendNotification(msg string) error
}
