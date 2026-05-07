package live

import (
	"testing"
	"time"

	"hvc/internal/config"
	"hvc/pkg/idgen"
)

func TestChannelService_CreateAndPlayback(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 1, 1)
	service := NewChannelService(config.DynamicRuntimeConfig{})
	channel := service.CreateChannel("room_1001", "test room", 1)
	if channel.ChannelID == 0 {
		t.Fatalf("channel id should not be zero")
	}
	playback := service.StartChannel(channel.ChannelKey)
	if playback.ChannelKey != channel.ChannelKey {
		t.Fatalf("playback channel key mismatch")
	}
	if playback.Status != "RUNNING" {
		t.Fatalf("playback status should be RUNNING")
	}
	stopped := service.StopChannel(channel.ChannelKey)
	if stopped.Status != "STOPPED" {
		t.Fatalf("playback status should be STOPPED")
	}
}
