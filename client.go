package checkpoint

import (
	"time"
)

// A simple interface for interacting with the checkpoint server, for reporting and version checking
type UsageClient interface {
	Start(name, version string)
}

var _ UsageClient = NewUsageClient()

func NewUsageClient() *usageClient {
	return &usageClient{}
}

type usageClient struct {
}

func (c *usageClient) Start(name, version string) {
	now := time.Now()
	// starts the background check process
	callCheck(name, version, now)

	// Do an immediate check and report within the next 30 seconds
	go func() {
		callReport(name, version, now)
		callCheckOnceNow(name, version)
	}()

}
