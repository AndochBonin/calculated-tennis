package models

// SubscriptionTopic represents the category a subscription is for.
type SubscriptionTopic string

const (
	SubscriptionTopicPrice  SubscriptionTopic = "price"
	SubscriptionTopicSports SubscriptionTopic = "sports"
)

// SubscriptionStatus represents the current state of a WebSocket subscription.
type SubscriptionStatus string

const (
	SubscriptionStatusActive SubscriptionStatus = "active"
	SubscriptionStatusClosed SubscriptionStatus = "closed"
	SubscriptionStatusError  SubscriptionStatus = "error"
)

// Subscription tracks an active WebSocket subscription for a topic.
type Subscription struct {
	Topic    SubscriptionTopic
	TokenIDs []string
	Status   SubscriptionStatus
	Error    error
}
