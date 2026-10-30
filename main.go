package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func usage() {
	fmt.Fprintf(os.Stderr, `gamepad-mapper - usa um gamepad como teclado no Linux

Uso:
  gamepad-mapper -list
  gamepad-mapper -monitor
  gamepad-mapper
  gamepad-mapper -preset port
  gamepad-mapper -device event29 -monitor
  gamepad-mapper -map "square=e,triangle=q,l1=1,r1=3"

Presets:
  launcher  teclas do launcher do SpartanPort (padrao, inclui mouse)
  port      teclas proprias do jogo sem o launcher (so teclado)

Flags:
`)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Exemplos:
  ./gamepad-mapper -monitor          descobre os botoes do controle
  sudo ./gamepad-mapper              roda o mapa no jogo
  ./gamepad-mapper -grab=false       nao esconde o controle do jogo
`)
}

func main() {
	list := flag.Bool("list", false, "lista os dispositivos de entrada")
	monitor := flag.Bool("monitor", false, "mostra em tempo real os botoes e eixos do controle")
	discover := flag.Bool("discover", false, "alias de -monitor")
	device := flag.String("device", "", "caminho, numero do event ou parte do nome do controle (padrao: auto)")
	presetName := flag.String("preset", "launcher", "mapa de teclas: launcher ou port")
	mapSpec := flag.String("map", "", "sobrescreve teclas: cross=space,l1=1,up=i,...")
	grab := flag.Bool("grab", true, "captura o controle para o jogo nao Receber entrada dupla")
	deadzone := flag.Float64("deadzone", 0.35, "zona morta dos analogicos (0.0 a 1.0)")
	wait := flag.Bool("wait", true, "espera um controle aparecer")
	flag.Usage = usage
	flag.Parse()

	if *list {
		listDevices()
		return
	}

	path, err := findDevice(*device, *wait)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Erro:", err)
		fmt.Fprintln(os.Stderr, "Rode ./gamepad-mapper -list para ver os dispositivos.")
		os.Exit(1)
	}

	d, err := openDevice(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Erro ao abrir", path+":", err)
		os.Exit(1)
	}
	defer d.Close()

	preset, err := loadPreset(*presetName)
	if err != nil {
		d.Close()
		fmt.Fprintln(os.Stderr, "Erro:", err)
		os.Exit(1)
	}
	if err := applyOverrides(&preset, *mapSpec); err != nil {
		d.Close()
		fmt.Fprintln(os.Stderr, "Erro:", err)
		os.Exit(1)
	}

	if *monitor || *discover {
		runMonitor(d, preset)
		return
	}
	if *deadzone < 0.05 || *deadzone > 0.95 {
		d.Close()
		fmt.Fprintln(os.Stderr, "Erro: -deadzone deve ficar entre 0.05 e 0.95")
		os.Exit(1)
	}
	if err := runMapper(d, preset, *grab, *deadzone); err != nil {
		fmt.Fprintln(os.Stderr, "Erro:", err)
		os.Exit(1)
	}
}

func runMapper(d *Device, preset Preset, grab bool, deadzone float64) error {
	p := detectProfile(d)
	fmt.Printf("Controle: %s (%s)\n", d.Name, d.Path)
	fmt.Printf("Perfil:   %s\n", p.describe())
	preset.describe()

	vi, err := newVirtualInput("SpartanPort Gamepad Mapper", presetKeys(preset))
	if err != nil {
		return err
	}
	defer vi.Close()

	if grab {
		if err := ioctlValue(d.File.Fd(), ioc(iocWrite, 'E', 0x90, 4), 1); err != nil {
			return fmt.Errorf("capturar o controle (EVIOCGRAB): %w (use -grab=false)", err)
		}
		defer ioctlValue(d.File.Fd(), ioc(iocWrite, 'E', 0x90, 4), 0)
		fmt.Println("Modo:     controle capturado; o jogo vera apenas o teclado virtual.")
	} else {
		fmt.Println("Modo:     sem captura; o controle continua visivel para o jogo.")
	}
	fmt.Println("Aperte Ctrl+C para sair.")

	m := newMapper(d, p, preset, vi, deadzone)
	m.initializeState()

	err = runWithStop(func(stop <-chan struct{}) error {
		return readEvents(int(d.File.Fd()), stop, m.handleEvent)
	})
	vi.ReleaseAll()
	return err
}

func runWithStop(run func(stop <-chan struct{}) error) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		select {
		case <-sig:
			close(stop)
		case <-done:
		}
	}()
	err := run(stop)
	close(done)
	return err
}

func waitReadable(fd int, timeout time.Duration) (bool, error) {
	var set syscall.FdSet
	set.Bits[fd/64] |= 1 << (uint(fd) % 64)
	tv := syscall.NsecToTimeval(timeout.Nanoseconds())
	n, err := syscall.Select(fd+1, &set, nil, nil, &tv)
	if err == syscall.EINTR {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func readEvents(fd int, stop <-chan struct{}, handler func(inputEvent)) error {
	const eventSize = 24
	buf := make([]byte, eventSize*64)
	leftover := make([]byte, 0, eventSize*2)
	for {
		select {
		case <-stop:
			return nil
		default:
		}
		ready, err := waitReadable(fd, 100*time.Millisecond)
		if err != nil {
			return err
		}
		if !ready {
			continue
		}
		n, err := syscall.Read(fd, buf)
		if err == syscall.EINTR || err == syscall.EAGAIN {
			continue
		}
		if err != nil {
			return err
		}
		if n <= 0 {
			continue
		}
		data := make([]byte, 0, len(leftover)+n)
		data = append(data, leftover...)
		data = append(data, buf[:n]...)
		offset := 0
		for offset+eventSize <= len(data) {
			chunk := data[offset : offset+eventSize]
			ev := inputEvent{
				Sec:   int64(binary.LittleEndian.Uint64(chunk[0:8])),
				Usec:  int64(binary.LittleEndian.Uint64(chunk[8:16])),
				Type:  binary.LittleEndian.Uint16(chunk[16:18]),
				Code:  binary.LittleEndian.Uint16(chunk[18:20]),
				Value: int32(binary.LittleEndian.Uint32(chunk[20:24])),
			}
			handler(ev)
			offset += eventSize
		}
		leftover = append(leftover[:0], data[offset:]...)
	}
}
