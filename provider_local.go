package main

import "github.com/xaviiromf/cliamp-x/provider"

// Local playlists, favorites and history. buildProviders makes the provider
// itself, because the Model and --playlist use it too.
func init() {
	registerProvider(providerFactory{
		providerKey: providerKey{key: "local", name: "Local"},
		order:       100,
		shortcut:    "L",
		build: func(env providerEnv) []provider.Entry {
			if env.local == nil {
				logProviderSkipped("Local", "local", "config directory unavailable")
				return nil
			}
			return []provider.Entry{{Provider: env.local}}
		},
	})
}
