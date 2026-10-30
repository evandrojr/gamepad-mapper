package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	evSyn = 0
	evKey = 1
	evRel = 2
	evAbs = 3
	evMsc = 4
)

const (
	keyEsc        = 1
	key1          = 2
	key2          = 3
	key3          = 4
	key4          = 5
	key5          = 6
	key6          = 7
	key7          = 8
	key8          = 9
	key9          = 10
	key0          = 11
	keyMinus      = 12
	keyEqual      = 13
	keyBackspace  = 14
	keyTab        = 15
	keyQ          = 16
	keyW          = 17
	keyE          = 18
	keyR          = 19
	keyT          = 20
	keyY          = 21
	keyU          = 22
	keyI          = 23
	keyO          = 24
	keyP          = 25
	keyLeftBrace  = 26
	keyA          = 30
	keyS          = 31
	keyD          = 32
	keyF          = 33
	keyG          = 34
	keyH          = 35
	keyJ          = 36
	keyK          = 37
	keyL          = 38
	keySemicolon  = 39
	keyApostrophe = 40
	keyGrave      = 41
	keyLShift     = 42
	keyBackslash  = 43
	keyZ          = 44
	keyX          = 45
	keyC          = 46
	keyV          = 47
	keyB          = 48
	keyN          = 49
	keyM          = 50
	keyComma      = 51
	keyDot        = 52
	keySlash      = 53
	keyRShift     = 54
	keyLCtrl      = 29
	keyLAlt       = 56
	keySpace      = 57
	keyCapsLock   = 58
	keyEnter      = 28
	keyHome       = 102
	keyUp         = 103
	keyPageUp     = 104
	keyLeft       = 105
	keyRight      = 106
	keyEnd        = 107
	keyDown       = 108
	keyPageDown   = 109
	keyInsert     = 110
	keyDelete     = 111
)

const (
	btnLeft    = 0x110
	btnRight   = 0x111
	btnMiddle  = 0x112
	btnSide    = 0x113
	btnExtra   = 0x114
	btnForward = 0x115
	btnBack    = 0x116
)

const (
	btnTrigger = 0x120
	btnThumb   = 0x121
	btnThumb2  = 0x122
	btnTop     = 0x123
	btnTop2    = 0x124
	btnPinkie  = 0x125
	btnBase    = 0x126
	btnBase2   = 0x127
	btnBase3   = 0x128
	btnBase4   = 0x129
	btnBase5   = 0x12a
	btnBase6   = 0x12b

	btnSouth  = 0x130
	btnEast   = 0x131
	btnWest   = 0x132
	btnNorth  = 0x133
	btnTL     = 0x136
	btnTR     = 0x137
	btnTL2    = 0x138
	btnTR2    = 0x139
	btnSelect = 0x13a
	btnStart  = 0x13b
	btnMode   = 0x13c
	btnThumbL = 0x13d
	btnThumbR = 0x13e
	btnTouch  = 0x14a

	btnDpadUp    = 0x220
	btnDpadDown  = 0x221
	btnDpadLeft  = 0x222
	btnDpadRight = 0x223

	btnTriggerHappy1 = 0x2c0
	btnTriggerHappy2 = 0x2c1
	btnTriggerHappy3 = 0x2c2
	btnTriggerHappy4 = 0x2c3
)

const (
	absX     = 0x00
	absY     = 0x01
	absZ     = 0x02
	absRX    = 0x03
	absRY    = 0x04
	absRZ    = 0x05
	absHat0X = 0x10
	absHat0Y = 0x11
)

const (
	iocNone  = 0
	iocWrite = 1
	iocRead  = 2
)

type absInfo struct {
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

type inputEvent struct {
	Sec   int64
	Usec  int64
	Type  uint16
	Code  uint16
	Value int32
}

type Device struct {
	Path    string
	File    *os.File
	Name    string
	Ev      map[uint16]bool
	Keys    map[uint16]bool
	Abs     map[uint16]bool
	AbsInfo map[uint16]absInfo
}

func ioc(dir uintptr, typ byte, nr uintptr, size uintptr) uintptr {
	return (dir << 30) | (size << 16) | (uintptr(typ) << 8) | nr
}

func ioctlPtr(fd uintptr, req uintptr, ptr unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(ptr))
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlValue(fd uintptr, req uintptr, value uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, value)
	if errno != 0 {
		return errno
	}
	return nil
}

