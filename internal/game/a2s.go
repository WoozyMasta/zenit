// Package game provides functionality to query game servers using the Source Engine Query (A2S) protocol.
package game

import (
	"github.com/woozymasta/a2s/pkg/a2s"
	"github.com/woozymasta/zenit/internal/config"
)

// QueryServer connects to a game server via UDP and requests A2S_INFO.
// It returns server details (such as name, map, players) or an error if the server is unreachable.
func QueryServer(ip string, port int, options config.A2S) (*a2s.Info, error) {
	client, err := a2s.New(ip, port)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	client.BufferSize = options.BufferSize
	client.Timeout = options.Timeout

	return client.GetInfo()
}
