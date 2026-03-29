package luaplugin

import (
	"time"

	lua "github.com/yuin/gopher-lua"
)

// registerControlAPI adds cliamp.player control methods (next, prev, play_pause,
// stop, set_volume, set_speed, seek, toggle_mono, set_eq_band) to the cliamp table.
// These are only functional if the plugin declared permissions = {"control"}.
func registerControlAPI(L *lua.LState, cliamp *lua.LTable, ctrl *ControlProvider, p *Plugin, logger *pluginLogger) {
	playerTbl := L.GetField(cliamp, "player")
	tbl, ok := playerTbl.(*lua.LTable)
	if !ok {
		return
	}

	guard := func(name string) bool {
		if !p.perms["control"] {
			logger.log(p.Name, "warn", "%s requires permissions = {\"control\"}", name)
			return false
		}
		return true
	}

	L.SetField(tbl, "next", L.NewFunction(func(L *lua.LState) int {
		if guard("next") && ctrl.Next != nil {
			ctrl.Next()
		}
		return 0
	}))

	L.SetField(tbl, "prev", L.NewFunction(func(L *lua.LState) int {
		if guard("prev") && ctrl.Prev != nil {
			ctrl.Prev()
		}
		return 0
	}))

	L.SetField(tbl, "play_pause", L.NewFunction(func(L *lua.LState) int {
		if guard("play_pause") && ctrl.TogglePause != nil {
			ctrl.TogglePause()
		}
		return 0
	}))

	L.SetField(tbl, "stop", L.NewFunction(func(L *lua.LState) int {
		if guard("stop") && ctrl.Stop != nil {
			ctrl.Stop()
		}
		return 0
	}))

	L.SetField(tbl, "set_volume", L.NewFunction(func(L *lua.LState) int {
		if !guard("set_volume") {
			return 0
		}
		db := float64(L.CheckNumber(1))
		if db < -30 {
			db = -30
		} else if db > 6 {
			db = 6
		}
		if ctrl.SetVolume != nil {
			ctrl.SetVolume(db)
		}
		return 0
	}))

	L.SetField(tbl, "set_speed", L.NewFunction(func(L *lua.LState) int {
		if !guard("set_speed") {
			return 0
		}
		ratio := float64(L.CheckNumber(1))
		if ratio < 0.25 {
			ratio = 0.25
		} else if ratio > 2.0 {
			ratio = 2.0
		}
		if ctrl.SetSpeed != nil {
			ctrl.SetSpeed(ratio)
		}
		return 0
	}))

	L.SetField(tbl, "seek", L.NewFunction(func(L *lua.LState) int {
		if !guard("seek") {
			return 0
		}
		secs := float64(L.CheckNumber(1))
		if ctrl.Seek != nil {
			ctrl.Seek(secs)
		}
		return 0
	}))

	L.SetField(tbl, "toggle_mono", L.NewFunction(func(L *lua.LState) int {
		if guard("toggle_mono") && ctrl.ToggleMono != nil {
			ctrl.ToggleMono()
		}
		return 0
	}))

	L.SetField(tbl, "set_eq_band", L.NewFunction(func(L *lua.LState) int {
		if !guard("set_eq_band") {
			return 0
		}
		band := L.CheckInt(1) - 1 // Lua 1-indexed → Go 0-indexed
		db := float64(L.CheckNumber(2))
		if band < 0 || band > 9 {
			L.ArgError(1, "band must be 1-10")
			return 0
		}
		if db < -12 {
			db = -12
		} else if db > 12 {
			db = 12
		}
		if ctrl.SetEQBand != nil {
			ctrl.SetEQBand(band, db)
		}
		return 0
	}))

	// Convenience: sleep for plugin scripts that need timing.
	L.SetField(tbl, "sleep", L.NewFunction(func(L *lua.LState) int {
		secs := float64(L.CheckNumber(1))
		if secs > 0 && secs <= 10 {
			time.Sleep(time.Duration(secs * float64(time.Second)))
		}
		return 0
	}))
}