func readName(fd uintptr) (string, error) {
	buf := make([]byte, 256)
	if err := ioctlPtr(fd, ioc(iocRead, 'E', 0x06, uintptr(len(buf))), unsafe.Pointer(&buf[0])); err != nil {
		return "", err
	}
	if i := bytes.IndexByte(buf, 0); i >= 0 {
		buf = buf[:i]
	}
	return strings.TrimSpace(string(buf)), nil
}

func readBits(fd uintptr, ev uint16, max int) (map[uint16]bool, error) {
	size := max/8 + 1
	buf := make([]byte, size)
	if err := ioctlPtr(fd, ioc(iocRead, 'E', uintptr(0x20+ev), uintptr(size)), unsafe.Pointer(&buf[0])); err != nil {
		return nil, err
	}
	out := make(map[uint16]bool)
	for i := 0; i <= max; i++ {
		if buf[i/8]&(1<<uint(i%8)) != 0 {
			out[uint16(i)] = true
		}
	}
	return out, nil
}

func readAbsInfo(fd uintptr, axis uint16) (absInfo, error) {
	var info absInfo
	err := ioctlPtr(fd, ioc(iocRead, 'E', uintptr(0x40+axis), unsafe.Sizeof(info)), unsafe.Pointer(&info))
	return info, err
}

func openDevice(path string) (*Device, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	d := &Device{Path: path, File: f, AbsInfo: make(map[uint16]absInfo)}
	fd := f.Fd()
	d.Name, err = readName(fd)
	if err != nil {
		f.Close()
		return nil, err
	}
	d.Ev, err = readBits(fd, 0, 31)
	if err != nil {
		f.Close()
		return nil, err
	}
	d.Keys, err = readBits(fd, evKey, 0x2ff)
	if err != nil {
		f.Close()
		return nil, err
	}
	d.Abs, err = readBits(fd, evAbs, 0x3f)
	if err != nil {
		f.Close()
		return nil, err
	}
	for axis := range d.Abs {
		info, err := readAbsInfo(fd, axis)
		if err == nil {
			d.AbsInfo[axis] = info
		}
	}
	return d, nil
}

func (d *Device) Close() error {
	if d.File == nil {
		return nil
	}
	return d.File.Close()
}

func eventNumber(path string) int {
	base := filepath.Base(path)
	n, _ := strconv.Atoi(strings.TrimPrefix(base, "event"))
	return n
}

func eventPaths() []string {
	paths, _ := filepath.Glob("/dev/input/event*")
	sort.Slice(paths, func(i, j int) bool {
		return eventNumber(paths[i]) < eventNumber(paths[j])
	})
	return paths
}

