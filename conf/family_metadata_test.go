package conf

import "testing"

func TestFamilyMetadataOptInDoesNotEnableBuiltIns(t *testing.T) {
	original := Server
	defer func() { Server = original }()
	Server.EnableFamilyMetadataPlugins = true
	Server.Agents = "apple-music,lastfm,coverartarchive,deezer"
	disableExternalServices()
	if Server.Agents != "apple-music,coverartarchive" || Server.LastFM.Enabled || Server.ListenBrainz.Enabled || Server.Deezer.Enabled || Server.EnableInsightsCollector {
		t.Fatal("metadata opt-in enabled unrelated services")
	}
	Server.EnableFamilyMetadataPlugins = false
	Server.Agents = "apple-music"
	disableExternalServices()
	if Server.Agents != "" {
		t.Fatal("external services kill switch changed without opt-in")
	}
}
