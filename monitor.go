package main

import (
	"fmt"
	"os"
	"time"
)

type axisMonitor struct {
	lastValue int32
	lastDir   int
	lastTime  time.Time
}

func runMonitor(d *Device, preset Preset) {
	p := detectProfile(d)
	fmt.Printf("Controle: %s (%s)\n", d.Name, d.Path)
	fmt.Printf("Perfil:   %s\n", p.describe())
	fmt.Printf("Botoes:   %s\n", joinCodeNames(d.Keys, buttonLinuxName))
	fmt.Printf("Eixos:    %s\n", joinCodeNames(d.Abs, absName))
	preset.describe()
	fmt.Println()
	fmt.Println("Aperte cada botao e mexa nos analogicos. Ctrl+C para sair.")
	fmt.Println()

	states := make(map[uint16]axisMonitor)
	handler := func(ev inputEvent) {
		switch ev.Type {
		case evKey:
			if ev.Value == 2 {
				return
			}
			action, ok := buttonAction(ev.Code)
			label := psButtonName(ev.Code)
			keyInfo := ""
			if ok {
				if actionLabel := actionLabels[action]; actionLabel != "" {
					label = actionLabel
				}
				key := preset.Keys[action]
				if key == 0 {
					keyInfo = " -> (sem tecla)"
				} else {
					keyInfo = " -> " + keyName(key)
				}
			}
			state := "PRESSIONADO"
			if ev.Value == 0 {
				state = "SOLTO      "
			}
			fmt.Printf("%s  %-24s %-18s%s\n", state, label, buttonLinuxName(ev.Code), keyInfo)
		case evAbs:
			printAxis(ev, p, preset, states, d)
		}
	}

	err := runWithStop(func(stop <-chan struct{}) error {
		return readEvents(int(d.File.Fd()), stop, handler)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Erro:", err)
	}
}

func printAxis(ev inputEvent, p profile, preset Preset, states map[uint16]axisMonitor, d *Device) {
	role := axisRole(ev.Code, p)
	if role == "" {
		role = "desconhecido"
	}
	now := time.Now()
	state := states[ev.Code]
	dir := axisDirectionFor(ev.Code, ev.Value, d)
	hat := ev.Code == absHat0X || ev.Code == absHat0Y
	changed := ev.Value != state.lastValue
	if hat {
		if !changed {
			return
		}
	} else if dir == state.lastDir && now.Sub(state.lastTime) < 100*time.Millisecond {
		return
	}
	extra := ""
	action := axisAction(ev.Code, p, dir)
	if action != "" {
		if key := preset.Keys[action]; key != 0 {
			extra = " -> " + keyName(key)
		}
	}
	fmt.Printf("EIXO       %-24s %-11s valor=%-6d%s\n", role, absName(ev.Code), ev.Value, extra)
	state.lastTime = now
	state.lastValue = ev.Value
	state.lastDir = dir
	states[ev.Code] = state
}

func axisRole(code uint16, p profile) string {
	switch code {
	case absX:
		return "analogico esq. X"
	case absY:
		return "analogico esq. Y"
	case absHat0X:
		return "D-pad X"
	case absHat0Y:
		return "D-pad Y"
	}
	if p.HasRight && code == p.RightX {
		return "analogico dir. X"
	}
	if p.HasRight && code == p.RightY {
		return "analogico dir. Y"
	}
	if p.TriggerL != 0 && code == p.TriggerL {
		return "gatilho L2"
	}
	if p.TriggerR != 0 && code == p.TriggerR {
		return "gatilho R2"
	}
	return ""
}

func axisAction(code uint16, p profile, dir int) string {
	switch code {
	case absHat0X:
		if dir < 0 {
			return "left"
		}
		if dir > 0 {
			return "right"
		}
	case absHat0Y:
		if dir < 0 {
			return "up"
		}
		if dir > 0 {
			return "down"
		}
	case absX:
		if dir < 0 {
			return "ls_left"
		}
		if dir > 0 {
			return "ls_right"
		}
	case absY:
		if dir < 0 {
			return "ls_up"
		}
		if dir > 0 {
			return "ls_down"
		}
	}
	if p.HasRight && code == p.RightX {
		if dir < 0 {
			return "rs_left"
		}
		if dir > 0 {
			return "rs_right"
		}
	}
	if p.HasRight && code == p.RightY {
		if dir < 0 {
			return "rs_up"
		}
		if dir > 0 {
			return "rs_down"
		}
	}
	if p.TriggerL != 0 && code == p.TriggerL {
		return "l2"
	}
	if p.TriggerR != 0 && code == p.TriggerR {
		return "r2"
	}
	return ""
}

func axisDirectionFor(code uint16, value int32, d *Device) int {
	if code == absHat0X || code == absHat0Y {
		return sign(value)
	}
	info, ok := d.AbsInfo[code]
	if !ok || info.Maximum == info.Minimum {
		return 0
	}
	minimum := float64(info.Minimum)
	maximum := float64(info.Maximum)
	center := minimum + (maximum-minimum)/2
	half := (maximum - minimum) / 2
	normalized := (float64(value) - center) / half
	if normalized <= -0.35 {
		return -1
	}
	if normalized >= 0.35 {
		return 1
	}
	return 0
}
