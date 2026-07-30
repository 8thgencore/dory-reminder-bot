package commands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupAppLinkUsesMainMiniApp(t *testing.T) {
	link := groupAppLink("production_bot", -1001234567890)

	require.Equal(
		t,
		"https://t.me/production_bot?startapp=chat_-1001234567890",
		link,
	)
}
