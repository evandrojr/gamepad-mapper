package main

import (
	"fmt"
	"os"
	"sort"
	"time"
	"unsafe"
)

const (
	uiSetEvBit   = 100
	uiSetKeyBit  = 101
	uiDevCreate  = 1
	uiDevDestroy = 2
)

const (
	synReport = 0
)

type inputID struct {
	BusType uint16
	Vendor  uint16
	Product uint16
	Version uint16
}

type uinputUserDev struct {
	Name         [80]byte
	ID           inputID
	FFEffectsMax uint32
	AbsMax       [64]int32
	AbsMin       [64]int32
	AbsFuzz      [64]int32
	AbsFlat      [64]int32
}

type virtualInput struct {
	file *os.File
	down map[uint16]bool
}

func newVirtualInput(name string, keys []uint16) (*virtualInput, error) {
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("abrir /dev/uinput: %w", err)
	}
	fd := f.Fd()
	fail := func(err error) (*virtualInput, error) {
		f.Close()
		return nil, err
	}
	if err := ioctlValue(fd, ioc(iocWrite, 'U', uiSetEvBit, uintptr(unsafe.Sizeof(uint32(0)))), evKey); err != nil {
		return fail(fmt.Errorf("UI_SET_EVBIT: %w", err))
	}
	seen := make(map[uint16]bool)
	for _, code := range keys {
		if code == 0 || seen[code] {
			continue
		}
		seen[code] = true
		if err := ioctlValue(fd, ioc(iocWrite, 'U', uiSetKeyBit, uintptr(unsafe.Sizeof(uint32(0)))), uintptr(code)); err != nil {
			return fail(fmt.Errorf("UI_SET_KEYBIT %d: %w", code, err))
		}
	}
	dev := uinputUserDev{ID: inputID{BusType: 0x03, Vendor: 0x1234, Product: 0x5678, Version: 1}}
	copy(dev.Name[:79], name)
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&dev)), int(unsafe.Sizeof(dev)))
	if _, err := f.Write(buf); err != nil {
		return fail(fmt.Errorf("gravar uinput_user_dev: %w", err))
	}
	if err := ioctlPtr(fd, ioc(iocNone, 'U', uiDevCreate, 0), nil); err != nil {
		return fail(fmt.Errorf("UI_DEV_CREATE: %w", err))
	}
	time.Sleep(120 * time.Millisecond)
	return &virtualInput{file: f, down: make(map[uint16]bool)}, nil
}

func (v *virtualInput) emit(typ uint16, code uint16, value int32) error {
	now := time.Now()
	ev := inputEvent{Sec: now.Unix(), Usec: int64(now.Nanosecond() / 1000), Type: typ, Code: code, Value: value}
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&ev)), int(unsafe.Sizeof(ev)))
	_, err := v.file.Write(buf)
	return err
}

func (v *virtualInput) sync() error {
	return v.emit(evSyn, synReport, 0)
}

func (v *virtualInput) Key(code uint16, pressed bool) error {
	if code == 0 {
		return nil
	}
	if pressed == v.down[code] {
		return nil
	}
	value := int32(0)
	if pressed {
		value = 1
	}
	if err := v.emit(evKey, code, value); err != nil {
		return err
	}
	if err := v.sync(); err != nil {
		return err
	}
	v.down[code] = pressed
	return nil
}

func (v *virtualInput) ReleaseAll() {
	codes := make([]int, 0, len(v.down))
	for code, pressed := range v.down {
		if pressed {
			codes = append(codes, int(code))
		}
	}
	sort.Ints(codes)
	for _, code := range codes {
		_ = v.Key(uint16(code), false)
	}
}

func (v *virtualInput) Close() {
	if v.file == nil {
		return
	}
	v.ReleaseAll()
	_ = ioctlPtr(v.file.Fd(), ioc(iocNone, 'U', uiDevDestroy, 0), nil)
	_ = v.file.Close()
	v.file = nil
}
