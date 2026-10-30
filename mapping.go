package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Combo struct {
	Name    string
	Actions []string
	Key     uint16
}

type Preset struct {
	Name   string
	Keys   map[string]uint16
	Combos []Combo
}

type profile struct {
	Legacy   bool
	RightX   uint16
	RightY   uint16
	TriggerL uint16
	TriggerR uint16
	HasRight bool
}

var actionLabels = map[string]string{
	"cross": "Cross / Xis", "circle": "Circulo", "square": "Quadrado", "triangle": "Triangulo",
	"l1": "L1", "r1": "R1", "l2": "L2", "r2": "R2", "l3": "L3", "r3": "R3",
	"options": "Options", "share": "Share/View", "ps": "PS", "touchpad": "Touchpad",
	"up": "D-pad cima", "down": "D-pad baixo", "left": "D-pad esquerda", "right": "D-pad direita",
	"ls_up": "Anal. esq. cima", "ls_down": "Anal. esq. baixo",
	"ls_left": "Anal. esq. esquerda", "ls_right": "Anal. esq. direita",
	"rs_up": "Anal. dir. cima", "rs_down": "Anal. dir. baixo",
	"rs_left": "Anal. dir. esquerda", "rs_right": "Anal. dir. direita",
}

var actionOrder = []string{
	"cross", "circle", "square", "triangle", "l1", "r1", "l2", "r2", "l3", "r3",
	"options", "share", "ps", "touchpad",
	"up", "down", "left", "right",
	"ls_up", "ls_down", "ls_left", "ls_right",
	"rs_up", "rs_down", "rs_left", "rs_right",
}

var actionAliases = map[string]string{
	"xis": "cross", "quadrado": "square", "circulo": "circle", "triangulo": "triangle",
	"cima": "up", "baixo": "down", "esquerda": "left", "direita": "right",
	"opcoes": "options", "compartilhar": "share", "analogico_esquerdo_cima": "ls_up",
}

func canonicalAction(name string) string {
	clean := strings.ToLower(strings.TrimSpace(name))
	if alias, ok := actionAliases[clean]; ok {
		return alias
	}
	if _, ok := actionLabels[clean]; ok {
		return clean
	}
	return ""
}

func portPreset() Preset {
	return Preset{
		Name: "port",
		Keys: map[string]uint16{
			"cross": keySpace, "circle": keyLShift, "square": keyE, "triangle": keyQ,
			"l1": key1, "r1": key3, "l2": keyR, "r2": keyF, "l3": keyZ, "r3": keyC,
			"options": keyEnter, "share": keyBackspace, "ps": keyInsert, "touchpad": keyTab,
			"up": keyI, "down": keyK, "left": keyJ, "right": keyL,
			"ls_up": keyW, "ls_down": keyS, "ls_left": keyA, "ls_right": keyD,
			"rs_up": keyUp, "rs_down": keyDown, "rs_left": keyLeft, "rs_right": keyRight,
		},
	}
}

func launcherPreset() Preset {
	return Preset{
		Name: "launcher",
		Keys: map[string]uint16{
			"cross": keySpace, "circle": keyLShift, "square": btnLeft, "triangle": btnRight,
			"l1": keyQ, "r1": keyE, "l2": keyR, "r2": btnForward, "l3": keyZ, "r3": keyX,
			"options": keyTab, "share": keyEsc, "ps": keyInsert, "touchpad": keyP,
			"up": key1, "down": key3, "left": key4, "right": key2,
			"ls_up": keyW, "ls_down": keyS, "ls_left": keyA, "ls_right": keyD,
			"rs_up": keyUp, "rs_down": keyDown, "rs_left": keyLeft, "rs_right": keyRight,
		},
		Combos: []Combo{
			{Name: "rage", Actions: []string{"l3", "r3"}, Key: keyF},
			{Name: "grab", Actions: []string{"l1", "circle"}, Key: keyC},
		},
	}
}

func loadPreset(name string) (Preset, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "launcher", "padrao":
		return launcherPreset(), nil
	case "port", "jogo", "sem-launcher":
		return portPreset(), nil
	default:
		return Preset{}, fmt.Errorf("preset desconhecido: %s (use launcher ou port)", name)
	}
}

func applyOverrides(preset *Preset, spec string) error {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}
	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("entrada invalida em -map: %q", entry)
		}
		action := canonicalAction(parts[0])
		if action == "" {
			return fmt.Errorf("acao desconhecida em -map: %q", parts[0])
		}
		key, err := parseKey(parts[1])
		if err != nil {
			return err
		}
		preset.Keys[action] = key
	}
	return nil
}

func presetKeys(preset Preset) []uint16 {
	set := make(map[uint16]bool)
	for _, key := range preset.Keys {
		if key != 0 {
			set[key] = true
		}
	}
	for _, combo := range preset.Combos {
		if combo.Key != 0 {
			set[combo.Key] = true
		}
	}
	codes := make([]int, 0, len(set))
	for key := range set {
		codes = append(codes, int(key))
	}
	sort.Ints(codes)
	out := make([]uint16, 0, len(codes))
	for _, code := range codes {
		out = append(out, uint16(code))
	}
	return out
}

