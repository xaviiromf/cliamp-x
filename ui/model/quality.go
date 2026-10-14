package model

import (
	"strings"

	"github.com/xaviiromf/cliamp-x/config"
	"github.com/xaviiromf/cliamp-x/player"
)

// ytQuality is the audio quality of YouTube songs: one per network, the
// network the app last reported, and the Premium quality switch.
type ytQuality struct {
	wifi, mobile string // alta, normal or ahorro
	network      string // wifi or mobile
	premium      bool
}

// qualityKbps is the bitrate cap of each quality, in kbps; 0 is the best.
var qualityKbps = map[string]int{"alta": 0, "normal": 128, "ahorro": 64}

// sectionStringSaver is implemented by config savers that can write a
// string key of a config section, such as quality_wifi in [ytmusic].
type sectionStringSaver interface {
	SaveSectionString(section, key, value string) error
}

// SetYTQuality applies the [ytmusic] quality keys. An empty quality is the
// default: alta on Wi-Fi, normal on mobile data.
func (m *Model) SetYTQuality(wifi, mobile string, premium bool) {
	if !config.ValidQuality(wifi) {
		wifi = "alta"
	}
	if !config.ValidQuality(mobile) {
		mobile = "normal"
	}
	m.ytQuality = ytQuality{wifi: wifi, mobile: mobile, network: "wifi", premium: premium}
	m.applyYTQuality()
}

// applyYTQuality hands the quality of the current network to the player.
func (m *Model) applyYTQuality() {
	q := m.ytQuality.wifi
	if m.ytQuality.network == "mobile" {
		q = m.ytQuality.mobile
	}
	player.SetYTDLQuality(qualityKbps[q])
	player.SetYTDLPremium(m.ytQuality.premium)
}

// setQualitySetting serves the quality names of settings.set and saves the
// choice in [ytmusic]. It reports false for a name or value it does not
// take. network is not saved: the app reports it at every start.
func (m *Model) setQualitySetting(name, value string) (bool, error) {
	q, value := &m.ytQuality, strings.ToLower(value)
	switch {
	case name == "quality_wifi" && config.ValidQuality(value):
		q.wifi = value
	case name == "quality_mobile" && config.ValidQuality(value):
		q.mobile = value
	case name == "network" && (value == "wifi" || value == "mobile"):
		q.network = value
	case name == "premium_quality" && (value == "on" || value == "off"):
		q.premium = value == "on"
	default:
		return false, nil
	}
	m.applyYTQuality()
	switch name {
	case "network":
	case "premium_quality":
		if s, ok := m.configSaver.(sectionBoolSaver); ok {
			return true, m.reportConfigSave(s.SaveSectionBool("ytmusic", name, q.premium))
		}
	default:
		if s, ok := m.configSaver.(sectionStringSaver); ok {
			return true, m.reportConfigSave(s.SaveSectionString("ytmusic", name, value))
		}
	}
	return true, nil
}