func isGamepad(d *Device) bool {
	hasAxis := d.Abs[absX] || d.Abs[absY] || d.Abs[absRX] || d.Abs[absRY] ||
		d.Abs[absHat0X] || d.Abs[absHat0Y]
	modern := d.Keys[btnSouth] || d.Keys[btnEast] || d.Keys[btnNorth] || d.Keys[btnWest]
	legacy := d.Keys[btnTrigger] && (d.Keys[btnBase3] || d.Keys[btnBase4])
	if hasAxis && (modern || legacy) {
		return true
	}
	name := strings.ToLower(d.Name)
	for _, marker := range []string{"gamepad", "joystick", "controller", "dualshock", "dualsense", "xbox", "game pad"} {
		if hasAxis && strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func listDevices() {
	fmt.Println("Dispositivos de entrada:")
	for _, path := range eventPaths() {
		d, err := openDevice(path)
		if err != nil {
			continue
		}
		mark := ""
		if isGamepad(d) {
			mark = "  <-- GAMEPAD"
		}
		fmt.Printf("  %-22s %s%s\n", path, d.Name, mark)
		if isGamepad(d) {
			fmt.Printf("      botoes: %s\n", joinCodeNames(d.Keys, buttonLinuxName))
			fmt.Printf("      eixos:  %s\n", joinCodeNames(d.Abs, absName))
		}
		d.Close()
	}
	fmt.Println()
	fmt.Println("Use -device event29 (ou parte do nome) para escolher outro dispositivo.")
}

func joinCodeNames(set map[uint16]bool, name func(uint16) string) string {
	codes := make([]int, 0, len(set))
	for code := range set {
		codes = append(codes, int(code))
	}
	sort.Ints(codes)
	parts := make([]string, 0, len(codes))
	for _, code := range codes {
		parts = append(parts, name(uint16(code)))
	}
	return strings.Join(parts, ", ")
}

func findDevice(selection string, wait bool) (string, error) {
	if strings.HasPrefix(selection, "/") {
		if _, err := os.Stat(selection); err != nil {
			return "", err
		}
		return selection, nil
	}
	for {
		paths := eventPaths()
		if selection != "" && selection != "auto" {
			if index, err := strconv.Atoi(selection); err == nil {
				if index >= 0 && index < len(paths) {
					return paths[index], nil
				}
			}
			for _, path := range paths {
				d, err := openDevice(path)
				if err != nil {
					continue
				}
				match := strings.EqualFold(path, selection) ||
					strings.Contains(strings.ToLower(d.Name), strings.ToLower(selection)) ||
					strings.Contains(strings.ToLower(filepath.Base(path)), strings.ToLower(selection))
				d.Close()
				if match {
					return path, nil
				}
			}
		} else {
			for _, path := range paths {
				d, err := openDevice(path)
				if err != nil {
					continue
				}
				ok := isGamepad(d) && !strings.HasPrefix(d.Name, "SpartanPort Gamepad Mapper")
				d.Close()
				if ok {
					return path, nil
				}
			}
		}
		if !wait {
			if selection != "" && selection != "auto" {
				return "", fmt.Errorf("dispositivo %q nao encontrado", selection)
			}
			return "", errors.New("nenhum gamepad encontrado")
		}
		fmt.Println("Aguardando gamepad... conecte o controle ou use -list.")
		time.Sleep(time.Second)
	}
}

var keyDisplayNames = map[uint16]string{
	keyEsc: "Esc", key1: "1", key2: "2", key3: "3", key4: "4", key5: "5", key6: "6",
	key7: "7", key8: "8", key9: "9", key0: "0", keyMinus: "-", keyEqual: "=",
	keyBackspace: "Backspace", keyTab: "Tab", keyQ: "Q", keyW: "W", keyE: "E", keyR: "R",
	keyT: "T", keyY: "Y", keyU: "U", keyI: "I", keyO: "O", keyP: "P",
	keyA: "A", keyS: "S", keyD: "D", keyF: "F", keyG: "G", keyH: "H", keyJ: "J",
	keyK: "K", keyL: "L", keyZ: "Z", keyX: "X", keyC: "C", keyV: "V", keyB: "B",
	keyN: "N", keyM: "M", keyComma: ",", keyDot: ".", keySlash: "/",
	keyLShift: "Shift esq.", keyRShift: "Shift dir.", keyLCtrl: "Ctrl esq.", keyLAlt: "Alt esq.",
	keySpace: "Espaco", keyCapsLock: "CapsLock", keyEnter: "Enter", keyHome: "Home",
	keyUp: "Seta cima", keyPageUp: "PageUp", keyLeft: "Seta esq.", keyRight: "Seta dir.",
	keyEnd: "End", keyDown: "Seta baixo", keyPageDown: "PageDown", keyInsert: "Insert",
	keyDelete: "Delete", keyLeftBrace: "[", keyBackslash: "\\", keySemicolon: ";",
	keyApostrophe: "'", keyGrave: "`",
	btnLeft: "Mouse esq.", btnRight: "Mouse dir.", btnMiddle: "Mouse meio",
	btnSide: "Mouse side", btnExtra: "Mouse extra", btnForward: "Mouse avancar", btnBack: "Mouse voltar",
}

func keyName(code uint16) string {
	if name, ok := keyDisplayNames[code]; ok {
		return name
	}
	return fmt.Sprintf("KEY_%d", code)
}

var keyParseNames = map[string]uint16{
	"esc": keyEsc, "escape": keyEsc, "1": key1, "2": key2, "3": key3, "4": key4,
	"5": key5, "6": key6, "7": key7, "8": key8, "9": key9, "0": key0,
	"minus": keyMinus, "-": keyMinus, "equal": keyEqual, "=": keyEqual,
	"backspace": keyBackspace, "tab": keyTab, "q": keyQ, "w": keyW, "e": keyE, "r": keyR,
	"t": keyT, "y": keyY, "u": keyU, "i": keyI, "o": keyO, "p": keyP,
	"a": keyA, "s": keyS, "d": keyD, "f": keyF, "g": keyG, "h": keyH, "j": keyJ,
	"k": keyK, "l": keyL, "z": keyZ, "x": keyX, "c": keyC, "v": keyV, "b": keyB,
	"n": keyN, "m": keyM, "comma": keyComma, ",": keyComma, "dot": keyDot, ".": keyDot,
	"slash": keySlash, "/": keySlash, "leftshift": keyLShift, "lshift": keyLShift,
	"shift": keyLShift, "rightshift": keyRShift, "rshift": keyRShift,
	"leftctrl": keyLCtrl, "lctrl": keyLCtrl, "ctrl": keyLCtrl, "leftalt": keyLAlt,
	"lalt": keyLAlt, "alt": keyLAlt, "space": keySpace, " ": keySpace,
	"capslock": keyCapsLock, "enter": keyEnter, "return": keyEnter,
	"home": keyHome, "up": keyUp, "pageup": keyPageUp, "left": keyLeft,
	"right": keyRight, "end": keyEnd, "down": keyDown, "pagedown": keyPageDown,
	"insert": keyInsert, "delete": keyDelete, "leftbrace": keyLeftBrace, "[": keyLeftBrace,
	"backslash": keyBackslash, "\\": keyBackslash, "semicolon": keySemicolon, ";": keySemicolon,
	"apostrophe": keyApostrophe, "'": keyApostrophe, "grave": keyGrave, "`": keyGrave,
	"mouseleft": btnLeft, "mouse1": btnLeft, "m1": btnLeft,
	"mouseright": btnRight, "mouse2": btnRight, "m2": btnRight,
	"mousemiddle": btnMiddle, "mouse3": btnMiddle, "m3": btnMiddle,
	"mouseside": btnSide, "mouse4": btnSide, "m4": btnSide,
	"mouseextra": btnExtra, "mouse5": btnExtra, "m5": btnExtra,
	"mouseforward": btnForward, "forward": btnForward,
	"mouseback": btnBack, "back": btnBack,
	"none": 0,
}

func parseKey(name string) (uint16, error) {
	clean := strings.ToLower(strings.TrimSpace(name))
	if code, ok := keyParseNames[clean]; ok {
		return code, nil
	}
	if strings.HasPrefix(clean, "key_") {
		n, err := strconv.Atoi(strings.TrimPrefix(clean, "key_"))
		if err == nil && n >= 0 && n <= 0x2ff {
			return uint16(n), nil
		}
	}
	if n, err := strconv.Atoi(clean); err == nil && n >= 0 && n <= 0x2ff {
		return uint16(n), nil
	}
	return 0, fmt.Errorf("tecla desconhecida: %s", name)
}

var keyLinuxNames = map[uint16]string{
	keyEsc: "KEY_ESC", key1: "KEY_1", key2: "KEY_2", key3: "KEY_3", key4: "KEY_4",
	key5: "KEY_5", key6: "KEY_6", key7: "KEY_7", key8: "KEY_8", key9: "KEY_9", key0: "KEY_0",
	keyBackspace: "KEY_BACKSPACE", keyTab: "KEY_TAB", keyQ: "KEY_Q", keyW: "KEY_W",
	keyE: "KEY_E", keyR: "KEY_R", keyT: "KEY_T", keyY: "KEY_Y", keyU: "KEY_U",
	keyI: "KEY_I", keyO: "KEY_O", keyP: "KEY_P", keyA: "KEY_A", keyS: "KEY_S",
	keyD: "KEY_D", keyF: "KEY_F", keyG: "KEY_G", keyH: "KEY_H", keyJ: "KEY_J",
	keyK: "KEY_K", keyL: "KEY_L", keyZ: "KEY_Z", keyX: "KEY_X", keyC: "KEY_C",
	keyV: "KEY_V", keyB: "KEY_B", keyN: "KEY_N", keyM: "KEY_M", keySpace: "KEY_SPACE",
	keyLShift: "KEY_LEFTSHIFT", keyLCtrl: "KEY_LEFTCTRL", keyLAlt: "KEY_LEFTALT",
	keyEnter: "KEY_ENTER", keyInsert: "KEY_INSERT", keyUp: "KEY_UP", keyDown: "KEY_DOWN",
	keyLeft: "KEY_LEFT", keyRight: "KEY_RIGHT", keyHome: "KEY_HOME", keyEnd: "KEY_END",
	keyPageUp: "KEY_PAGEUP", keyPageDown: "KEY_PAGEDOWN", keyDelete: "KEY_DELETE",
}

func linuxKeyName(code uint16) string {
	if name, ok := keyLinuxNames[code]; ok {
		return name
	}
	return fmt.Sprintf("KEY_%d", code)
}

var buttonLinuxNames = map[uint16]string{
	btnTrigger: "BTN_TRIGGER", btnThumb: "BTN_THUMB", btnThumb2: "BTN_THUMB2",
	btnTop: "BTN_TOP", btnTop2: "BTN_TOP2", btnPinkie: "BTN_PINKIE",
	btnBase: "BTN_BASE", btnBase2: "BTN_BASE2", btnBase3: "BTN_BASE3",
	btnBase4: "BTN_BASE4", btnBase5: "BTN_BASE5", btnBase6: "BTN_BASE6",
	btnSouth: "BTN_SOUTH", btnEast: "BTN_EAST", btnWest: "BTN_WEST", btnNorth: "BTN_NORTH",
	btnTL: "BTN_TL", btnTR: "BTN_TR", btnTL2: "BTN_TL2", btnTR2: "BTN_TR2",
	btnSelect: "BTN_SELECT", btnStart: "BTN_START", btnMode: "BTN_MODE",
	btnThumbL: "BTN_THUMBL", btnThumbR: "BTN_THUMBR", btnTouch: "BTN_TOUCH",
	btnDpadUp: "BTN_DPAD_UP", btnDpadDown: "BTN_DPAD_DOWN",
	btnDpadLeft: "BTN_DPAD_LEFT", btnDpadRight: "BTN_DPAD_RIGHT",
	btnTriggerHappy1: "BTN_TRIGGER_HAPPY1", btnTriggerHappy2: "BTN_TRIGGER_HAPPY2",
	btnTriggerHappy3: "BTN_TRIGGER_HAPPY3", btnTriggerHappy4: "BTN_TRIGGER_HAPPY4",
}

func buttonLinuxName(code uint16) string {
	if name, ok := buttonLinuxNames[code]; ok {
		return name
	}
	return fmt.Sprintf("BTN_%d", code)
}

var absLinuxNames = map[uint16]string{
	absX: "ABS_X", absY: "ABS_Y", absZ: "ABS_Z", absRX: "ABS_RX",
	absRY: "ABS_RY", absRZ: "ABS_RZ", absHat0X: "ABS_HAT0X", absHat0Y: "ABS_HAT0Y",
}

func absName(code uint16) string {
	if name, ok := absLinuxNames[code]; ok {
		return name
	}
	return fmt.Sprintf("ABS_%d", code)
}

var psButtonNames = map[uint16]string{
	btnSouth: "Cross / Xis", btnEast: "Circulo", btnWest: "Quadrado", btnNorth: "Triangulo",
	btnTL: "L1", btnTR: "R1", btnTL2: "L2", btnTR2: "R2",
	btnThumbL: "L3", btnThumbR: "R3", btnSelect: "Share/View", btnStart: "Options",
	btnMode: "PS", btnTouch: "Touchpad",
	btnDpadUp: "D-pad cima", btnDpadDown: "D-pad baixo",
	btnDpadLeft: "D-pad esquerda", btnDpadRight: "D-pad direita",
	btnTrigger: "Triangulo", btnThumb: "Circulo", btnThumb2: "Cross / Xis",
	btnTop: "Quadrado", btnTop2: "L1", btnPinkie: "R1", btnBase: "L2", btnBase2: "R2",
	btnBase3: "Share/View", btnBase4: "Options", btnBase5: "L3", btnBase6: "R3",
	btnTriggerHappy1: "D-pad cima", btnTriggerHappy2: "D-pad baixo",
	btnTriggerHappy3: "D-pad esquerda", btnTriggerHappy4: "D-pad direita",
}

func psButtonName(code uint16) string {
	if name, ok := psButtonNames[code]; ok {
		return name
	}
	return "botao"
}