func (p Preset) describe() {
	fmt.Printf("Preset: %s\n", p.Name)
	for _, action := range actionOrder {
		key := p.Keys[action]
		label := actionLabels[action]
		if label == "" {
			label = action
		}
		if key == 0 {
			fmt.Printf("  %-20s -> (sem tecla)\n", label)
		} else {
			fmt.Printf("  %-20s -> %s\n", label, keyName(key))
		}
	}
	for _, combo := range p.Combos {
		fmt.Printf("  %-20s -> %s (combo)\n", combo.Name, keyName(combo.Key))
	}
}

var actionByCode = map[uint16]string{
	btnSouth: "cross", btnEast: "circle", btnWest: "square", btnNorth: "triangle",
	btnTL: "l1", btnTR: "r1", btnTL2: "l2", btnTR2: "r2",
	btnThumbL: "l3", btnThumbR: "r3",
	btnSelect: "share", btnStart: "options", btnMode: "ps", btnTouch: "touchpad",
	btnDpadUp: "up", btnDpadDown: "down", btnDpadLeft: "left", btnDpadRight: "right",
	btnTrigger: "triangle", btnThumb: "circle", btnThumb2: "cross", btnTop: "square",
	btnBase: "l1", btnBase2: "r1", btnTop2: "l2", btnPinkie: "r2",
	btnBase3: "share", btnBase4: "options", btnBase5: "l3", btnBase6: "r3",
	btnTriggerHappy1: "up", btnTriggerHappy2: "down",
	btnTriggerHappy3: "left", btnTriggerHappy4: "right",
}

func buttonAction(code uint16) (string, bool) {
	action, ok := actionByCode[code]
	return action, ok
}

func detectProfile(d *Device) profile {
	modern := d.Keys[btnSouth] || d.Keys[btnEast] || d.Keys[btnNorth] || d.Keys[btnWest] ||
		d.Keys[btnTL] || d.Keys[btnTR] || d.Keys[btnTL2] || d.Keys[btnTR2] ||
		d.Keys[btnThumbL] || d.Keys[btnThumbR] || d.Keys[btnStart]

	legacy := !modern && d.Keys[btnTrigger] &&
		(d.Keys[btnBase3] || d.Keys[btnBase4] || d.Keys[btnBase] || d.Keys[btnBase2])

	p := profile{Legacy: legacy}
	switch {
	case d.Abs[absRX] && d.Abs[absRY]:
		p.RightX, p.RightY, p.HasRight = absRX, absRY, true
		if d.Abs[absZ] {
			p.TriggerL = absZ
		}
		if d.Abs[absRZ] {
			p.TriggerR = absRZ
		}
	case legacy && d.Abs[absZ] && d.Abs[absRZ]:
		p.RightX, p.RightY, p.HasRight = absZ, absRZ, true
	case modern:
		if d.Abs[absZ] {
			p.TriggerL = absZ
		}
		if d.Abs[absRZ] {
			p.TriggerR = absRZ
		}
	}
	return p
}

func (p profile) describe() string {
	parts := []string{}
	if p.Legacy {
		parts = append(parts, "DragonRise/legado")
	}
	if p.HasRight {
		parts = append(parts, fmt.Sprintf("analogico direito %s/%s", absName(p.RightX), absName(p.RightY)))
	}
	if p.TriggerL != 0 || p.TriggerR != 0 {
		parts = append(parts, fmt.Sprintf("gatilhos %s/%s", absName(p.TriggerL), absName(p.TriggerR)))
	}
	if len(parts) == 0 {
		parts = append(parts, "padrao")
	}
	return strings.Join(parts, ", ")
}

type mapper struct {
	dev         *Device
	profile     profile
	preset      Preset
	vi          *virtualInput
	keyRefs     map[uint16]int
	sources     map[string]uint16
	axisDir     map[uint16]int
	triggerOn   map[uint16]bool
	actionCount map[string]int
	buttonSeen  map[string]bool
	hatX        int
	hatY        int
	deadzone    float64
	verbose     bool
}

func newMapper(d *Device, p profile, preset Preset, vi *virtualInput, deadzone float64, verbose bool) *mapper {
	return &mapper{
		dev:         d,
		profile:     p,
		preset:      preset,
		vi:          vi,
		keyRefs:     make(map[uint16]int),
		sources:     make(map[string]uint16),
		axisDir:     make(map[uint16]int),
		triggerOn:   make(map[uint16]bool),
		actionCount: make(map[string]int),
		buttonSeen:  make(map[string]bool),
		deadzone:    deadzone,
		verbose:     verbose,
	}
}

