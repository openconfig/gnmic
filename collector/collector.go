package collector

type Collector struct {
	Targets       map[string]*Target
	Subscriptions map[string]*Subscription
}
