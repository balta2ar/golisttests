package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTreeSitterTRunStringLiteral(t *testing.T) {
	require.Equal(t,
		[]string{"TestWeb", "TestWeb/works"},
		ParseTestNames(Spit(`
package test
func TestWeb(t *testing.T) {
	t.Run("works", func(t *testing.T) {
	})
}
`)))
}

func TestTreeSitterTRunStructLiteral(t *testing.T) {
	require.Equal(t,
		[]string{"TestWeb", "TestWeb/device_event"},
		ParseTestNames(Spit(`
package test
func TestWeb(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		data         []byte
		expectedType EventType
	}{
		{
			name:         "device event",
			path:         "/api/timon/v1/onDeviceEvent",
			data:         MakeDeviceEventStartCall(t, "hillary"),
			expectedType: DeviceEvent,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
		})
	}
}
`)))
}