func (m *mapper) setSource(source string, key uint16, active bool) {
	current, held := m.sources[source]
	if active {
		if held {
			return
		}
		if key == 0 {
			return
		}
		m.sources[source] = key
		m.keyRefs[key]++
		if m.keyRefs[key] == 1 {
			_ = m.vi.Key(key, true)
			if m.verbose {
				fmt.Printf("TECLA DOWN %-16s (%s)\n", keyName(key), source)
			}
		}
		return
	}
	if !held {
		return
	}
	delete(m.sources, source)
	if current == 0 {
		return
	}
	m.keyRefs[current]--
	if m.keyRefs[current] <= 0 {
		delete(m.keyRefs, current)
		_ = m.vi.Key(current, false)
		if m.verbose {
			fmt.Printf("TECLA UP   %-16s (%s)\n", keyName(current), source)
		}
	}
}

func (m *mapper) updateCombos() {
	for _, combo := range m.preset.Combos {
		active := true
		for _, action := range combo.Actions {
			if m.actionCount[action] <= 0 {
				active = false
				break
			}
		}
		m.setSource("combo:"+combo.Name, combo.Key, active)
	}
}

func (m *mapper) handleButton(code uint16, value int32) {
	action, ok := buttonAction(code)
	if !ok || value == 2 {
		return
	}
	source := "btn:" + strconv.Itoa(int(code))
	pressed := value > 0
	if pressed == m.buttonSeen[source] {
		return
	}
	m.buttonSeen[source] = pressed
	if pressed {
		m.actionCount[action]++
	} else if m.actionCount[action] > 0 {
		m.actionCount[action]--
	}
	m.setSource(source, m.preset.Keys[action], pressed)
	m.updateCombos()
}

func (m *mapper) axisDirection(axis uint16, value int32) int {
	info, ok := m.dev.AbsInfo[axis]
	if !ok || info.Maximum == info.Minimum {
		return 0
	}
	minimum := float64(info.Minimum)
	maximum := float64(info.Maximum)
	center := minimum + (maximum-minimum)/2
	half := (maximum - minimum) / 2
	normalized := (float64(value) - center) / half
	previous := m.axisDir[axis]
	release := m.deadzone * 0.75
	switch {
	case normalized <= -m.deadzone:
		previous = -1
	case normalized >= m.deadzone:
		previous = 1
	case normalized > -release && normalized < release:
		previous = 0
	}
	m.axisDir[axis] = previous
	return previous
}

func (m *mapper) handleStick(axis uint16, value int32, prefix, actionNeg, actionPos string) {
	direction := m.axisDirection(axis, value)
	m.setSource(prefix+":-", m.preset.Keys[actionNeg], direction < 0)
	m.setSource(prefix+":+", m.preset.Keys[actionPos], direction > 0)
}

func (m *mapper) handleTrigger(axis uint16, value int32, action string) {
	info, ok := m.dev.AbsInfo[axis]
	if !ok || info.Maximum == info.Minimum {
		return
	}
	normalized := (float64(value) - float64(info.Minimum)) / float64(info.Maximum-info.Minimum)
	active := m.triggerOn[axis]
	if normalized >= 0.5 {
		active = true
	} else if normalized <= 0.35 {
		active = false
	}
	m.triggerOn[axis] = active
	m.setSource("trig:"+strconv.Itoa(int(axis)), m.preset.Keys[action], active)
}

func (m *mapper) updateHat() {
	m.setSource("hat:up", m.preset.Keys["up"], m.hatY < 0)
	m.setSource("hat:down", m.preset.Keys["down"], m.hatY > 0)
	m.setSource("hat:left", m.preset.Keys["left"], m.hatX < 0)
	m.setSource("hat:right", m.preset.Keys["right"], m.hatX > 0)
}

func (m *mapper) handleAbs(code uint16, value int32) {
	switch {
	case code == absHat0X:
		m.hatX = sign(value)
		m.updateHat()
	case code == absHat0Y:
		m.hatY = sign(value)
		m.updateHat()
	case m.profile.HasRight && code == m.profile.RightX:
		m.handleStick(code, value, "rsx", "rs_left", "rs_right")
	case m.profile.HasRight && code == m.profile.RightY:
		m.handleStick(code, value, "rsy", "rs_up", "rs_down")
	case code == absX:
		m.handleStick(code, value, "lsx", "ls_left", "ls_right")
	case code == absY:
		m.handleStick(code, value, "lsy", "ls_up", "ls_down")
	case m.profile.TriggerL != 0 && code == m.profile.TriggerL:
		m.handleTrigger(code, value, "l2")
	case m.profile.TriggerR != 0 && code == m.profile.TriggerR:
		m.handleTrigger(code, value, "r2")
	}
}

func (m *mapper) handleEvent(ev inputEvent) {
	switch ev.Type {
	case evKey:
		m.handleButton(ev.Code, ev.Value)
	case evAbs:
		m.handleAbs(ev.Code, ev.Value)
	}
}

func (m *mapper) initializeState() {
	codes := make([]int, 0, len(m.dev.AbsInfo))
	for code := range m.dev.AbsInfo {
		codes = append(codes, int(code))
	}
	sort.Ints(codes)
	for _, code := range codes {
		m.handleAbs(uint16(code), m.dev.AbsInfo[uint16(code)].Value)
	}
}

func sign(value int32) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}
